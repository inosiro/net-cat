package chat

import (
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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
	if c.Out != nil {
		close(c.Out)
	}
	if c.Conn != nil {
		c.Conn.Close()
	}
	return true
}

type Server struct {
	Rooms       map[string]*Room
	Mu          sync.Mutex
	BannedIPs   map[string]time.Time
	Users       map[string]*Client // Global user index for O(1) lookups
	UserUpdates chan struct{}
	RoomUpdates chan struct{}
}

var ErrServerFull = errors.New("server is full: maximum clients reached")

// NewServer creates a new server with a default "Main Room".
func NewServer() *Server {
	s := &Server{
		Rooms:       make(map[string]*Room),
		BannedIPs:   make(map[string]time.Time),
		Users:       make(map[string]*Client),
		UserUpdates: make(chan struct{}, 10),
		RoomUpdates: make(chan struct{}, 10),
	}
	s.Rooms["Main Room"] = NewRoom("Main Room")
	go s.Rooms["Main Room"].RoomBroadcaster(s)
	go s.ServerBroadcaster()
	return s
}

// ServerBroadcaster listens for room updates and broadcasts the available rooms to all clients.
func (s *Server) ServerBroadcaster() {
	for range s.RoomUpdates {
		rooms := s.ListRooms()
		var lines []string
		lines = append(lines, "Available rooms:")
		for _, roomName := range rooms {
			room, _ := s.GetRoom(roomName)
			lines = append(lines, fmt.Sprintf("  %s (%d users)", roomName, room.ClientCount()))
		}
		msg := strings.Join(lines, "\n")

		s.Mu.Lock()
		roomObjects := make([]*Room, 0, len(s.Rooms))
		for _, r := range s.Rooms {
			roomObjects = append(roomObjects, r)
		}
		s.Mu.Unlock()

		for _, room := range roomObjects {
			room.Mu.Lock()
			for _, c := range room.Clients {
				select {
				case c.Out <- msg:
				default:
				}
			}
			room.Mu.Unlock()
		}
	}
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
	log.Printf("Room %s created\n", name)

	select {
	case s.RoomUpdates <- struct{}{}:
	default:
	}

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
	s.Mu.Lock()
	defer s.Mu.Unlock()

	totalClients := 0
	for _, currentRoom := range s.Rooms {
		currentRoom.Mu.Lock()
		totalClients += len(currentRoom.Clients)
		currentRoom.Mu.Unlock()
	}
	if totalClients >= MaxClients {
		return ErrServerFull
	}

	c.Username = strings.TrimRight(c.Username, "\r\n")
	if !utf8.ValidString(c.Username) {
		return errors.New("No valid utf8 username\n")
	}
	if strings.HasPrefix(c.Username, "/") {
		return errors.New("Username cannot start with /\n")
	}
	if _, exists := s.Users[c.Username]; exists {
		return errors.New("username already taken: " + c.Username)
	}

	room.Mu.Lock()
	if _, exists := room.Clients[c.Username]; exists {
		room.Mu.Unlock()
		return errors.New("username already taken in this room: " + c.Username)
	}
	room.Clients[c.Username] = c
	s.Users[c.Username] = c
	room.Mu.Unlock()
	log.Printf("Client %s joined room %s\n", c.Username, room.Name)

	select {
	case s.RoomUpdates <- struct{}{}:
	default:
	}
	select {
	case s.UserUpdates <- struct{}{}:
	default:
	}

	return nil
}

// MoveClientToRoom transfers a client from one room to another without closing the connection.
func (s *Server) MoveClientToRoom(c *Client, oldRoom, newRoom *Room) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if oldRoom == newRoom {
		return errors.New("already in this room")
	}

	newRoom.Mu.Lock()
	if _, exists := newRoom.Clients[c.Username]; exists {
		newRoom.Mu.Unlock()
		return errors.New("username already taken in target room")
	}

	oldRoom.Mu.Lock()
	delete(oldRoom.Clients, c.Username)
	oldRoom.Mu.Unlock()

	newRoom.Clients[c.Username] = c
	newRoom.Mu.Unlock()

	select {
	case s.RoomUpdates <- struct{}{}:
	default:
	}
	// Trigger user list updates for both rooms
	select {
	case s.UserUpdates <- struct{}{}:
	default:
	}

	return nil
}

// DisconnectClientFromRoom removes a client from a room.
func (s *Server) DisconnectClientFromRoom(room *Room, username string) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	room.Mu.Lock()
	client, exists := room.Clients[username]
	if !exists {
		room.Mu.Unlock()
		return false
	}

	log.Printf("Client %s left room %s\n", username, room.Name)
	delete(room.Clients, username)
	delete(s.Users, username)
	room.Mu.Unlock()

	// Close the client's resources after removing from maps
	client.SafeClose()

	select {
	case s.RoomUpdates <- struct{}{}:
	default:
	}
	select {
	case s.UserUpdates <- struct{}{}:
	default:
	}

	return true
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
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		ip = addr // fallback if no port present
	}
	log.Print("Ban enforced for IP: ", addr)
	s.BannedIPs[ip] = time.Now().Add(1 * time.Minute)
}

func (s *Server) IsIPBanned(addr string) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		ip = addr
	}
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

// Shutdown gracefully stops all room broadcasters.
func (s *Server) Shutdown() {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	for _, room := range s.Rooms {
		select {
		case <-room.Done: // Already closed
		default:
			close(room.Done)
		}
	}
}
