package chat

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	logo            = "Welcome to TCP-Chat!\n         _nnnn_\n        dGGGGMMb\n       @p~qp~~qMb\n       M|@||@) M|\n       @,----.JM|\n      JS^\\__/  qKL\n     dZP        qKRb\n    dZP          qKKb\n   fZP            SMMb\n   HZM            MMMM\n   FqM            MMMM\n __| \".        |\\dS\"qML\n |    `.       | `' \\Zq\n_)      \\.___.,|     .'\n\\____   )MMMMMP|   .'\n     `-'       `--'"
	namePrompt      = "[ENTER YOUR NAME]:"
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

func (c *Client) disconnect(s *Server, announceLeave bool) {
	s.DisconnectClientFromRoom(c.currentRoom, c.Username)
	logConnectionDisconnected(c.Conn)
	close(c.Out)
	c.Conn.Close()
	log.Printf("%s disconnected.\n", c.Username)
	if announceLeave {
		s.AnnounceLeave(c.currentRoom, c.Username)
	}
}

func logConnectionDisconnected(conn net.Conn) {
	log.Printf("Connection from %s disconnected.\n", conn.RemoteAddr().String())
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

		// Handle /leave command
		if text == "/leave" {
			c.Conn.Write([]byte("Goodbye!\n"))

			// Properly clean up the client
			if c.SafeClose() {
				c.disconnect(s, true)
			}
			return
		}

		// Handle /dm command
		if strings.HasPrefix(text, "/dm ") {
			parts := strings.SplitN(text[4:], " ", 2)
			if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
				c.Out <- "Usage: /dm <username> <message>"
				continue
			}
			targetUsername := strings.TrimSpace(parts[0])
			message := strings.TrimSpace(parts[1])

			// Find target user across all rooms
			var targetClient *Client
			rooms := s.ListRooms()
			for _, roomName := range rooms {
				room, _ := s.GetRoom(roomName)
				room.Mu.Lock()
				if client, exists := room.Clients[targetUsername]; exists {
					targetClient = client
					room.Mu.Unlock()
					break
				}
				room.Mu.Unlock()
			}

			if targetClient == nil {
				c.Out <- "This username doesnt exists"
				continue
			}

			// Send DM to target user
			dmMessage := fmt.Sprintf("[DM From %q][%s]: %s", c.Username, time.Now().Format("2006-01-02 15:04:05"), message)
			targetClient.Out <- dmMessage

			// Send confirmation to sender
			c.Out <- fmt.Sprintf("DM sent to %s", targetUsername)
			continue
		}

		// Handle /rooms command
		if text == "/rooms" {
			rooms := s.ListRooms()
			c.Out <- "Available rooms:"
			for _, roomName := range rooms {
				room, _ := s.GetRoom(roomName)
				c.Out <- fmt.Sprintf("  %s (%d users)", roomName, room.ClientCount())
			}
			continue
		}

		// Handle /users command
		if text == "/users" {
			c.Out <- "All users:"
			clients := c.currentRoom.GetClients()
			if len(clients) > 0 {
				c.Out <- fmt.Sprintf("  [%s]: %s", c.currentRoom.Name, strings.Join(clients, ", "))
			}
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
				// Room doesn't exist, create it
				newRoom, err = s.CreateRoom(newRoomName)
				if err != nil {
					c.Out <- fmt.Sprintf("Failed to create room: %s", err.Error())
					continue
				}
				// Start broadcaster for new room
				go newRoom.RoomBroadcaster()
			}

			// Remove from old room
			oldRoom := c.currentRoom
			s.DisconnectClientFromRoom(oldRoom, c.Username)

			// Add to new room
			newRoom.Mu.Lock()
			if _, exists := newRoom.Clients[c.Username]; exists {
				newRoom.Mu.Unlock()
				c.Out <- "Username already taken in that room"
				// Re-add to old room
				oldRoom.Mu.Lock()
				oldRoom.Clients[c.Username] = c
				oldRoom.Mu.Unlock()

				select {
				case s.RoomUpdates <- struct{}{}:
				default:
				}
				select {
				case oldRoom.UserUpdates <- struct{}{}:
				default:
				}

				continue
			}
			newRoom.Clients[c.Username] = c
			newRoom.Mu.Unlock()

			select {
			case s.RoomUpdates <- struct{}{}:
			default:
			}
			select {
			case newRoom.UserUpdates <- struct{}{}:
			default:
			}

			// Update current room reference
			c.currentRoom = newRoom
			log.Printf("%s switched room from %s to %s\n", c.Username, oldRoom.Name, newRoom.Name)

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

			select {
			case c.currentRoom.UserUpdates <- struct{}{}:
			default:
			}

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
				if c.SafeClose() {
					c.disconnect(s, true)
				}
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

	c.disconnect(s, true)
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
	log.Printf("Incoming connection from %s\n", conn.RemoteAddr().String())

	if s.IsIPBanned(conn.RemoteAddr().String()) {
		conn.Write([]byte("You are banned from this server for 1 minute.\n"))
		logConnectionDisconnected(conn)
		conn.Close()
		return
	}

	conn.Write([]byte(logo + "\n"))
	conn.Write([]byte("Use /help for commands.\n"))
	conn.Write([]byte(namePrompt))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		logConnectionDisconnected(conn)
		conn.Close()
		return
	}

	name := strings.TrimRight(scanner.Text(), "\r\n")
	if !ValidateUsername(name) {
		conn.Write([]byte("Invalid username\n"))
		logConnectionDisconnected(conn)
		conn.Close()
		return
	}

	c := &Client{
		Conn:     conn,
		Username: name,
		Out:      make(chan string, 32),
	}

	// Get Main Room
	selectedRoom, err := s.GetRoom("Main Room")
	if err != nil {
		// Should never happen since Main Room is created on startup, but handle it just in case
		conn.Write([]byte("Server error: Main Room not found\n"))
		logConnectionDisconnected(conn)
		conn.Close()
		return
	}

	// Register client in room
	if err := s.RegisterClientInRoom(selectedRoom, c); err != nil {
		if errors.Is(err, ErrServerFull) {
			conn.Write([]byte("Chat is full. Try again later.\n"))
			logConnectionDisconnected(conn)
			conn.Close()
			return
		}
		conn.Write([]byte("Username already taken in this room\n"))
		logConnectionDisconnected(conn)
		conn.Close()
		return
	}
	log.Printf("%s connected.\n", c.Username)

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
	ln, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		return err
	}
	s.AcceptLoop(ln)
	return nil
}
