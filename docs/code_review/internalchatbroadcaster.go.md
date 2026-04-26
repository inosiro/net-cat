"internal/chat/broadcaster.go"

# High-level assessment

✅ **What’s good**

*   Clear ownership: one goroutine per room
*   Correct conceptual model: single broadcaster serializes room state
*   Buffered channels reduce head‑of‑line blocking
*   Non‑blocking fan‑out to clients
*   History appended in one place ✅ (good design choice)
*   Separation between room events and server events (`AnnounceJoin/Leave`)

❌ **Critical issues**

*   Sending on client channels while holding `Room.Mu`
*   Calling `DisconnectClientFromRoom` from inside the broadcaster
*   Unbounded history growth
*   User list broadcast logic duplicates responsibility
*   No shutdown / lifecycle control

***

# 1. 🚨 Mutex + channel send = deadlock risk (CRITICAL)

### Problem

Inside the broadcaster:

```go
r.Mu.Lock()
for _, c := range r.Clients {
	select {
	case c.Out <- formatted:
	default:
		go r.DisconnectClientFromRoom(c.Username)
	}
}
r.History = append(r.History, msg)
r.Mu.Unlock()
```

You are:

*   holding `r.Mu`
*   sending on `c.Out`

Even with `default`, this is unsafe because:

*   another goroutine may hold `c.mu` and try to acquire `r.Mu`
*   disconnect logic also touches room state
*   lock inversion is likely under load

### ✅ Correct pattern

> **Never send to channels while holding a mutex**.

Refactor to **snapshot clients under lock, then release**:

```go
r.Mu.Lock()
clients := make([]*Client, 0, len(r.Clients))
for _, c := range r.Clients {
	clients = append(clients, c)
}
r.History = append(r.History, msg)
r.Mu.Unlock()

for _, c := range clients {
	select {
	case c.Out <- formatted:
	default:
		go r.requestDisconnect(c)
	}
}
```

***

# 2. ❌ Disconnecting clients from inside broadcaster

```go
go r.DisconnectClientFromRoom(c.Username)
```

This is extremely dangerous:

*   Broadcaster owns room state
*   `DisconnectClientFromRoom` also locks `r.Mu`
*   You are mutating the same structure *from two goroutines*

This breaks your **single-writer room model**.

### ✅ Correct design

The broadcaster **must not modify membership** directly.

Instead:

*   Send a signal/event
*   Or request server‑level disconnect

Example:

```go
func (r *Room) requestDisconnect(c *Client) {
	if c.SafeClose() {
		c.Server.DisconnectClientFromRoom(r, c.Username)
	}
}
```

Even better: route all disconnects through the `Server`.

***

# 3. Unbounded history growth ❌

```go
r.History = append(r.History, msg)
```

There is:

*   no cap
*   no pruning

This will grow forever.

### ✅ Fix

Define a cap (you already do this elsewhere):

```go
const MaxRoomHistory = 128
```

Enforce it:

```go
r.History = append(r.History, msg)
if len(r.History) > MaxRoomHistory {
	r.History = r.History[1:]
}
```

***

# 4. `UserUpdates` duplicates responsibilities ⚠️

This block:

```go
case <-r.UserUpdates:
	...
	for _, c := range r.Clients {
		select {
		case c.Out <- msg:
		default:
		}
	}
```

Concerns:

*   User list formatting belongs to **Server view logic**
*   Broadcaster already handles messages
*   This creates two message pathways to clients:
    *   `Room.Messages`
    *   `UserUpdates`

### ✅ Recommendation

Either:

*   Convert user updates into **system messages**:

```go
room.Messages <- ChatMessage{ User: "SERVER", ... }
```

Or:

*   Move user list broadcasting into server code

**Single message pipeline per room is cleaner.**

***

# 5. Shutdown semantics (missing)

```go
for {
	select {
	case msg, ok := <-r.Messages:
		if !ok {
			return
		}
```

Issues:

*   `UserUpdates` channel never checked for close
*   No way to stop broadcaster cleanly on server shutdown
*   Goroutine leak potential

### ✅ Suggested improvement

Use a `context.Context` or a `Done` channel:

```go
select {
case <-r.Done:
	return
case msg := <-r.Messages:
	...
}
```

***

# 6. Announcement helper functions ✅ (mostly good)

```go
func (s *Server) AnnounceJoin(room *Room, username string)
```

✅ Good:

*   Messages go through broadcaster
*   System messages are consistent
*   Time is centralized

⚠️ Minor improvements:

*   Avoid blocking if room is overloaded:

```go
select {
case room.Messages <- msg:
default:
}
```

*   Avoid ellipses (`...`)—cosmetic but inconsistent UX

***

# 7. Concurrency ownership (important design note)

You’re *almost* following a very strong pattern:

> **One goroutine owns room state**

But this file slightly breaks it by:

*   locking inside the broadcaster
*   letting other goroutines mutate room state

### ✅ Ideal model

*   `RoomBroadcaster` is the **only writer**
*   Other goroutines send **events**
*   No one else touches `r.Clients` or `r.History`

This would eliminate \~80% of race conditions.

***

# Summary Scorecard

| Area                      | Verdict       |
| ------------------------- | ------------- |
| Overall design            | ✅ Strong      |
| Broadcaster concept       | ✅ Correct     |
| Mutex discipline          | ❌ Unsafe      |
| Client disconnect logic   | ❌ Broken      |
| Memory safety             | ❌ Unbounded   |
| Responsibility separation | ⚠️ Mixed      |
| Production readiness      | ❌ Needs fixes |

***

# Final verdict

This is a **good architectural idea implemented unsafely**.

The concept of a room‑local broadcaster goroutine is **exactly right**, but:

### 🚨 Must‑fix before production

1.  Do **not** send on channels while holding `Room.Mu`
2.  Do **not** modify room membership inside the broadcaster
3.  Cap room history
4.  Centralize disconnect logic
5.  Add shutdown semantics

Once those are fixed, this becomes a **very solid, scalable room engine**.

