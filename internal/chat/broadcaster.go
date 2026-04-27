package chat

import (
	"fmt"
	"time"
)

// RoomBroadcaster is the central loop for a room that distributes messages to all clients
// in that room and appends them to the room's history. Must be run as a goroutine.
func (r *Room) RoomBroadcaster(s *Server) {
	for {
		select {
		// add Done to room to avoid goroutine leaks, stop broadcaster on server shutdown
		case <-r.Done:
			return
		case msg, ok := <-r.Messages:
			if !ok {
				return
			}
			formatted := msg.FormatMessage()

			r.Mu.Lock()
			clients := make([]*Client, 0, len(r.Clients))
			for _, c := range r.Clients {
				clients = append(clients, c)
			}
			r.History = append(r.History, msg)
			if len(r.History) > MaxRoomHistory {
				r.History = r.History[1:]
			}
			r.Mu.Unlock()

			for _, c := range clients {
				select {
				case c.Out <- formatted:
				default:
					// Slow / dead client — disconnect asynchronously so we don't block
					// go s.DisconnectClientFromRoom(r, c.Username)
					r.requestDisconnect(s, c)
				}
			}
			// case <-r.UserUpdates:
			// 	r.Mu.Lock()
			// 	var clients []string
			// 	for _, c := range r.Clients {
			// 		clients = append(clients, c.Username)
			// 	}
			// 	var msg string
			// 	if len(clients) > 0 {
			// 		msg = fmt.Sprintf("All users:\n  [%s]: %s", r.Name, strings.Join(clients, ", "))
			// 	} else {
			// 		msg = "All users:\n"
			// 	}
			// 	for _, c := range r.Clients {
			// 		select {
			// 		case c.Out <- msg:
			// 		default:
			// 		}
			// 	}
			// 	r.Mu.Unlock()
		}
	}
}

func (r *Room) requestDisconnect(s *Server, c *Client) {
	if c.SafeClose() {
		s.DisconnectClientFromRoom(r, c.Username)
	}
}

// AnnounceJoin sends a join system message through the broadcaster.
func (s *Server) AnnounceJoin(room *Room, username string) {
	room.Messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      SystemUser,
		Text:      fmt.Sprintf("%s has joined %s...", username, room.Name),
	}
}

// AnnounceLeave sends a leave system message through the broadcaster.
func (s *Server) AnnounceLeave(room *Room, username string) {
	room.Messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      SystemUser,
		Text:      fmt.Sprintf("%s has left %s...", username, room.Name),
	}
}
