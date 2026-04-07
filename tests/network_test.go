package chat_test

import (
"bufio"
"net"
"testing"

"netcat/internal/chat"
)

// ── Task 4.1: Client Writer ───────────────────────────────────────────────────

func TestClientWriter(t *testing.T) {
serverSide, clientSide := net.Pipe()
defer clientSide.Close()

c := &chat.Client{
Conn:     serverSide,
Username: "testwriter",
Out:      make(chan string, 10),
}

go func() {
c.Writer()
serverSide.Close()
}()

c.Out <- "Hello client!"
c.Out <- "Second message"
close(c.Out)

scanner := bufio.NewScanner(clientSide)

if !scanner.Scan() {
t.Fatal("Expected first message, got EOF or error")
}
if scanner.Text() != "Hello client!" {
t.Errorf("Expected 'Hello client!', got %q", scanner.Text())
}

if !scanner.Scan() {
t.Fatal("Expected second message, got EOF or error")
}
if scanner.Text() != "Second message" {
t.Errorf("Expected 'Second message', got %q", scanner.Text())
}

if scanner.Scan() {
t.Errorf("Expected EOF, but got: %s", scanner.Text())
}
}
