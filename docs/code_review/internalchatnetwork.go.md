"internal/chat/network.go"

# High‑level assessment

✅ **What this code does very well**

*   Clear separation between **network I/O**, **command parsing**, and **domain logic**
*   Thoughtful command set: `/dm`, `/switch`, `/nick`, `/rooms`, `/stats`, `/users`
*   Good use of buffered channels to decouple I/O
*   Rate‑limiting + spam protection is well thought out
*   UTF‑8 and message length validation ✅
*   Non‑blocking update broadcasts ✅
*   Logical connection handshake and onboarding flow

⚠️ **Major issues**

*   **Client lifecycle is broken (double close / inconsistent ownership)**
*   **Mutex + channel send patterns can deadlock**
*   **`bufio.Scanner` message size limit**
*   **Race conditions around `currentRoom`**
*   **Server / Room responsibilities are mixed**
*   **Ban + disconnect flow is inconsistent**

***

# 1. 🚨 Client lifecycle & double‑close bugs (CRITICAL)

### The biggest problem in this file

You have **multiple independent shutdown paths** that all:

*   call `SafeClose`
*   close `c.Out`
*   close `c.Conn`

#### Example

```go
if c.SafeClose() {
    c.disconnect(s, true)
}
```

But `disconnect()` does this:

```go
close(c.Out)
c.Conn.Close()
```

And **other code paths do the same thing**.

### What this causes

*   `panic: close of closed channel`
*   races between Reader/Writer goroutines
*   undefined behavior under load

### Root cause

**No single owner of the client lifecycle**

### ✅ Fix (mandatory design rule)

> **Only `Client.SafeClose()` may close resources.**

Refactor so:

*   `SafeClose()` = *idempotent, owns shutdown*
*   Everyone else only *requests* shutdown

#### Correct pattern

```go
func (c *Client) SafeClose() bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.closed {
        return false
    }
    c.closed = true
    close(c.Out)
    c.Conn.Close()
    return true
}
```

And change:

```go
func (c *Client) disconnect(...) {
    if !c.SafeClose() {
        return
    }
    ...
}
```

✅ **This single change fixes \~50% of your stability issues**

***

# 2. Scanner usage is unsafe ⚠️

```go
scanner := bufio.NewScanner(c.Conn)
```

### Problem

`bufio.Scanner` has a **hard 64K token limit**, and failure is silent.

Even with `MaxMessageSize = 128`, this still affects:

*   handshake
*   commands
*   malformed packets

### ✅ Fix

Use `bufio.Reader` consistently:

```go
reader := bufio.NewReader(c.Conn)
line, err := reader.ReadString('\n')
```

This also aligns with your Writer logic.

***

# 3. Locking while sending on channels ⚠️

### Example

```go
c.currentRoom.Messages <- ChatMessage{...}
```

If:

*   the room broadcaster is slow
*   message buffer fills

➡️ Your **Reader goroutine stalls** while holding logic state

### Worse examples

```go
room.Mu.Lock()
if client, exists := room.Clients[targetUsername]; exists {
    targetClient = client
    room.Mu.Unlock()
    break
}
room.Mu.Unlock()
```

This lock‑dance across rooms is fragile.

### ✅ Fix principle

> **Never hold a mutex while sending to a channel**

Snapshot data under lock, then act.

***

# 4. `/dm` lookup is expensive and racy ⚠️

```go
rooms := s.ListRooms()
for _, roomName := range rooms {
    room, _ := s.GetRoom(roomName)
    room.Mu.Lock()
    if client, exists := room.Clients[targetUsername]; exists {
        targetClient = client
        room.Mu.Unlock()
        break
    }
    room.Mu.Unlock()
}
```

### Issues

*   O(rooms × users)
*   Repeated lock/unlock
*   Username collision ambiguity (multiple rooms)
*   Race if user disconnects right after lookup

### ✅ Fix

Maintain a \**server‑global username → *Client map**:

```go
Server.Users map[string]*Client
```

Updated on:

*   connect
*   /nick
*   disconnect

Then `/dm` becomes O(1).

***

# 5. `currentRoom` is not synchronized ❌

You read/write:

```go
c.currentRoom = newRoom
```

While:

*   Writer goroutine may still send
*   Disconnect logic may read it
*   Server may act on it

### ✅ Fix

Either:

*   Guard `currentRoom` with `c.mu`
*   Or make `currentRoom` immutable during command processing

Minimal fix:

```go
c.mu.Lock()
c.currentRoom = newRoom
c.mu.Unlock()
```

***

# 6. `/switch` command does too much inline ⚠️

`/switch` currently:

*   removes client from old room
*   inserts into new room
*   handles username conflicts
*   emits announcements
*   updates server stats
*   updates client state

This is **server responsibility**, not client I/O logic.

### ✅ Fix

Move into:

```go
func (s *Server) SwitchRoom(c *Client, roomName string) error
```

Cleaner, safer, testable.

***

# 7. `/nick` command race & scope issues ⚠️

```go
c.currentRoom.Mu.Lock()
delete(c.currentRoom.Clients, oldName)
c.Username = newName
c.currentRoom.Clients[newName] = c
c.currentRoom.Mu.Unlock()
```

### Issues

*   Server‑wide username uniqueness not enforced
*   DM index (if added) not updated
*   `c.Username` changed while Writer may be formatting messages

### ✅ Fix

*   Update username under `c.mu`
*   Route rename through server

***

# 8. Spam protection is well done ✅ (minor tweak)

### Good

*   Time‑based cooldown
*   Escalation to ban
*   Server‑level enforcement

### Improvement

Use monotonic time only:

```go
time.Since(c.lastMessageAt)
```

✅ already correct

Optional UX improvement:

*   Send remaining cooldown time

***

# 9. Accept loop & shutdown ⚠️

```go
func (s *Server) AcceptLoop(ln net.Listener)
```

*   No way to shut this down
*   Listener is never closed
*   Goroutines leak on shutdown

### ✅ Fix (future)

Use `context.Context` and a `Close()` method on `Server`.

***

# 10. Design clarity (important but not urgent)

Right now:

*   `Server` manages rooms
*   `Room` manages clients
*   `Client` mutates everything

This works, but it’s fragile.

### Long‑term ideal

*   **Server owns state changes**
*   `Client.Reader` parses commands → calls server methods
*   `Room` becomes mostly passive

You’re *very* close to this already.

***

# Summary Scorecard

| Area                 | Verdict       |
| -------------------- | ------------- |
| Core functionality   | ✅ Strong      |
| Protocol handling    | ✅ Thoughtful  |
| Concurrency safety   | ❌ Needs fixes |
| Lifecycle management | ❌ Broken      |
| Scalability          | ⚠️ Acceptable |
| Maintainability      | ⚠️ Medium     |
| Production readiness | ❌ Not yet     |

***

# Final verdict

This is **ambitious, well‑thought‑out code**, and it shows good Go instincts. The major problems are **not conceptual**, they’re **lifecycle and concurrency discipline issues**, which are very common at this stage.

### ✅ Fix these first (in order)

1.  Centralize client shutdown in `SafeClose()`
2.  Stop closing channels/connections elsewhere
3.  Replace `bufio.Scanner`
4.  Avoid mutex + channel send
5.  Synchronize `currentRoom`
6.  Move `/switch` & `/nick` logic into `Server`

