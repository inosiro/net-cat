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
		s.Mu.Lock()
		s.Clients[c.Username] = c
		s.Mu.Unlock()
	}

	// Start the broadcaster in the background
	go s.Broadcaster()

	msg := chat.ChatMessage{
		Timestamp: time.Now(),
		User:      "user0",
		Text:      "hello",
	}
	s.Messages <- msg

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
		s.Mu.Lock()
		defer s.Mu.Unlock()
		if len(s.History) != 1 {
			t.Errorf("Expected 1 history entry, got %d", len(s.History))
		}
	})
}

// ── Task 3.2: System Announcements (join/leave) ───────────────────────────────

func TestSystemAnnouncements(t *testing.T) {
	s := chat.NewServer()
	go s.Broadcaster()

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	observer := &chat.Client{
		Conn:     serverSide,
		Username: "observer",
		Out:      make(chan string, 32),
	}
	s.Mu.Lock()
	s.Clients["observer"] = observer
	s.Mu.Unlock()

	t.Run("Join announcement", func(t *testing.T) {
		s.AnnounceJoin("Alice")
		select {
		case got := <-observer.Out:
			expected := "Alice has joined our chat..."
			if got != expected {
				t.Errorf("Expected %q, got %q", expected, got)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Expected observer to receive leave announcement")
		}
	})

	t.Run("Leave announcement", func(t *testing.T) {
		s.AnnounceLeave("Bob")
		select {
		case got := <-observer.Out:
			expected := "Bob has left our chat..."
			if got != expected {
				t.Errorf("Expected %q, got %q", expected, got)
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Expected observer to receive leave announcement")
		}
	})
}

// ── Task 3.3: Non-blocking Fan-out / Slow Clients ────────────────────────────

func TestNonBlockingFanout(t *testing.T) {
	s := chat.NewServer()

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	// Create a slow client with a FULL, zero-buffered channel to simulate blocking
	slowClient := &chat.Client{
		Conn:     serverSide,
		Username: "slow",
		Out:      make(chan string), // unbuffered = will always block
	}
	s.Mu.Lock()
	s.Clients["slow"] = slowClient
	s.Mu.Unlock()

	go s.Broadcaster()

	// Send a message — broadcaster must NOT deadlock despite slow client
	done := make(chan struct{})
	go func() {
		s.Messages <- chat.ChatMessage{
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
