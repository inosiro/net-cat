package chat

import (
	"fmt"
	"time"
)

// Broadcaster is the central loop that distributes messages to all clients
// and appends them to history. Must be run as a goroutine.
func (s *Server) Broadcaster() {
	for msg := range s.Messages {
		formatted := msg.FormatMessage()

		s.Mu.Lock()
		for _, c := range s.Clients {
			select {
			case c.Out <- formatted:
			default:
				// Slow / dead client — disconnect asynchronously so we don't block
				go s.DisconnectClient(c)
			}
		}
		s.History = append(s.History, msg)
		s.Mu.Unlock()
	}
}

// AnnounceJoin sends a join system message through the broadcaster.
func (s *Server) AnnounceJoin(username string) {
	s.Messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      "SERVER",
		Text:      fmt.Sprintf("%s has joined our chat...", username),
	}
}

// AnnounceLeave sends a leave system message through the broadcaster.
func (s *Server) AnnounceLeave(username string) {
	s.Messages <- ChatMessage{
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

	s.Mu.Lock()
	delete(s.Clients, c.Username)
	close(c.Out)
	s.Mu.Unlock()

	c.Conn.Close()

	s.AnnounceLeave(c.Username)
}
