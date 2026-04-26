"internal/chat/room.go"

## High-level assessment

✅ **What’s good**

*   Clear responsibility separation (`Room` owns clients & messages)
*   Proper mutex use around shared maps
*   Buffered channels to avoid blocking
*   Logical API surface (`AddClient`, `RemoveClient`, `SendHistory`)
*   Deterministic user ordering via `slices.Sort`
*   Server-generated join/leave messages are cleanly modeled

⚠️ **Primary issues**

*   Double-closing client resources (serious bug)
*   Lock + channel-send combination (deadlock risk)
*   Inconsistent ownership of client lifecycle
*   `Room.Messages` channel is never consumed here
*   History growth is unbounded
*   Redundant / overlapping APIs with `Server`

***

## 1. Client lifecycle ownership ❌ (most serious issue)

### Problem

```go
if !client.SafeClose() {
	return false
}
...
close(client.Out)
client.Conn.Close()
```

This is **dangerous and incorrect** given your existing `Client.SafeClose()`:

*   `SafeClose()` already marks the client closed
*   `close(client.Out)` is done **outside** of `Client`
*   `client.Conn.Close()` is also done here

➡️ This violates **single-owner lifecycle control** and will cause:

*   double-closes
*   panics (`close of closed channel`)
*   subtle race conditions

### Fix ✅

**Only `Client` should close its own resources.**  
`Room` should *request* shutdown, not perform it.

✅ Correct version:

```go
func (r *Room) DisconnectClientFromRoom(username string) bool {
	r.Mu.Lock()
	client, exists := r.Clients[username]
	if !exists {
		r.Mu.Unlock()
		return false
	}
	delete(r.Clients, username)
	r.Mu.Unlock()

	if !client.SafeClose() {
		return false
	}

	log.Printf("%s left room %s\n", username, r.Name)
	return true
}
```

And ensure `SafeClose()` does:

```go
close(c.Out)
c.Conn.Close()
```

***

## 2. Locking while sending on channels ⚠️

### Problem

```go
func (r *Room) SendHistory(c *Client) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	for _, msg := range r.History {
		c.Out <- msg.FormatMessage()
	}
}
```

You are:

*   holding `r.Mu`
*   sending on a potentially blocking channel

If the client’s write loop stalls → **deadlock**

### Fix ✅

Always snapshot under lock, send outside:

```go
func (r *Room) SendHistory(c *Client) {
	r.Mu.Lock()
	history := append([]ChatMessage(nil), r.History...)
	r.Mu.Unlock()

	for _, msg := range history {
		select {
		case c.Out <- msg.FormatMessage():
		default:
			return
		}
	}
}
```

***

## 3. Unbounded history growth ❌

```go
History []ChatMessage
```

There is:

*   no limit
*   no pruning
*   no rotation

This will eventually consume unbounded memory.

### Fix ✅

Add a constant:

```go
const MaxRoomHistory = 128
```

And enforce:

```go
r.History = append(r.History, msg)
if len(r.History) > MaxRoomHistory {
	r.History = r.History[1:]
}
```

***

## 4. `Room.Messages` channel ownership ⚠️

```go
Messages chan ChatMessage
```

`Room` **sends** on this channel (`AnnounceJoin/Leave`) but:

*   No goroutine in this file consumes it
*   Responsibility unclear (Server? Room loop?)

This is fine **only if**:

*   There is exactly one goroutine elsewhere draining it forever

If not → messages will block when the buffer fills (128).

### Recommendation ✅

Document ownership clearly or add a room loop:

```go
go r.Run()
```

With a consumer:

```go
for msg := range r.Messages {
	broadcast(msg)
}
```

***

## 5. AddClient / RemoveClient duplication ⚠️

You have:

*   `Room.AddClient`
*   `Room.RemoveClient`
*   `Room.DisconnectClientFromRoom`
*   `Server.RegisterClientInRoom`
*   `Server.DisconnectClientFromRoom`

This creates **API ambiguity** and inconsistent logic paths.

### Recommendation ✅

Pick **one owner** for membership rules:

✅ Best option:

*   `Server` handles *membership*
*   `Room` is a dumb container

Then remove:

*   `Room.AddClient`
*   `Room.RemoveClient`
*   `Room.DisconnectClientFromRoom`

Or make those unexported and used only by `Server`.

***

## 6. Mutex scope is good ✅

Examples like:

```go
func (r *Room) ClientCount() int
func (r *Room) GetClients() []string
```

✅ Correct:

*   Small lock scope
*   Safe access
*   Deterministic output

This part is well-written.

***

## 7. Minor design notes

### ✅ `UserUpdates`

Good pattern. Buffered, non-blocking.

### ⚠️ Logging inside lock

```go
log.Printf("%s left room %s\n", username, r.Name)
```

Log calls can block (I/O).  
You already fixed this in earlier reviews—apply same rule here.

***

## Summary scorecard

| Area                 | Verdict                         |
| -------------------- | ------------------------------- |
| Structure            | ✅ Clean                         |
| Concurrency          | ⚠️ Channel + lock risks         |
| Resource lifecycle   | ❌ Broken (double-close)         |
| Memory safety        | ❌ Unbounded history             |
| API clarity          | ⚠️ Overlapping responsibilities |
| Production readiness | ⚠️ Needs fixes                  |

***

## Final verdict

This is **good foundational code** with solid intent, but there are two **must-fix issues** before this is safe:

1.  **Client lifecycle must be owned by `Client` only**
2.  **Never send to channels while holding a mutex**

Fix those, cap your history size, and clarify who owns `Room.Messages`, and this becomes a **clean, reliable room implementation**.

