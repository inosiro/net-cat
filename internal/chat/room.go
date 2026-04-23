package chat

import (
	"log"
	"slices"
	"sync"
	"time"
)

const MaxRooms = 256

type Room struct {
	Name        string
	Clients     map[string]*Client
	Messages    chan ChatMessage
	History     []ChatMessage
	Mu          sync.Mutex
	UserUpdates chan struct{}
}

// NewRoom creates a new room with the given name.
func NewRoom(name string) *Room {
	return &Room{
		Name:        name,
		Clients:     make(map[string]*Client),
		Messages:    make(chan ChatMessage, 128),
		UserUpdates: make(chan struct{}, 10),
	}
}

// AddClient adds a client to the room.
func (r *Room) AddClient(c *Client) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.Clients[c.Username] = c
}

// RemoveClient removes a client from the room.
func (r *Room) RemoveClient(username string) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	delete(r.Clients, username)
}

// ClientCount returns the number of clients in the room.
func (r *Room) ClientCount() int {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return len(r.Clients)
}

// GetClients returns a list of client usernames in the room.
func (r *Room) GetClients() []string {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	clients := make([]string, 0, len(r.Clients))
	for username := range r.Clients {
		clients = append(clients, username)
	}
	slices.Sort(clients)
	return clients
}

// SendHistory sends the room's history to a client.
func (r *Room) SendHistory(c *Client) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	for _, msg := range r.History {
		c.Out <- msg.FormatMessage()
	}
}

// AnnounceJoin sends a join message to the room.
func (r *Room) AnnounceJoin(username string) {
	r.Messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      "SERVER",
		Text:      username + " has joined " + r.Name,
	}
}

// AnnounceLeave sends a leave message to the room.
func (r *Room) AnnounceLeave(username string) {
	r.Messages <- ChatMessage{
		Timestamp: time.Now(),
		User:      "SERVER",
		Text:      username + " has left " + r.Name,
	}
}

// DisconnectClientFromRoom safely removes a client from the room.
func (r *Room) DisconnectClientFromRoom(username string) bool {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	client, exists := r.Clients[username]
	if !exists {
		return false
	}
	if !client.SafeClose() {
		return false
	}
	log.Printf("%s left room %s\n", username, r.Name)
	delete(r.Clients, username)
	close(client.Out)
	client.Conn.Close()
	return true
}
