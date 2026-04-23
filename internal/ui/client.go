package ui

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type UIClient struct {
	conn                    net.Conn
	username                string
	ui                      *UI
	awaitingInitialRoomList bool
	awaitingRoomJoin        bool
}

func NewUIClient(addr string, ui *UI) (*UIClient, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	client := &UIClient{
		conn: conn,
		ui:   ui,
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	_ = string(buf[:n])

	go client.reader()

	return client, nil
}

func (c *UIClient) SendUsername() {
	c.conn.Write([]byte(c.username + "\n"))
	c.awaitingInitialRoomList = true
}

func (c *UIClient) SendRoomSelection(selection string) {
	c.conn.Write([]byte(selection + "\n"))
	c.awaitingRoomJoin = true
}

func (c *UIClient) SendMessage(msg string) {
	c.conn.Write([]byte(msg + "\n"))
}

func (c *UIClient) reader() {
	scanner := bufio.NewScanner(c.conn)
	rooms := []string{}
	users := []string{}
	collectingInitialRooms := false
	collectingRooms := false
	collectingUsers := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Switched to room:") {
			c.ui.AddChatMessage(line)
			continue
		}

		if line == "Invalid username" || line == "Chat is full. Try again later." || strings.HasPrefix(line, "You are banned") {
			c.ui.showUsernameError(line)
			continue
		}

		if c.awaitingInitialRoomList {
			if line == "Available rooms:" {
				rooms = []string{}
				collectingInitialRooms = true
				continue
			}

			if collectingInitialRooms {
				if strings.HasPrefix(line, "Select room (enter number):") {
					c.ui.SetRooms(rooms)
					c.ui.showRoomSelection()
					collectingInitialRooms = false
					c.awaitingInitialRoomList = false
					continue
				}

				parts := strings.SplitN(line, ". ", 2)
				if len(parts) == 2 {
					roomName := strings.TrimSpace(parts[1])
					if roomName != "" && !strings.Contains(roomName, "[Create new room]") {
						rooms = append(rooms, roomName)
					}
				}
			}

			continue
		}

		if c.awaitingRoomJoin {
			if strings.HasPrefix(line, "Invalid selection") || strings.HasPrefix(line, "Invalid room name") || strings.HasPrefix(line, "Failed to create room") || strings.HasPrefix(line, "Username already taken") {
				c.awaitingRoomJoin = false
				c.ui.showRoomError(line)
				continue
			}

			c.awaitingRoomJoin = false
			c.ui.users = []string{c.username}
			c.ui.showChatLayout()
		}

		if line == "Available rooms:" {
			collectingRooms = true
			rooms = []string{}
			continue
		}

		if collectingRooms {
			if strings.HasPrefix(line, "  ") && strings.Contains(line, "(") {
				trimmed := strings.TrimSpace(line)
				if idx := strings.LastIndex(trimmed, " ("); idx > 0 {
					roomName := strings.TrimSpace(trimmed[:idx])
					rooms = append(rooms, roomName)
					c.ui.UpdateRooms(rooms)
				}
				continue
			}

			collectingRooms = false
			c.ui.UpdateRooms(rooms)
			// Fall through to process the current line, otherwise room events can be dropped.
		}

		if line == "All users:" {
			collectingUsers = true
			users = []string{}
			continue
		}

		if collectingUsers {
			if strings.HasPrefix(line, "  [") {
				parts := strings.SplitN(line, "]: ", 2)
				if len(parts) == 2 {
					userList := strings.Split(parts[1], ", ")
					users = append(users, userList...)
					c.ui.UpdateUsers(users)
				}
				continue
			}

			collectingUsers = false
			c.ui.UpdateUsers(users)
			// Fall through to process the current line, otherwise room events can be dropped.
		}

		// System messages about joining/leaving are now handled by broadcaster


		c.ui.AddChatMessage(line)
	}

	if collectingRooms {
		c.ui.UpdateRooms(rooms)
	}
	if collectingUsers {
		c.ui.UpdateUsers(users)
	}

	if err := scanner.Err(); err != nil {
		c.ui.AddChatMessage(fmt.Sprintf("Connection error: %v", err))
	}
}


