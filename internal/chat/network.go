package chat

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	logo            = "Welcome to TCP-Chat!\n         _nnnn_\n        dGGGGMMb\n       @p~qp~~qMb\n       M|@||@) M|\n       @,----.JM|\n      JS^\\__/  qKL\n     dZP        qKRb\n    dZP          qKKb\n   fZP            SMMb\n   HZM            MMMM\n   FqM            MMMM\n __| \".        |\\dS\"qML\n |    `.       | `' \\Zq\n_)      \\.___.,|     .'\n\\____   )MMMMMP|   .'\n     `-'       `--'\n[ENTER YOUR NAME]:"
	MaxMessageSize  = 128
	MessageCooldown = 1 * time.Second
	MaxSpamAttempts = 5
)

// Writer writes messages from the client's out channel to their TCP connection.
func (c *Client) Writer() {
	for msg := range c.Out {
		_, err := c.Conn.Write([]byte(msg + "\n"))
		if err != nil {
			return
		}
	}
}

// Reader reads incoming lines from the client's TCP connection.
func (c *Client) Reader(room *Room, s *Server) {
	c.currentRoom = room
	scanner := bufio.NewScanner(c.Conn)

	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		// Handle /switch command
		if strings.HasPrefix(text, "/switch ") {
			newRoomName := strings.TrimSpace(text[8:])
			if newRoomName == "" {
				c.Out <- "Usage: /switch <roomname>"
				continue
			}

			newRoom, err := s.GetRoom(newRoomName)
			if err != nil {
				c.Out <- fmt.Sprintf("Room not found: %s", newRoomName)
				continue
			}

			// Remove from old room
			oldRoom := c.currentRoom
			oldRoom.Mu.Lock()
			delete(oldRoom.Clients, c.Username)
			oldRoom.Mu.Unlock()

			// Add to new room
			newRoom.Mu.Lock()
			if _, exists := newRoom.Clients[c.Username]; exists {
				newRoom.Mu.Unlock()
				c.Out <- "Username already taken in that room"
				// Re-add to old room
				oldRoom.Mu.Lock()
				oldRoom.Clients[c.Username] = c
				oldRoom.Mu.Unlock()
				continue
			}
			newRoom.Clients[c.Username] = c
			newRoom.Mu.Unlock()

			// Update current room reference
			c.currentRoom = newRoom

			// Announce leave from old room
			oldRoom.Messages <- ChatMessage{
				Timestamp: time.Now(),
				User:      "SERVER",
				Text:      fmt.Sprintf("%s left %s", c.Username, oldRoom.Name),
			}

			// Announce join to new room
			newRoom.Messages <- ChatMessage{
				Timestamp: time.Now(),
				User:      "SERVER",
				Text:      fmt.Sprintf("%s joined %s", c.Username, newRoom.Name),
			}

			// Send new room history
			newRoom.SendHistory(c)
			c.Out <- fmt.Sprintf("Switched to room: %s", newRoom.Name)
			continue
		}

		// Handle /nick command
		if strings.HasPrefix(text, "/nick ") {
			newName := strings.TrimSpace(text[6:])
			if newName == "" || !ValidateUsername(newName) {
				c.Out <- "Invalid username"
				continue
			}

			// Check if new name already exists in room
			c.currentRoom.Mu.Lock()
			if _, exists := c.currentRoom.Clients[newName]; exists && newName != c.Username {
				c.currentRoom.Mu.Unlock()
				c.Out <- "Username already taken in this room"
				continue
			}

			// Update client's username
			oldName := c.Username
			delete(c.currentRoom.Clients, oldName)
			c.Username = newName
			c.currentRoom.Clients[newName] = c
			c.currentRoom.Mu.Unlock()

			// Announce nick change
			c.currentRoom.Messages <- ChatMessage{
				Timestamp: time.Now(),
				User:      "SERVER",
				Text:      fmt.Sprintf("%s changed nickname to %s", oldName, newName),
			}
			continue
		}

		// Handle /stats command
		if text == "/stats" {
			totalRooms := s.GetRoomCount()
			totalUsers := s.GetTotalUserCount()
			c.Out <- fmt.Sprintf("Server Stats: %d rooms, %d total users", totalRooms, totalUsers)
			continue
		}

		if len(text) > MaxMessageSize {
			c.Out <- "Message too long. Maximum 128 characters allowed."
			continue
		}

		if !utf8.ValidString(text) {
			c.Out <- "Invalid UTF-8 characters in message."
			continue
		}

		c.mu.Lock()
		if time.Since(c.lastMessageAt) < MessageCooldown {
			c.spamCount++
			if c.spamCount >= MaxSpamAttempts {
				c.mu.Unlock()
				c.Conn.Write([]byte("You have been disconnected for spamming.\n"))
				s.BanIP(c.Conn.RemoteAddr().String())
				c.Conn.Close()
				return
			}
			c.mu.Unlock()
			c.Out <- "Please wait 1 second between messages."
			continue
		}
		c.lastMessageAt = time.Now()
		c.spamCount = 0
		c.mu.Unlock()

		c.currentRoom.Messages <- ChatMessage{
			Timestamp: time.Now(),
			User:      c.Username,
			Text:      text,
		}
	}

	if !c.SafeClose() {
		return
	}

	c.currentRoom.Mu.Lock()
	delete(c.currentRoom.Clients, c.Username)
	close(c.Out)
	c.currentRoom.Mu.Unlock()

	c.Conn.Close()

	s.AnnounceLeave(c.currentRoom, c.Username)
}

