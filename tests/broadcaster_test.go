package chat_test

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"netcat/internal/chat"
)

// ── Task 3.1: Broadcaster – fan-out + history ─────────────────────────────────

func TestBroadcaster(t *testing.T) {
	s := chat.NewServer()

	room, err := s.CreateRoom("TestRoom")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	go room.RoomBroadcaster(s)

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
		room.Join(c)
		
		// consume join broadcasts ([USER_LIST_START], user0, ..., [USER_LIST_END])
		timeout := time.After(500 * time.Millisecond)
		for {
			select {
			case msg := <-c.Out:
				if strings.Contains(msg, "[USER_LIST_END]") {
					goto joined
				}
			case <-timeout:
				t.Fatalf("Timeout waiting for client join %d", i)
			}
		}
	joined:
	}

	// now that all clients have joined and channels are clear
	msg := chat.ChatMessage{
		Timestamp: time.Now(),
		User:      "user0",
		Text:      "hello",
	}
	room.Messages <- msg

	time.Sleep(20 * time.Millisecond)

	t.Run("Message fanned out to all clients", func(t *testing.T) {
		for _, c := range clients {
			if len(c.Out) == 0 {
				t.Errorf("Expected client %q to have received the message", c.Username)
			}
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
	
	room.Join(observer)
	
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case msg := <-observer.Out:
			t.Logf("Received message: %q", msg)
			if strings.Contains(msg, "[USER_LIST_END]") {
				// Flush the "observer has joined..." message that gets queued
				// after the user list broadcast.
				select {
				case m2 := <-observer.Out:
					t.Logf("Flushed message: %q", m2)
				case <-time.After(100 * time.Millisecond):
					t.Logf("No message to flush")
				}
				goto observerJoined
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for observer join")
		}
	}
observerJoined:

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

	go room.RoomBroadcaster(s)

	slowClient := &chat.Client{
		Conn:     serverSide,
		Username: "slow",
		Out:      make(chan string), 
	}
	room.Join(slowClient)

	done := make(chan struct{})
	go func() {
		// wait a bit so join can block or timeout the slow client
		time.Sleep(10 * time.Millisecond)
		room.Messages <- chat.ChatMessage{
			Timestamp: time.Now(),
			User:      "SERVER",
			Text:      "this should not block",
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Broadcaster deadlocked on slow client")
	}
}
