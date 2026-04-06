package chat

import (
	"fmt"
	"time"
)

// Broadcaster is the central loop that distributes messages to all clients
// and appends them to history. Must be run as a goroutine.
func (s *Server) Broadcaster() {
	for msg := range s.messages {
		formatted := msg.FormatMessage()

		s.mu.Lock()
		for _, c := range s.clients {
			select {
			case c.out <- formatted:
			default:
				// Slow / dead client — disconnect asynchronously so we don't block
				go s.DisconnectClient(c)
			}
		}
		s.history = append(s.history, msg)
		s.mu.Unlock()
	}
}

// AnnounceJoin sends a join system message through the broadcaster.
func (s *Server) AnnounceJoin(username string) {
	s.messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      "SERVER",
		Text:      fmt.Sprintf("%s has joined our chat...", username),
	}
}

// AnnounceLeave sends a leave system message through the broadcaster.
func (s *Server) AnnounceLeave(username string) {
	s.messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      "SERVER",
		Text:      fmt.Sprintf("%s has left our chat...", username),
	}
}

// DisconnectClient safely removes a client from the server, closes their
// output channel, and closes the TCP connection.
func (s *Server) DisconnectClient(c *Client) {
	if !c.SafeClose() {
		return // already being disconnected
	}

	s.mu.Lock()
	delete(s.clients, c.username)
	close(c.out)
	s.mu.Unlock()

	c.conn.Close()

	s.AnnounceLeave(c.username)
}
