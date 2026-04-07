package chat

import (
	"fmt"
	"time"
)

// RoomBroadcaster is the central loop for a room that distributes messages to all clients
// in that room and appends them to the room's history. Must be run as a goroutine.
func (r *Room) RoomBroadcaster() {
	for msg := range r.Messages {
		formatted := msg.FormatMessage()

		r.Mu.Lock()
		for _, c := range r.Clients {
			select {
			case c.Out <- formatted:
			default:
				// Slow / dead client — disconnect asynchronously so we don't block
				go r.DisconnectClientFromRoom(c.Username)
			}
		}
		r.History = append(r.History, msg)
		r.Mu.Unlock()
	}
}

// AnnounceJoin sends a join system message through the broadcaster.
func (s *Server) AnnounceJoin(room *Room, username string) {
	room.Messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      "SERVER",
		Text:      fmt.Sprintf("%s has joined %s...", username, room.Name),
	}
}

// AnnounceLeave sends a leave system message through the broadcaster.
func (s *Server) AnnounceLeave(room *Room, username string) {
	room.Messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      "SERVER",
		Text:      fmt.Sprintf("%s has left %s...", username, room.Name),
	}
}
