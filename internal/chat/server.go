package chat

import (
	"errors"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

const MaxClients = 10

type Client struct {
	Conn          net.Conn
	Username      string
	Out           chan string
	currentRoom   *Room
	closed        bool
	mu            sync.Mutex
	lastMessageAt time.Time
	spamCount     int
}

func (c *Client) SafeClose() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	c.closed = true
	return true
}

type Server struct {
	Rooms     map[string]*Room
	Mu        sync.Mutex
	BannedIPs map[string]time.Time
}

// NewServer creates a new server with a default "Main Room".
func NewServer() *Server {
	s := &Server{
		Rooms:     make(map[string]*Room),
		BannedIPs: make(map[string]time.Time),
	}
	s.Rooms["Main Room"] = NewRoom("Main Room")
	return s
}

// CreateRoom creates a new room if it doesn't exist.
func (s *Server) CreateRoom(name string) (*Room, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if len(s.Rooms) >= MaxRooms {
		return nil, errors.New("server is full: maximum rooms reached")
	}
	if _, exists := s.Rooms[name]; exists {
		return nil, errors.New("room already exists: " + name)
	}
	room := NewRoom(name)
	s.Rooms[name] = room
	return room, nil
}

// GetRoom retrieves a room by name.
func (s *Server) GetRoom(name string) (*Room, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if room, exists := s.Rooms[name]; exists {
		return room, nil
	}
	return nil, errors.New("room not found: " + name)
}

// ListRooms returns a list of room names.
func (s *Server) ListRooms() []string {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	rooms := make([]string, 0, len(s.Rooms))
	for name := range s.Rooms {
		rooms = append(rooms, name)
	}
	slices.Sort(rooms)
	return rooms
}

// RegisterClientInRoom adds a client to a room.
func (s *Server) RegisterClientInRoom(room *Room, c *Client) error {
	room.Mu.Lock()
	defer room.Mu.Unlock()
	if _, exists := room.Clients[c.Username]; exists {
		return errors.New("username already taken in this room: " + c.Username)
	}
	room.Clients[c.Username] = c
	return nil
}

// DisconnectClientFromRoom removes a client from a room.
func (s *Server) DisconnectClientFromRoom(room *Room, username string) {
	room.Mu.Lock()
	defer room.Mu.Unlock()
	delete(room.Clients, username)
}

// GetTotalUserCount returns total users across all rooms
func (s *Server) GetTotalUserCount() int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	total := 0
	for _, room := range s.Rooms {
		room.Mu.Lock()
		total += len(room.Clients)
		room.Mu.Unlock()
	}
	return total
}

// GetRoomCount returns the number of rooms
func (s *Server) GetRoomCount() int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return len(s.Rooms)
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
