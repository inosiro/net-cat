package chat

import (
	"fmt"
	"net"
	"testing"
)

// ── Task 2.1: Server Initialization ──────────────────────────────────────────

func TestNewServer(t *testing.T) {
	s := NewServer()

	if s == nil {
		t.Fatal("Expected NewServer() to return a non-nil *Server")
	}

	if s.clients == nil {
		t.Error("Expected clients map to be initialized")
	}

	if s.messages == nil {
		t.Error("Expected messages channel to be initialized")
	}

	// Verify the mutex is usable and the clients map is accessible under lock
	s.mu.Lock()
	count := len(s.clients)
	s.mu.Unlock()
	if count != 0 {
		t.Errorf("Expected 0 clients on init, got %d", count)
	}
}

// ── Task 2.2: Max Connections ─────────────────────────────────────────────────

func TestMaxConnections(t *testing.T) {
	s := NewServer()

	// Fill server to the max (10 clients)
	for i := 0; i < 10; i++ {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		c := &Client{
			conn:     serverConn,
			username: fmt.Sprintf("user%d", i),
			out:      make(chan string, 32),
		}
		s.mu.Lock()
		s.clients[c.username] = c
		s.mu.Unlock()
	}

	t.Run("10th client is accepted", func(t *testing.T) {
		s.mu.Lock()
		count := len(s.clients)
		s.mu.Unlock()
		if count != 10 {
			t.Errorf("Expected 10 clients, got %d", count)
		}
	})

	t.Run("11th client is rejected", func(t *testing.T) {
		if !s.IsFull() {
			t.Error("Expected server to be full after 10 clients")
		}
	})
}

// ── Task 2.3: Client Registration & History Sync ──────────────────────────────

func TestRegisterClient(t *testing.T) {
	s := NewServer()
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := &Client{
		conn:     serverConn,
		username: "Alice",
		out:      make(chan string, 32),
	}

	t.Run("Register new client", func(t *testing.T) {
		err := s.RegisterClient(c)
		if err != nil {
			t.Errorf("Expected no error registering Alice, got: %v", err)
		}
		s.mu.Lock()
		_, ok := s.clients["Alice"]
		s.mu.Unlock()
		if !ok {
			t.Error("Expected Alice to be in clients map")
		}
	})

	t.Run("Reject duplicate username", func(t *testing.T) {
		serverConn2, clientConn2 := net.Pipe()
		defer serverConn2.Close()
		defer clientConn2.Close()

		duplicate := &Client{
			conn:     serverConn2,
			username: "Alice",
			out:      make(chan string, 32),
		}
		err := s.RegisterClient(duplicate)
		if err == nil {
			t.Error("Expected an error when registering a duplicate username")
		}
	})
}

func TestSendHistory(t *testing.T) {
	s := NewServer()
	s.history = []ChatMessage{
		{User: "Bob", Text: "hello"},
		{User: "Charlie", Text: "world"},
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := &Client{
		conn:     serverConn,
		username: "Dave",
		out:      make(chan string, 32),
	}

	s.SendHistory(c)

	if len(c.out) != 2 {
		t.Errorf("Expected 2 history messages in client.out, got %d", len(c.out))
	}
}
