Below is a **clean, explicit protocol design** that you can introduce **incrementally**, keeps **backward compatibility**, and fits perfectly with your **single-writer server + single-writer UI** architecture.

The goal is to move from *“infer meaning from text”* → *“parse intent from markers”* **without breaking existing clients**.

***

# Design goals

✅ Human-readable (still usable over telnet / nc)  
✅ Line-oriented (Scanner friendly)  
✅ Explicit markers (no string guessing)  
✅ Backward compatible  
✅ Easy to extend (v2, v3…)  
✅ Maps cleanly to UI and room event loops

***

# 1. Core protocol rule

> **Every server-generated control payload starts with a marker line, and ends with `END` unless it is single-line.**

Markers are **ALL-CAPS**, start with `@`.

***

# 2. Protocol message types

## 2.1 Chat message

### Server → Client

    @MSG
    alice: Hello everyone

*   Always exactly **2 lines**
*   Human-readable
*   No `END` needed (fixed size)

✅ Old clients still work (they just see the text)

***

## 2.2 System message

### Server → Client

    @SYS
    alice joined the room

or

    @SYS
    bob left the room

✅ UI can now distinguish system events without parsing text  
✅ Still readable

***

## 2.3 Room list

### Server → Client

    @ROOMS
    Main Room (3 users)
    Gaming (1 users)
    END

✅ Explicit start  
✅ Explicit end  
✅ No indentation hacks

***

## 2.4 User list (new, authoritative)

### Server → Client

    @USERS
    alice
    bob
    charlie
    END

✅ No guessing join/leave  
✅ UI can reset state safely  
✅ Re-syncs after reconnect

***

## 2.5 Errors

### Server → Client

    @ERR
    room does not exist

✅ Structured  
✅ Still readable

***

## 2.6 Optional future markers (designed now)

You don't need them yet, but this protocol **allows clean growth**:

    @JOINED room-name
    @LEFT room-name
    @INFO arbitrary text
    @PING
    @PONG

***

# 3. Server-side changes (minimal & safe)

## 3.1 ChatMessage formatting

```go
func (m ChatMessage) FormatProtocol() []string {
    return []string{
        "@MSG",
        m.Username + ": " + m.Text,
    }
}
```

Broadcast as:

```go
for _, line := range msg.FormatProtocol() {
    c.Out <- line
}
```

***

## 3.2 System messages (join/leave)

Replace:

```go
"[system] alice joined the room"
```

With:

```go
func SystemJoined(username string) []string {
    return []string{
        "@SYS",
        username + " joined the room",
    }
}
```

Same for leave.

***

## 3.3 Room list broadcaster

```go
func (s *Server) SendRoomList(c *Client) {
    c.Out <- "@ROOMS"
    for _, r := range s.Rooms {
        c.Out <- fmt.Sprintf("%s (%d users)", r.Name, len(r.Clients))
    }
    c.Out <- "END"
}
```

✅ No fragile indentation  
✅ Order preserved

***

## 3.4 User list broadcaster (new)

When someone joins or leaves a room:

```go
func (r *Room) broadcastUsers() {
    for _, c := range r.clients {
        c.Out <- "@USERS"
        for username := range r.clients {
            c.Out <- username
        }
        c.Out <- "END"
    }
}
```

✅ Room remains single writer  
✅ UI state stays authoritative

***

# 4. UIClient.reader() with markers (clean & exact)

This is **dramatically simpler** than the inferred version.

```go
func (c *UIClient) reader() {
    scanner := bufio.NewScanner(c.conn)

    for scanner.Scan() {
        line := scanner.Text()

        switch line {

        case "@MSG":
            scanner.Scan()
            c.ui.PostChatMessage(scanner.Text())

        case "@SYS":
            scanner.Scan()
            text := scanner.Text()
            c.ui.PostChatMessage("[system] " + text)

        case "@ROOMS":
            rooms := []string{}
            for scanner.Scan() && scanner.Text() != "END" {
                rooms = append(rooms, scanner.Text())
            }
            c.ui.PostRooms(rooms)

        case "@USERS":
            users := []string{}
            for scanner.Scan() && scanner.Text() != "END" {
                users = append(users, scanner.Text())
            }
            c.ui.PostUsers(users)

        case "@ERR":
            scanner.Scan()
            c.ui.PostChatMessage("[error] " + scanner.Text())

        default:
            // backward-compatibility path
            c.ui.PostChatMessage(line)
        }
    }

    c.ui.Quit()
}
```

✅ No fragile suffix matching  
✅ No guessing  
✅ Fully deterministic

***

# 5. Backward compatibility strategy (important)

You **do not break old clients** if you:

1.  Continue sending plain chat lines
2.  Add markers **before** structured content
3.  Treat unknown lines as chat text

Old clients will see:

    @ROOMS
    Main Room (3 users)
    END

Which is ugly but safe.

New clients will parse it correctly.

***

# 6. Why this fits your architecture perfectly

| Aspect                 | Match                            |
| ---------------------- | -------------------------------- |
| Single-writer room     | Emits consistent protocol events |
| Single-writer UI       | Consumes ordered protocol events |
| No mutexes             | Channels + serialization         |
| Deterministic ordering | One goroutine per stream         |
| Future-proof           | Markers can evolve               |

This is essentially a **text-based event protocol**, not just “chat output”.

***

# 7. Recommended rollout plan

✅ Step 1 - Introduce markers on server  
✅ Step 2 - Update UIClient.reader()  
✅ Step 3 - Add `@USERS` broadcaster  
✅ Step 4 - Remove join/leave guessing  
✅ Step 5 - (Optional) add protocol version marker

Optional handshake:

    @HELLO 1

***

## Final takeaway

You've now evolved from:

> “Chat as text”

to:

> “Chat as a structured event stream”

…**without losing simplicity or debuggability**.

This is exactly the right protocol design for:

*   learning
*   correctness
*   incremental growth
*   concurrency-safe systems

If you want next, I can:

*   version this protocol cleanly (`@HELLO 1`)
*   compress multi-line sends safely
*   or help you add room switching with zero ambiguity
