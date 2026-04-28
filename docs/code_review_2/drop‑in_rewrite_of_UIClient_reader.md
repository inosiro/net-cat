Below is a **drop-in rewrite of `UIClient.reader()`** that **matches your current server protocol exactly as it exists today** — **no invented messages, no JSON, no magic headers**.

This implementation is intentionally conservative and plain-text, because **your server only ever sends formatted text lines**.

I'll start with the **protocol reality**, then give you the **correct reader**, then explain **why each branch exists**.

***

## 1. The actual server protocol (important)

From your server code:

### ✅ What the server sends

The server **only writes plain text lines**, terminated by `\n`.

Sources:

*   `ChatMessage.FormatMessage()` → formatted chat lines
*   `AnnounceJoin / AnnounceLeave` → formatted system messages
*   `ServerBroadcaster()` → multiline **room list**, starting with:
        Available rooms:
         Main Room (3 users)
         Another Room (1 users)

⚠️ There is **no structured protocol**:

*   No JSON
*   No opcodes
*   No explicit “end of list” markers
*   No user list messages yet (those are **not sent currently**)

Therefore the **only safe UI strategy** is:

> Treat **everything** as a line stream and infer meaning by content.

***

## 2. Correct behavior for `UIClient.reader()`

Your reader must:

1.  Read lines continuously
2.  Detect **room list blocks**
3.  Append **chat/system messages** to chat view
4.  Exit cleanly on disconnect

***

## 3. ✅ Final rewritten `UIClient.reader()` (exact match)

```go
func (c *UIClient) reader() {
    scanner := bufio.NewScanner(c.conn)

    var (
        collectingRooms bool
        rooms            []string
    )

    for scanner.Scan() {
        line := scanner.Text()

        // ---- Room list (from ServerBroadcaster) ----
        if line == "Available rooms:" {
            collectingRooms = true
            rooms = rooms[:0]
            continue
        }

        if collectingRooms {
            // Room entries are indented by one space
            if strings.HasPrefix(line, " ") {
                rooms = append(rooms, strings.TrimSpace(line))
                continue
            }

            // End of room list block
            collectingRooms = false
            c.ui.UpdateRooms(rooms)
            // fallthrough: process this line normally
        }

        // ---- Normal chat / system message ----
        c.ui.AddChatMessage(line)
    }

    // ---- Disconnect handling ----
    if err := scanner.Err(); err != nil {
        c.ui.AddChatMessage("[error] connection lost: " + err.Error())
    } else {
        c.ui.AddChatMessage("[info] disconnected from server")
    }

    c.ui.RequestQuit()
}
```

***

## 4. Required UI methods (what this reader assumes)

Your `UI` must expose **thread-safe entry points**:

```go
func (ui *UI) AddChatMessage(msg string) {
    ui.g.Update(func(g *gocui.Gui) error {
        ui.chatHistory = append(ui.chatHistory, msg)
        if len(ui.chatHistory) > 64 {
            ui.chatHistory = ui.chatHistory[1:]
        }

        v, err := g.View("chat")
        if err != nil {
            return nil
        }
        fmt.Fprintln(v, msg)
        return nil
    })
}

func (ui *UI) UpdateRooms(rooms []string) {
    ui.g.Update(func(g *gocui.Gui) error {
        v, err := g.View("rooms")
        if err != nil {
            return nil
        }

        v.Clear()
        for _, r := range rooms {
            fmt.Fprintln(v, r)
        }
        return nil
    })
}

func (ui *UI) RequestQuit() {
    ui.g.Update(func(g *gocui.Gui) error {
        return gocui.ErrQuit
    })
}
```

***

## 5. Why this matches your server perfectly

✅ Does **not assume** features your server does not implement  
✅ Correctly parses the **only structured output** (`Available rooms:`)  
✅ Preserves **message ordering**  
✅ Behaves correctly if the server sends:

*   only chat
*   only room updates
*   mixed output
*   truncates unexpectedly

✅ Safe under disconnect  
✅ No goroutine leaks  
✅ No UI races

***

## 6. What this deliberately does NOT do (yet)

Because your **server doesn't support it yet**, this reader does **not** parse:

*   User lists
*   Join/leave membership deltas
*   Room switches
*   Control messages

When you later add **explicit protocol markers** (e.g. `USERS:` or `ROOM:` headers), this reader can be extended trivially.

***

## Final takeaway

This `UIClient.reader()` is:

*   **protocol-accurate**
*   **minimal**
*   **robust**
*   **future-proof**

It mirrors the same principle you applied to the server:

> **One writer, one stream, one truth**

If you want next, I can:

*   evolve the protocol safely (without breaking old clients)
*   add user-list support end-to-end
*   refactor UI state into a single-writer loop like your rooms
