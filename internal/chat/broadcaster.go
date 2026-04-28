package chat

import (
	"fmt"
	"strings"
	"time"
	"slices"
)

func (r *Room) RoomBroadcaster(s *Server) {
	for {
		select {
		case msg, ok := <-r.Messages:
			if !ok {
				r.shutdown()
				return
			}
			r.handleMessage(s, msg)

		case ev := <-r.events:
			switch ev.typ {
			case roomJoin:
				r.handleJoin(s, ev.client)

			case roomLeave:
				r.handleLeave(s, ev.client)

			case roomDisconnect:
				r.handleDisconnect(s, ev.client)

			case roomShutdown:
				r.shutdown()
				return

			case roomNickChange:
				r.handleNickChange(s, ev.client, ev.oldName, ev.newName)

			case roomClientCount:
				ev.respChan <- len(r.clients)

			case roomGetClients:
				clients := make([]string, 0, len(r.clients))
				for username := range r.clients {
					clients = append(clients, username)
				}
				slices.Sort(clients)
				ev.respChan <- clients
			}

		case <-r.done:
			return
		}
	}
}

func (r *Room) handleMessage(s *Server, msg ChatMessage) {
	formatted := msg.FormatMessage()

	r.history = append(r.history, msg)
	if len(r.history) > MaxRoomHistory {
		r.history = r.history[1:]
	}

	for _, c := range r.clients {
		select {
		case c.Out <- formatted:
		default:
			r.events <- roomEvent{
				typ:    roomDisconnect,
				client: c,
			}
		}
	}
}

func (r *Room) handleJoin(s *Server, c *Client) {
	r.clients[c.Username] = c

	for _, msg := range r.history {
		select {
		case c.Out <- msg.FormatMessage():
		default:
			r.events <- roomEvent{typ: roomDisconnect, client: c}
			return
		}
	}

	s.AnnounceJoin(r, c.Username)
	r.broadcastUsersList()
}

func (r *Room) handleLeave(s *Server, c *Client) {
	if _, ok := r.clients[c.Username]; !ok {
		return
	}

	delete(r.clients, c.Username)
	s.AnnounceLeave(r, c.Username)
	r.broadcastUsersList()
}

func (r *Room) handleDisconnect(s *Server, c *Client) {
	if _, ok := r.clients[c.Username]; !ok {
		return
	}

	delete(r.clients, c.Username)
	c.SafeClose()
	s.FinalizeClientDisconnect(c)
	s.AnnounceLeave(r, c.Username)
	r.broadcastUsersList()
}

func (r *Room) handleNickChange(s *Server, c *Client, oldName, newName string) {
	if _, ok := r.clients[oldName]; ok {
		delete(r.clients, oldName)
		r.clients[newName] = c
	}
	r.broadcastUsersList()
}

func (r *Room) shutdown() {
	for _, c := range r.clients {
		c.SafeClose()
	}

	r.clients = nil
	close(r.done)
}

func (r *Room) broadcastUsersList() {
	var clients []string
	for _, c := range r.clients {
		clients = append(clients, c.Username)
	}
	slices.Sort(clients)

	var lines []string
	lines = append(lines, "[USER_LIST_START]")
	lines = append(lines, clients...)
	lines = append(lines, "[USER_LIST_END]")
	msg := strings.Join(lines, "\n")

	for _, c := range r.clients {
		select {
		case c.Out <- msg:
		default:
		}
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
