Below is a **full refactor of your UI into a true single-writer event loop**, deliberately modeled after the **room broadcaster pattern** you now use on the server.

This is not a cosmetic change — it **eliminates UI data races**, simplifies reasoning, and makes future features (users list, rooms, system messages) much easier to add safely.

***

# Goal

✅ **Exactly one goroutine mutates UI state**  
✅ Network code never touches UI state directly  
✅ gocui remains the only place actual drawing happens  
✅ Order is deterministic  
✅ No locks needed

> Same rule as the server: **the UI owns its state**

***

# 1. High-level architecture (before vs after)

## ❌ Before (current)

    network goroutine → ui.chatHistory append
    keybinding goroutine → ui.chatHistory append
    gocui.Update → reads ui.chatHistory

Problems:

*   Data races on slices
*   Hard to reason about ordering
*   UI bugs under load

***

## ✅ After (single-writer UI)

    network → UIEvent channel  ┐
    keyboard → UIEvent channel ├─► UI.Run() ─► gocui.Update()
    timers → UIEvent channel   ┘

✅ One writer  
✅ One ordering authority  
✅ No shared mutable state

***

# 2. UI state (owned by UI goroutine only)

```go
type UI struct {
    g *gocui.Gui

    // event input
    events chan UIEvent

    // owned state (single‑writer)
    chatHistory []string
    rooms       []string
    users       []string
}
```

**Important rule**  
🚫 Nothing outside `UI.Run()` may mutate `chatHistory`, `rooms`, or `users`.

***

# 3. UI events (internal protocol)

This is how *everything* talks to the UI.

```go
type UIEventType int

const (
    UIEventChatMessage UIEventType = iota
    UIEventRoomsUpdate
    UIEventUsersUpdate
    UIEventQuit
)

type UIEvent struct {
    Type  UIEventType
    Text  string
    Items []string
}
```

***

# 4. Public UI API (safe to call from any goroutine)

Network code, keybindings, timers — **all use this**.

```go
func (ui *UI) PostChatMessage(msg string) {
    ui.events <- UIEvent{
        Type: UIEventChatMessage,
        Text: msg,
    }
}

func (ui *UI) PostRooms(rooms []string) {
    ui.events <- UIEvent{
        Type:  UIEventRoomsUpdate,
        Items: rooms,
    }
}

func (ui *UI) PostUsers(users []string) {
    ui.events <- UIEvent{
        Type:  UIEventUsersUpdate,
        Items: users,
    }
}

func (ui *UI) Quit() {
    ui.events <- UIEvent{Type: UIEventQuit}
}
```

✅ No locks  
✅ No gocui calls  
✅ No state mutation

***

# 5. ✅ The single‑writer UI loop

This is the heart of the refactor.

```go
func (ui *UI) Run() error {
    for {
        select {
        case ev := <-ui.events:
            switch ev.Type {

            case UIEventChatMessage:
                ui.handleChatMessage(ev.Text)

            case UIEventRoomsUpdate:
                ui.handleRoomsUpdate(ev.Items)

            case UIEventUsersUpdate:
                ui.handleUsersUpdate(ev.Items)

            case UIEventQuit:
                return gocui.ErrQuit
            }
        }
    }
}
```

✅ All UI state changes happen here  
✅ Sequential, deterministic, testable

***

# 6. Event handlers (mutate state + schedule redraw)

### Chat messages

```go
func (ui *UI) handleChatMessage(msg string) {
    ui.chatHistory = append(ui.chatHistory, msg)
    if len(ui.chatHistory) > 64 {
        ui.chatHistory = ui.chatHistory[1:]
    }

    ui.g.Update(func(g *gocui.Gui) error {
        v, err := g.View("chat")
        if err != nil {
            return nil
        }
        fmt.Fprintln(v, msg)
        return nil
    })
}
```

***

### Rooms list

```go
func (ui *UI) handleRoomsUpdate(rooms []string) {
    ui.rooms = append([]string(nil), rooms...)

    ui.g.Update(func(g *gocui.Gui) error {
        v, err := g.View("rooms")
        if err != nil {
            return nil
        }
        v.Clear()
        for _, r := range ui.rooms {
            fmt.Fprintln(v, r)
        }
        return nil
    })
}
```

***

### Users list (future-proofed)

```go
func (ui *UI) handleUsersUpdate(users []string) {
    ui.users = append([]string(nil), users...)

    ui.g.Update(func(g *gocui.Gui) error {
        v, err := g.View("users")
        if err != nil {
            return nil
        }
        v.Clear()
        for _, u := range ui.users {
            fmt.Fprintln(v, u)
        }
        return nil
    })
}
```

***

# 7. UI startup (Run + MainLoop)

```go
func (ui *UI) Start() error {
    ui.g.SetManagerFunc(ui.layout)
    ui.bindKeys()

    go func() {
        _ = ui.Run() // exits via ErrQuit
    }()

    if err := ui.g.MainLoop(); err != nil && err != gocui.ErrQuit {
        return err
    }
    return nil
}
```

✅ UI loop controls state  
✅ gocui loop controls rendering lifecycle

***

# 8. ✅ Updating `UIClient.reader()` (important change)

Old behavior (wrong):

```go
ui.AddChatMessage(...)
ui.UpdateRooms(...)
```

New behavior (correct):

```go
c.ui.PostChatMessage(line)
c.ui.PostRooms(rooms)
c.ui.Quit()
```

📌 **Network code never touches UI state directly.**

***

# 9. Why this matches your server design perfectly

| Server rooms            | UI                      |
| ----------------------- | ----------------------- |
| Single writer goroutine | Single writer goroutine |
| Events via channels     | Events via channels     |
| Broadcaster owns state  | UI owns state           |
| No mutexes              | No mutexes              |
| Deterministic order     | Deterministic order     |

Your mental model is now **consistent across client and server**.

***

# Final takeaway

This refactor gives you:

✅ Race-free UI  
✅ Clean separation of concerns  
✅ Exact same concurrency philosophy end-to-end  
✅ Easier future protocol changes  
✅ Easier testing

> You didn't just fix bugs — you aligned your entire system around a single, coherent concurrency model.
