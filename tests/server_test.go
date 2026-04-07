package chat_test

import (
	"fmt"
	"net"
	"testing"

	"netcat/internal/chat"
)

// ── Task 2.1: Server Initialization ──────────────────────────────────────────

func TestNewServer(t *testing.T) {
	s := chat.NewServer()

	if s == nil {
		t.Fatal("Expected NewServer() to return a non-nil *Server")
	}

	if s.Clients == nil {
		t.Error("Expected clients map to be initialized")
	}

	if s.Messages == nil {
		t.Error("Expected messages channel to be initialized")
	}

	// Verify the mutex is usable and the clients map is accessible under lock
	s.Mu.Lock()
	count := len(s.Clients)
	s.Mu.Unlock()
	if count != 0 {
		t.Errorf("Expected 0 clients on init, got %d", count)
	}
}

// ── Task 2.2: Max Connections ─────────────────────────────────────────────────

func TestMaxConnections(t *testing.T) {
	s := chat.NewServer()

	// Fill server to the max (10 clients)
	for i := 0; i < 10; i++ {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		c := &chat.Client{
			Conn:     serverConn,
			Username: fmt.Sprintf("user%d", i),
			Out:      make(chan string, 32),
		}
		s.Mu.Lock()
		s.Clients[c.Username] = c
		s.Mu.Unlock()
	}

	t.Run("10th client is accepted", func(t *testing.T) {
		s.Mu.Lock()
		count := len(s.Clients)
		s.Mu.Unlock()
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
	s := chat.NewServer()
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := &chat.Client{
		Conn:     serverConn,
		Username: "Alice",
		Out:      make(chan string, 32),
	}

	t.Run("Register new client", func(t *testing.T) {
		err := s.RegisterClient(c)
		if err != nil {
			t.Errorf("Expected no error registering Alice, got: %v", err)
		}
		s.Mu.Lock()
		_, ok := s.Clients["Alice"]
		s.Mu.Unlock()
		if !ok {
			t.Error("Expected Alice to be in clients map")
		}
	})

	t.Run("Reject duplicate username", func(t *testing.T) {
		serverConn2, clientConn2 := net.Pipe()
		defer serverConn2.Close()
		defer clientConn2.Close()

		duplicate := &chat.Client{
			Conn:     serverConn2,
			Username: "Alice",
			Out:      make(chan string, 32),
		}
		err := s.RegisterClient(duplicate)
		if err == nil {
			t.Error("Expected an error when registering a duplicate username")
		}
	})
}

func TestSendHistory(t *testing.T) {
	s := chat.NewServer()
	s.History = []chat.ChatMessage{
		{User: "Bob", Text: "hello"},
		{User: "Charlie", Text: "world"},
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := &chat.Client{
		Conn:     serverConn,
		Username: "Dave",
		Out:      make(chan string, 32),
	}

	s.SendHistory(c)

	if len(c.Out) != 2 {
		t.Errorf("Expected 2 history messages in client.out, got %d", len(c.Out))
	}
}
