package chat_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"netcat/internal/chat"
)

// ── Task 3.1: Broadcaster – fan-out + history ─────────────────────────────────

func TestBroadcaster(t *testing.T) {
	s := chat.NewServer()

	// Create a room
	room, err := s.CreateRoom("TestRoom")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	// Create two connected clients
	clients := make([]*chat.Client, 2)
	for i := 0; i < 2; i++ {
		serverSide, clientSide := net.Pipe()
		defer serverSide.Close()
		defer clientSide.Close()
		c := &chat.Client{
			Conn:     serverSide,
			Username: fmt.Sprintf("user%d", i),
			Out:      make(chan string, 32),
		}
		clients[i] = c
		room.Mu.Lock()
		room.Clients[c.Username] = c
		room.Mu.Unlock()
	}

	// Start the broadcaster in the background
	go room.RoomBroadcaster(s)

	msg := chat.ChatMessage{
		Timestamp: time.Now(),
		User:      "user0",
		Text:      "hello",
	}
	room.Messages <- msg

	// Give the broadcaster a moment to process
	time.Sleep(20 * time.Millisecond)

	t.Run("Message fanned out to all clients", func(t *testing.T) {
		for _, c := range clients {
			if len(c.Out) == 0 {
				t.Errorf("Expected client %q to have received the message", c.Username)
			}
		}
	})

	t.Run("Message appended to history", func(t *testing.T) {
		room.Mu.Lock()
		defer room.Mu.Unlock()
		if len(room.History) != 1 {
			t.Errorf("Expected 1 history entry, got %d", len(room.History))
		}
	})
}

// ── Task 3.2: System Announcements (join/leave) ───────────────────────────────

func TestSystemAnnouncements(t *testing.T) {
	s := chat.NewServer()
	room, err := s.CreateRoom("TestRoom")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	go room.RoomBroadcaster(s)

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	observer := &chat.Client{
		Conn:     serverSide,
		Username: "observer",
		Out:      make(chan string, 32),
	}
	room.Mu.Lock()
	room.Clients["observer"] = observer
	room.Mu.Unlock()

	t.Run("Join announcement", func(t *testing.T) {
		s.AnnounceJoin(room, "Alice")
		select {
		case got := <-observer.Out:
			if got == "" {
				t.Error("Expected join announcement")
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Expected observer to receive join announcement")
		}
	})

	t.Run("Leave announcement", func(t *testing.T) {
		s.AnnounceLeave(room, "Bob")
		select {
		case got := <-observer.Out:
			if got == "" {
				t.Error("Expected leave announcement")
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Expected observer to receive leave announcement")
		}
	})
}

// ── Task 3.3: Non-blocking Fan-out / Slow Clients ────────────────────────────

func TestNonBlockingFanout(t *testing.T) {
	s := chat.NewServer()
	room, err := s.CreateRoom("TestRoom")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	// Create a slow client with a FULL, zero-buffered channel to simulate blocking
	slowClient := &chat.Client{
		Conn:     serverSide,
		Username: "slow",
		Out:      make(chan string), // unbuffered = will always block
	}
	room.Mu.Lock()
	room.Clients["slow"] = slowClient
	room.Mu.Unlock()

	go room.RoomBroadcaster(s)

	// Send a message — broadcaster must NOT deadlock despite slow client
	done := make(chan struct{})
	go func() {
		room.Messages <- chat.ChatMessage{
			Timestamp: time.Now(),
			User:      "SERVER",
			Text:      "this should not block",
		}
		close(done)
	}()

	select {
	case <-done:
		// success — broadcaster didn't hang
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Broadcaster deadlocked on slow client")
	}
}
