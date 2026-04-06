package chat

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// ── Task 3.1: Broadcaster – fan-out + history ─────────────────────────────────

func TestBroadcaster(t *testing.T) {
	s := NewServer()

	// Create two connected clients
	clients := make([]*Client, 2)
	for i := 0; i < 2; i++ {
		serverSide, clientSide := net.Pipe()
		defer serverSide.Close()
		defer clientSide.Close()
		c := &Client{
			conn:     serverSide,
			username: fmt.Sprintf("user%d", i),
			out:      make(chan string, 32),
		}
		clients[i] = c
		s.mu.Lock()
		s.clients[c.username] = c
		s.mu.Unlock()
	}

	// Start the broadcaster in the background
	go s.Broadcaster()

	msg := ChatMessage{
		Timestamp: time.Now(),
		User:      "user0",
		Text:      "hello",
	}
	s.messages <- msg

	// Give the broadcaster a moment to process
	time.Sleep(20 * time.Millisecond)

	t.Run("Message fanned out to all clients", func(t *testing.T) {
		for _, c := range clients {
			if len(c.out) == 0 {
				t.Errorf("Expected client %q to have received the message", c.username)
			}
		}
	})

	t.Run("Message appended to history", func(t *testing.T) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(s.history) != 1 {
			t.Errorf("Expected 1 history entry, got %d", len(s.history))
		}
	})
}

// ── Task 3.2: System Announcements (join/leave) ───────────────────────────────

func TestSystemAnnouncements(t *testing.T) {
	s := NewServer()
	go s.Broadcaster()

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	observer := &Client{
		conn:     serverSide,
		username: "observer",
		out:      make(chan string, 32),
	}
	s.mu.Lock()
	s.clients["observer"] = observer
	s.mu.Unlock()

	t.Run("Join announcement", func(t *testing.T) {
		s.AnnounceJoin("Alice")
		time.Sleep(20 * time.Millisecond)
		if len(observer.out) == 0 {
			t.Error("Expected observer to receive join announcement")
		}
		got := <-observer.out
		expected := "Alice has joined our chat..."
		if got != expected {
			t.Errorf("Expected %q, got %q", expected, got)
		}
	})

	t.Run("Leave announcement", func(t *testing.T) {
		s.AnnounceLeave("Bob")
		time.Sleep(20 * time.Millisecond)
		if len(observer.out) == 0 {
			t.Error("Expected observer to receive leave announcement")
		}
		got := <-observer.out
		expected := "Bob has left our chat..."
		if got != expected {
			t.Errorf("Expected %q, got %q", expected, got)
		}
	})
}

// ── Task 3.3: Non-blocking Fan-out / Slow Clients ────────────────────────────

func TestNonBlockingFanout(t *testing.T) {
	s := NewServer()

	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	// Create a slow client with a FULL, zero-buffered channel to simulate blocking
	slowClient := &Client{
		conn:     serverSide,
		username: "slow",
		out:      make(chan string), // unbuffered = will always block
	}
	s.mu.Lock()
	s.clients["slow"] = slowClient
	s.mu.Unlock()

	go s.Broadcaster()

	// Send a message — broadcaster must NOT deadlock despite slow client
	done := make(chan struct{})
	go func() {
		s.messages <- ChatMessage{
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
