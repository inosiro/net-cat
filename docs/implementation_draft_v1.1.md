# ✅ **Implementation Plan for NetCat‑Style Go Chat Server (Draft 1.1)**

# 1. **High‑Level Architecture**

## **Server Components**

1.  **TCP Listener**
    *   Listens on a port (default: `8989`).
    *   Accepts incoming connections.
    *   Rejects new connections when client count ≥ 10.

2.  **Client Handler**  
    For each connected client:
    *   **Reader goroutine**: reads input → sends to server’s message channel.
    *   **Writer goroutine**: reads from channel → writes to TCP connection.

3.  **Broadcaster Goroutine**
    *   The *only* goroutine distributing messages to all clients.
    *   Appends messages to server history.
    *   Handles join/leave event broadcasting.

4.  **Message History**
    *   Stored in a slice.
    *   Sent to new clients after they register their username.

## **Client Components**

*   Connect to server and submit username.
*   Receive chat history.
*   Read from stdin / write to server.
*   Receive messages from server and print to stdout.

# 2. **Data Structures**

## **Server**

```go
type Server struct {
    clients  map[string]*Client
    mu       sync.Mutex
    messages chan ChatMessage
    history  []ChatMessage
}
```

## **Client**

```go
type Client struct {
    conn     net.Conn
    username string
    out      chan string
    closed   bool
    mu       sync.Mutex
}
```

## **Chat Message**

```go
type ChatMessage struct {
    Timestamp time.Time
    User      string
    Text      string
}
```

# 3. **Client Connection Lifecycle**

## **Sequence**

    Connected → Welcome Banner → AwaitUsername → Validate → Add to Map → Send History → Broadcast Join → Active

### join order:
1.  Send Welcome Banner (ASCII Art).
2.  Max connection check.
3.  Read username.
4.  Validate (non‑empty, unique, valid format).
5.  Add client to map.
6.  Send message history **only to that client**.
7.  Broadcast `"X joined the chat"` via `messages <-`.

# 4. **Algorithms**

## **A. Join Algorithm**

### Max Connections Check (in main loop):
```go
s.mu.Lock()
if len(s.clients) >= 10 {
    s.mu.Unlock()
    conn.Write([]byte("Chat is full. Try again later.\n"))
    conn.Close()
    return
}
s.mu.Unlock()
```


```go
name, err := reader.ReadString('\n')
if err != nil {
    conn.Close()
    return
}
name = strings.TrimRight(name, "\r\n")

if name == "" || len(name) > 32 {
    conn.Write([]byte("Invalid username\n"))
    conn.Close()
    return
}

s.mu.Lock()
if _, exists := s.clients[name]; exists {
    s.mu.Unlock()
    conn.Write([]byte("Username already exists\n"))
    conn.Close()
    return
}

client := &Client{
    conn:     conn,
    username: name,
    out:      make(chan string, 32),
}

s.clients[name] = client
s.mu.Unlock()
```

**Better approach**: Username validation too permissive. Should reject control characters, newlines, etc.

### Send history:

```go
s.mu.Lock()
for _, msg := range s.history {
    client.out <- format(msg)
}
s.mu.Unlock()
```

**Should add**: max history items constant

### Broadcast join:

```go
s.messages <- ChatMessage{
    Timestamp: time.Now(),
    User:      "SERVER",
    Text:      fmt.Sprintf("%s has joined our chat...", name),
}
```

## **B. Broadcasting (Non‑Blocking Fan‑Out)**

The broadcaster goroutine:

```go
func (s *Server) broadcaster() {
    for msg := range s.messages {
        s.mu.Lock()

        for _, c := range s.clients {
            select {
            case c.out <- format(msg):
            default:
                // Slow or dead client → disconnect safely
                go s.DisconnectClient(c)
            }
        }

        s.history = append(s.history, msg)
        s.mu.Unlock()
    }
}
```

Properties:

