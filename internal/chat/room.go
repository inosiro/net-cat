package chat

import (
	"slices"
	"sync"
)

const MaxRooms = 256
const MaxRoomHistory = 64

type Room struct {
	Name     string
	Clients  map[string]*Client
	Messages chan ChatMessage
	History  []ChatMessage
	Mu       sync.Mutex
	Done     chan struct{}
}

// NewRoom creates a new room with the given name.
func NewRoom(name string) *Room {
	return &Room{
		Name:     name,
		Clients:  make(map[string]*Client),
		Messages: make(chan ChatMessage, 128),
		Done:     make(chan struct{}),
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
