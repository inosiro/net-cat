package chat

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const MaxClients = 10

type Server struct {
	Clients   map[string]*Client
	Mu        sync.Mutex
	Messages  chan ChatMessage
	History   []ChatMessage
	BannedIPs map[string]time.Time
}

// NewServer creates a new server.
func NewServer() *Server {
	return &Server{
		Clients:   make(map[string]*Client),
		Messages:  make(chan ChatMessage, 128),
		BannedIPs: make(map[string]time.Time),
	}
}

// IsFull checks if the server is full.
func (s *Server) IsFull() bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return len(s.Clients) >= MaxClients
}

// RegisterClient registers a new client.
func (s *Server) RegisterClient(c *Client) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if _, exists := s.Clients[c.Username]; exists {
		return errors.New("username already taken: " + c.Username)
	}
	s.Clients[c.Username] = c
	return nil
}

// SendHistory sends the chat history to a client.
func (s *Server) SendHistory(c *Client) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for _, msg := range s.History {
		c.Out <- msg.FormatMessage()
	}
}

func (s *Server) BanIP(addr string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	ip := strings.Split(addr, ":")[0]
	s.BannedIPs[ip] = time.Now().Add(1 * time.Minute)
}

func (s *Server) IsIPBanned(addr string) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	ip := strings.Split(addr, ":")[0]
	banUntil, exists := s.BannedIPs[ip]
	if !exists {
		return false
	}
	if time.Now().After(banUntil) {
		delete(s.BannedIPs, ip)
		return false
	}
	return true
}