*   Prevents deadlock if a client stops reading.
*   Only this goroutine writes to history.
*   Ordered delivery to all clients.

**Better approach**: `go s.DisconnectClient(c)`
Have broadcaster track failed sends and mark clients for cleanup, letting the reader/close mechanism handle it exclusively

## **C. Client Reader (reads from TCP)**

```go
func (c *Client) reader(s *Server) {
    scanner := bufio.NewScanner(c.conn)

    for scanner.Scan() {
        text := strings.TrimSpace(scanner.Text())
        if text == "" {
            continue
        }

        s.messages <- ChatMessage{
            Timestamp: time.Now(),
            User:      c.username,
            Text:      text,
        }
    }

    if c.SafeClose() {
        s.DisconnectClient(c)
    }
}
```

## **D. Client Writer (writes to TCP)**


```go
func (c *Client) writer() {
    for msg := range c.out {
        _, err := c.conn.Write([]byte(msg + "\n"))
        if err != nil {
            return
        }
    }
}
```

## **E. Leave / Disconnect Algorithm**

```go
func (s *Server) DisconnectClient(c *Client) {
    if !c.SafeClose() { return }

    s.mu.Lock()
    delete(s.clients, c.username)
    close(c.out)
    s.mu.Unlock()

    c.conn.Close()

    s.messages <- ChatMessage{
        Timestamp: time.Now(),
        User:      "SERVER",
        Text:      fmt.Sprintf("%s left the chat", c.username),
    }
}
```

Properties:

*   Map removal prevents broadcaster from sending to closed channel.
*   `SafeClose()` prevents double cleanup.
*   "Left" event goes through broadcaster preserving order.

## F. **Safe Close Mechanism**

```go
func (c *Client) SafeClose() bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.closed {
        return false
    }
    c.closed = true
    return true
}
```

# 5. **Concurrency Safety (Critical Sections)**

1.  **server.clients map**
    *   All reads/writes must be under `s.mu`.

2.  **History access**
    *   Only broadcaster modifies history.
    *   History sent to clients under `s.mu` read lock.

3.  **Message broadcasting**
    *   Non‑blocking channel sends to avoid freezes.

4.  **One writer goroutine per client**
    *   No other goroutine calls `conn.Write()`.

5.  **Client cleanup**
    *   Map removal → `close(out)` → conn close → broadcast leave.
    *   `SafeClose()` prevents double-disconnect.

6.  **Accept loop**
    *   Only max‑connections check is performed under lock.

# 6. **Error Handling & Edge Cases**

- Username invalid  
- Username already taken  
- Server full  
- Client disconnects unexpectedly
- Slow client (auto‑disconnect via non‑blocking send)  
- Writer goroutine death  
- EOF in reader from client  
- CRLF normalization (`\r\n`, `\n`)  
- Unicode-safe handling (UTF‑8 assumed)
- safe double-disconnect handling

# 7. **Message Format**

All messages:

    [YYYY-MM-DD HH:MM:SS][client.name]:[client.message]

Event messages:

    [client.name] has joined our chat...
    [client.name] has left our chat...

# 8. **Testing Strategy**

## **Unit Tests**

*   Username validation
*   Message formatting
*   Map add/remove operations
*   History send order
*   SafeClose() behavior
*   Non‑blocking send logic

## **Integration Tests**

*   Multiple clients using `net.Pipe`
*   Join → history → messaging → leave sequence
*   Concurrent join + concurrent disconnect
*   Slow client triggering disconnection
*   Ordered broadcast verification

## **Stress Tests**

*   50–100 rapid connect/disconnect cycles
*   Random sleeps inserted to induce race conditions
*   Run with:
        go test -race -count=100 ./...

# 9. **Recommended Project Structure**

```
/cmd
    /server
        main.go
    /client
        main.go

/internal
    /chat
        server.go
        client.go
        message.go
        utils.go

/tests
    server_test.go
    client_test.go
    integration_test.go
```