// AcceptLoop accepts incoming connections continuously.
func (s *Server) AcceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		go s.handleNewConnection(conn)
	}
}

func (s *Server) handleNewConnection(conn net.Conn) {
	if s.IsIPBanned(conn.RemoteAddr().String()) {
		conn.Write([]byte("You are banned from this server for 1 minute.\n"))
		conn.Close()
		return
	}

	conn.Write([]byte(logo))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		conn.Close()
		return
	}

	name := strings.TrimRight(scanner.Text(), "\r\n")
	if !ValidateUsername(name) {
		conn.Write([]byte("Invalid username\n"))
		conn.Close()
		return
	}

	c := &Client{
		Conn:     conn,
		Username: name,
		Out:      make(chan string, 32),
	}

	// Show available rooms
	rooms := s.ListRooms()
	roomList := "Available rooms:\n"
	for i, room := range rooms {
		roomList += fmt.Sprintf("%d. %s\n", i+1, room)
	}
	roomList += fmt.Sprintf("%d. [Create new room]\n", len(rooms)+1)
	roomList += "Select room (enter number): "
	conn.Write([]byte(roomList))

	// Read room selection
	if !scanner.Scan() {
		conn.Close()
		return
	}

	selection := strings.TrimSpace(scanner.Text())
	var selectedRoom *Room
	var err error

	if selection == fmt.Sprintf("%d", len(rooms)+1) {
		// Create new room
		conn.Write([]byte("Enter room name: "))
		if !scanner.Scan() {
			conn.Close()
			return
		}
		roomName := strings.TrimSpace(scanner.Text())
		if roomName == "" {
			conn.Write([]byte("Invalid room name\n"))
			conn.Close()
			return
		}
		selectedRoom, err = s.CreateRoom(roomName)
		if err != nil {
			conn.Write([]byte("Failed to create room: " + err.Error() + "\n"))
			conn.Close()
			return
		}
		// Start broadcaster for new room
		go selectedRoom.RoomBroadcaster()
	} else {
		// Join existing room
		roomIndex := 0
		if _, err := fmt.Sscanf(selection, "%d", &roomIndex); err != nil || roomIndex < 1 || roomIndex > len(rooms) {
			conn.Write([]byte("Invalid selection\n"))
			conn.Close()
			return
		}
		selectedRoom = s.Rooms[rooms[roomIndex-1]]
	}

	// Register client in room
	if err := s.RegisterClientInRoom(selectedRoom, c); err != nil {
		conn.Write([]byte("Username already taken in this room\n"))
		conn.Close()
		return
	}

	// Send room history
	selectedRoom.SendHistory(c)

	// Announce join
	s.AnnounceJoin(selectedRoom, c.Username)

	// Start writer and reader
	go c.Writer()
	go c.Reader(selectedRoom, s)
}

// ListenAndServe starts the TCP listener on the given port.
func (s *Server) ListenAndServe(port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	s.AcceptLoop(ln)
	return nil
}
