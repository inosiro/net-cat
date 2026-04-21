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

	if s.Rooms == nil {
		t.Error("Expected rooms map to be initialized")
	}

	// Verify server starts with Main Room
	mainRoom, err := s.GetRoom("Main Room")
	if err != nil {
		t.Errorf("Expected Main Room to be created, got error: %v", err)
	}
	if mainRoom == nil {
		t.Error("Expected Main Room to be non-nil")
	}
}

// ── Task 2.2: Max Connections Enforcement ────────────────────────────────────

func TestRoomCapacity(t *testing.T) {
	s := chat.NewServer()
	room, _ := s.GetRoom("Main Room")
	connections := make([]net.Conn, 0, chat.MaxClients*2+2)
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()

	t.Run("Tenth client succeeds", func(t *testing.T) {
		for i := 0; i < chat.MaxClients; i++ {
			serverConn, clientConn := net.Pipe()
			connections = append(connections, serverConn, clientConn)

			c := &chat.Client{
				Conn:     serverConn,
				Username: fmt.Sprintf("user%d", i),
				Out:      make(chan string, 32),
			}

			if err := s.RegisterClientInRoom(room, c); err != nil {
				t.Fatalf("Expected client %d to be admitted, got error: %v", i+1, err)
			}
		}

		room.Mu.Lock()
		count := len(room.Clients)
		room.Mu.Unlock()
		if count != chat.MaxClients {
			t.Fatalf("Expected %d clients in room, got %d", chat.MaxClients, count)
		}
	})

	t.Run("Eleventh client is rejected", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		connections = append(connections, serverConn, clientConn)

		c := &chat.Client{
			Conn:     serverConn,
			Username: "user-overflow",
			Out:      make(chan string, 32),
		}

		err := s.RegisterClientInRoom(room, c)
		if err == nil {
			t.Fatal("Expected 11th client to be rejected")
		}
		if err != chat.ErrServerFull {
			t.Fatalf("Expected ErrServerFull, got %v", err)
		}
	})

	t.Run("Room exists", func(t *testing.T) {
		if room == nil {
			t.Error("Expected room to exist")
		}
	})
}

// ── Task 2.3: Client Registration & History Sync ──────────────────────────────

func TestRegisterClient(t *testing.T) {
	s := chat.NewServer()
	room, _ := s.GetRoom("Main Room")
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := &chat.Client{
		Conn:     serverConn,
		Username: "Alice",
		Out:      make(chan string, 32),
	}

	t.Run("Register new client", func(t *testing.T) {
		err := s.RegisterClientInRoom(room, c)
		if err != nil {
			t.Errorf("Expected no error registering Alice, got: %v", err)
		}
		room.Mu.Lock()
		_, ok := room.Clients["Alice"]
		room.Mu.Unlock()
		if !ok {
			t.Error("Expected Alice to be in room clients map")
		}
	})

	t.Run("Duplicate username rejected", func(t *testing.T) {
		c2 := &chat.Client{
			Conn:     clientConn,
			Username: "Alice",
			Out:      make(chan string, 32),
		}
		err := s.RegisterClientInRoom(room, c2)
		if err == nil {
			t.Error("Expected error when registering duplicate username")
		}
	})
}

func TestSendHistory(t *testing.T) {
	s := chat.NewServer()
	room, _ := s.GetRoom("Main Room")
	room.History = []chat.ChatMessage{
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

	room.SendHistory(c)

	if len(c.Out) != 2 {
		t.Errorf("Expected 2 history messages in client.out, got %d", len(c.Out))
	}
}
