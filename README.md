# NetCat — TCP Group Chat

A TCP-based group chat server and client inspired by the Unix `nc` (NetCat) utility, written in Go. Supports multiple chat rooms, direct messages, nick changes, spam protection, IP banning, and an optional terminal UI.

---

## Features

- **TCP server** accepting up to 10 concurrent clients
- **Multiple chat rooms** (up to 256) with per-room message history (last 64 messages)
- **Linux logo banner** and username prompt on connect
- **Timestamped messages** in the format `[YYYY-MM-DD HH:MM:SS][username]: message`
- **Chat history replay** for new joiners
- **Join / leave announcements** broadcast to all room members
- **Empty message filtering** — blank lines are silently dropped
- **Nickname changes** (`/nick`) announced to the room
- **Room switching** with dynamic room creation (`/switch`)
- **Direct messages** (`/dm`)
- **Spam protection** — rate-limiting with auto-disconnect after repeated violations
- **IP banning** — banned IPs are rejected on reconnect
- **Session logging** to a file (`-o` flag)
- **Optional terminal UI** built with [gocui](https://github.com/jroimartin/gocui) (`--ui` flag)
- Graceful shutdown on `SIGINT`/`SIGTERM`

---

## Project Structure

```
.
├── main.go                  # Entry point — server & client modes
├── go.mod
├── internal/
│   ├── chat/
│   │   ├── server.go        # Server, room management, IP banning
│   │   ├── room.go          # Room: single-writer event loop, history
│   │   ├── broadcaster.go   # Room broadcaster goroutine
│   │   ├── client.go        # Client struct, SafeClose, Send
│   │   ├── network.go       # Reader, Writer, command handling
│   │   ├── message.go       # ChatMessage type, formatting
│   │   └── utils.go         # Username validation, banner, help text
│   └── ui/
│       ├── ui.go            # gocui-based terminal UI layout & keybindings
│       └── client.go        # UI network client (reader goroutine)
└── tests/
    ├── integration_test.go  # End-to-end client lifecycle & banning tests
    ├── broadcaster_test.go  # Broadcaster, announcements, non-blocking fanout
    ├── server_test.go       # Server init, room capacity, registration, history
    ├── client_test.go       # Client SafeClose
    ├── network_test.go      # Client Writer
    ├── message_test.go      # Message formatting
    └── utils_test.go        # Username validation
```

---

## Requirements

- **Go 1.22+**
- Dependencies (managed via `go.mod`)

Install dependencies:

```bash
go mod download
```

### Why the two indirect packages are required

The project directives allow only `github.com/jroimartin/gocui` as an external dependency. We initially tried to keep `go.mod` to that single line, but `go mod tidy` (and the build itself) rejects that — it refuses to compile unless every transitive dependency is explicitly pinned in `go.mod`. The two indirect entries are not optional extras; they are `gocui`'s own dependencies and Go modules requires them to be recorded for a reproducible build:

- **`github.com/nsf/termbox-go v1.1.1`** — the low-level terminal rendering engine that `gocui` is built on. It handles raw terminal I/O, keyboard/mouse events, and screen cells. `gocui` cannot function without it.
- **`github.com/mattn/go-runewidth v0.0.9`** — provides correct display-width for Unicode characters. Both `termbox-go` and `gocui` depend on it to measure and align text in the terminal grid.

Neither package is imported anywhere in our own source files (hence the `// indirect` comment), but omitting them from `go.mod` causes a build failure. They are there solely because Go modules mandates that the full dependency graph be recorded.

---
| `/rooms` | List all available rooms |
| `/users` | List users in the current room |
| `/dm <user> <message>` | Send a direct message |
| `/history` | Show the last 64 messages in the current room |
| `/stats` | Show server statistics |
| `/leave` | Disconnect from the server |
| `/help` | Show the help message |

## Usage

### Server

Start the server on the default port **8989**:

```bash
go run .
```

Start on a custom port:

```bash
go run . 2525
```

Start with session logging to a file:

```bash
go run . -o chat.log 2525
```

> If more than one positional argument is provided the program exits with:
> `[USAGE]: ./TCPChat $port`

### Client (plain `nc`)

```bash
nc localhost 8989
```

You will be greeted with the Linux logo banner and prompted for a username.

### Client (built-in plain mode)

```bash
go run . -c -s 127.0.0.1 8989
```

### Client (terminal UI)

```bash
go run . --ui 8989
```

The UI provides dedicated panes for the chat, room list, and user list, with mouse and keyboard control.

---

## In-Chat Commands

| Command | Description |
|---|---|
| `/nick <name>` | Change your nickname |
| `/switch <room>` | Switch to (or create) a room |
| `/rooms` | List all available rooms |
| `/users` | List users in the current room |
| `/dm <user> <message>` | Send a direct message |
| `/history` | Show the last 64 messages in the current room |
| `/stats` | Show server statistics |
| `/leave` | Disconnect from the server |
| `/help` | Show the help message |

---

## Limits & Constraints

| Setting | Value |
|---|---|
| Max simultaneous clients | 10 |
| Max rooms | 256 |
| Room message history | 64 messages |
| Max message size | 128 characters |
| Spam cooldown | 1 second between messages |
| Max spam attempts before kick | 5 |

---

## Running Tests

```bash
go test ./tests/... -v
```

Run a specific test:

```bash
go test ./tests/... -run TestIntegration_ClientLifecycle -v
```

---

## Example Session

**Server:**
```
$ go run . 2525
2026/05/04 16:00:00 Starting TCP-Chat server on port 2525
```

**Client 1 (Alice):**
```
$ nc localhost 2525
Welcome to TCP-Chat!
         _nnnn_
        dGGGGMMb
       @p~qp~~qMb
       M|@||@) M|
       @,----.JM|
      JS^\__/  qKL
     dZP        qKRb
    dZP          qKKb
   fZP            SMMb
   HZM            MMMM
   FqM            MMMM
 __| ".        |\dS"qML
 |    `.       | `' \Zq
_)      \.___.,|     .'
\____   )MMMMMP|   .'
     `-'       `--'
[ENTER YOUR NAME]: Alice
[2026-05-04 16:00:05][Alice]:Hello!
Bob has joined our chat...
[2026-05-04 16:00:12][Bob]:Hey Alice!
```

**Client 2 (Bob):**
```
$ nc localhost 2525
...
[ENTER YOUR NAME]: Bob
[2026-05-04 16:00:05][Alice]:Hello!
[2026-05-04 16:00:12][Bob]:Hey Alice!
```

---

## License

This project was completed as part of the [Zone01](https://zone01.org) curriculum.
