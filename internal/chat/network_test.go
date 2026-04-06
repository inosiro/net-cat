package chat

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── Task 4.1: Client Writer ───────────────────────────────────────────────────

func TestClientWriter(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer clientSide.Close()

	c := &Client{
		conn:     serverSide,
		username: "testwriter",
		out:      make(chan string, 10),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Writer()
		serverSide.Close()
	}()

	c.out <- "Hello client!"
	c.out <- "Second message"
	close(c.out)

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

	wg.Wait()

	if scanner.Scan() {
		t.Fatal("Expected EOF after channel closure")
	}
}

// ── Task 4.2: Client Reader ───────────────────────────────────────────────────

func TestClientReader(t *testing.T) {
	s := NewServer()

	serverSide, clientSide := net.Pipe()

	c := &Client{
		conn:     serverSide,
		username: "testreader",
		out:      make(chan string, 10),
	}
	s.mu.Lock()
	s.clients[c.username] = c
	s.mu.Unlock()

	go c.Reader(s)

	go func() {
		clientSide.Write([]byte("First line\n"))
		clientSide.Write([]byte("\n")) // Empty line to be discarded
		clientSide.Write([]byte("Second line\n"))
		clientSide.Close() // Triggers EOF
	}()

	var received []ChatMessage
	timeout := time.After(500 * time.Millisecond)

loop:
	for {
		select {
		case msg := <-s.messages:
			received = append(received, msg)
			if len(received) == 2 {
				break loop
			}
		case <-timeout:
			t.Fatal("Timeout waiting for messages")
		}
	}

	if received[0].Text != "First line" || received[1].Text != "Second line" {
		t.Errorf("Unexpected messages received: %+v", received)
	}

	// Verify client got disconnected on EOF
	time.Sleep(100 * time.Millisecond)
	s.mu.Lock()
	_, exists := s.clients["testreader"]
	s.mu.Unlock()

	if exists {
		t.Error("Expected testreader to be disconnected after EOF")
	}
}

// ── Task 4.3: TCP Listener ───────────────────────────────────────────────────

func TestListenAndServe(t *testing.T) {
	s := NewServer()
	
	// Start serving on a free port
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	port := strings.Split(ln.Addr().String(), ":")[1]

	go s.AcceptLoop(ln) // We test AcceptLoop directly rather than ListenAndServe to pass the dynamic port

	// Connect to it
	conn, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Initially we expect the linux logo and asking for name
	// This ensures the accept loop handles incoming connections
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from connection: %v", err)
	}
	
	text := string(buf[:n])
	if !strings.Contains(text, "[ENTER YOUR NAME]:") {
		t.Errorf("Expected name prompt, got %q", text)
	}
}
