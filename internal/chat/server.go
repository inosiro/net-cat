package chat

import (
	"errors"
	"sync"
)

const MaxClients = 10

type Server struct {
	clients  map[string]*Client
	mu       sync.Mutex
	messages chan ChatMessage
	history  []ChatMessage
}

// NewServer creates a new server.
func NewServer() *Server {
	return &Server{
		clients:  make(map[string]*Client),
		messages: make(chan ChatMessage, 128),
	}
}

// IsFull checks if the server is full.
func (s *Server) IsFull() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients) >= MaxClients
}

// RegisterClient registers a new client.
func (s *Server) RegisterClient(c *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.clients[c.username]; exists {
		return errors.New("username already taken: " + c.username)
	}
	s.clients[c.username] = c
	return nil
}

// SendHistory sends the chat history to a client.
func (s *Server) SendHistory(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, msg := range s.history {
		c.out <- msg.FormatMessage()
	}
}
