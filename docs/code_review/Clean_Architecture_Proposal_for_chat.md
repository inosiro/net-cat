# Clean Architecture Proposal for TCP-Chat

## Design goals

1.  **Single owner per resource**
2.  **Clear write ownership**
3.  **No mutex + channel send**
4.  **Idempotent, centralized shutdown**
5.  **Race-detector–clean**
6.  **Easy to reason about under load**

***

# High-level architecture

    +------------------+
    |   TCP Listener   |
    +------------------+
             |
             v
    +------------------+
    |     Server       |  ← authoritative owner
    |------------------|
    | rooms            |
    | clients          |
    | bans             |
    +------------------+
       |        |
       |        |
       v        v
    +------+  +------+
    | Room |  | Room |
    +------+  +------+
       |
       v
    +------------------+
    | RoomBroadcaster  |  ← single writer
    +------------------+
             |
             v
    +------------------+
    |     Client       |  ← owns Conn + Out
    +------------------+

***

# Core ownership rules (immutable laws)

These rules *must never be violated*. Everything else follows from them.

***

## Rule 1: Client owns its resources

**Client owns:**

*   `net.Conn`
*   `Out chan string`
*   its own closed state

✅ Only `Client.SafeClose()` may:

*   close the connection
*   close `Out`
*   mark client closed

❌ No other struct may close:

*   `Conn`
*   `Out`

***

## Rule 2: Server owns membership and lifecycle

**Server owns:**

*   room membership
*   switching rooms
*   disconnect decisions
*   username uniqueness
*   global user index

The Server is the **only authority** that may:

*   add/remove a client from a room
*   move clients between rooms
*   initiate client shutdown due to policy (spam, bans)

***

## Rule 3: RoomBroadcaster is the sole room writer

Each `Room` has **exactly one goroutine** that:

*   reads room events
*   fans out messages
*   appends history

No other goroutine:

*   sends messages to all clients
*   modifies room state directly

This removes almost all locking complexity.

***

# Revised data model

### Client (lightweight, safe)

```go
type Client struct {
    Conn     net.Conn
    Out      chan string
    Username string

    mu     sync.Mutex
    closed bool
}
```

Responsibilities:

*   TCP write loop
*   safe shutdown
*   no room logic

***

### Server (authoritative coordinator)

```go
type Server struct {
    mu sync.Mutex

    rooms   map[string]*Room
    users   map[string]*Client  // GLOBAL username index
    bans    map[string]time.Time
}
```

Responsibilities:

*   accept connections
*   register/unregister clients
*   route commands
*   maintain invariants

***

### Room (passive container)

```go
type Room struct {
    name string

    events chan RoomEvent
    quit   chan struct{}
}
```

Room does **not** export:

*   `Clients`
*   `History`

Those live *inside the broadcaster goroutine*.

***

# Event-driven RoomBroadcaster (key change)

### RoomEvent

```go
type RoomEvent struct {
    Type    EventType
    Client  *Client
    Message ChatMessage
}
```

### RoomBroadcaster loop

```go
func (r *Room) run() {
    clients := map[string]*Client{}
    history := []ChatMessage{}

    for {
        select {
        case ev := <-r.events:
            switch ev.Type {
            case Join:
                clients[ev.Client.Username] = ev.Client
            case Leave:
                delete(clients, ev.Client.Username)
            case Message:
                history = append(history, ev.Message)
                broadcast(clients, ev.Message)
            }
        case <-r.quit:
            return
        }
    }
}
```

✅ No mutex  
✅ No external mutation  
✅ Deterministic order  
✅ History bounded here

***

# Message flow (example: chat message)

    Client.Reader
       ↓
    Server.HandleClientMessage(client, text)
       ↓
    server.currentRoom.events <- MessageEvent
       ↓
    RoomBroadcaster
       ↓
    for each client → non-blocking send

👉 **Clients never broadcast directly**

***

# Disconnect flow (clean & safe)

### Any goroutine may request shutdown:

```go
server.Disconnect(client, ReasonSpam)
```

### Server handles it:

```go
func (s *Server) Disconnect(c *Client, reason Reason) {
    if !c.SafeClose() {
        return
    }

    s.removeFromRoom(c)
    s.removeUserIndex(c)
}
```

✅ Idempotent  
✅ Centralized  
✅ No double close  
✅ No race

***

# What gets removed compared to current code

✅ Removed

*   `Room.Mu`
*   `Server.Mu` nesting with room locks
*   `Room.DisconnectClientFromRoom`
*   `Client.currentRoom` mutation without lock
*   channel sends under mutex
*   broadcaster-triggered disconnects

✅ Simplified

*   `/switch`
*   `/nick`
*   `/dm`
*   spam banning
*   join/leave announcements

***

# Why this architecture works

| Problem in current code | Solved by                    |
| ----------------------- | ---------------------------- |
| Double-close panics     | Single `SafeClose()` owner   |
| Deadlocks               | No mutex + channel send      |
| Racy room state         | Single broadcaster goroutine |
| Complex locking         | Event queues                 |
| Goroutine leaks         | Explicit quit channels       |
| Debug difficulty        | Deterministic flow           |

***

# Migration strategy (important)

You **do not** need to rewrite everything at once.

### Step-by-step migration plan

1.  Fix `Client.SafeClose()` ownership
2.  Stop closing channels elsewhere
3.  Move all room broadcasts into `RoomBroadcaster`
4.  Route room joins/leaves through events
5.  Remove room mutex entirely
6.  Introduce server-wide user map
7.  Replace `Scanner` with `bufio.Reader`

Each step is independently valuable and testable.
