package ui

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
)

type UIClient struct {
	conn     net.Conn
	username string
	ui       *UI
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
}

func (c *UIClient) SendRoomSelection(selection string) {
	c.conn.Write([]byte(selection + "\n"))
	
	c.ui.rooms = c.ui.rooms
	c.ui.users = []string{c.username}
	c.ui.g.Update(func(g *gocui.Gui) error {
		c.ui.updateRoomsView()
		c.ui.updateUsersView()
		return nil
	})
	
	go func() {
		time.Sleep(300 * time.Millisecond)
		c.conn.Write([]byte("/users\n"))
		time.Sleep(100 * time.Millisecond)
		c.conn.Write([]byte("/rooms\n"))
	}()
}

func (c *UIClient) SendMessage(msg string) {
	c.conn.Write([]byte(msg + "\n"))
}

func (c *UIClient) reader() {
	scanner := bufio.NewScanner(c.conn)
	rooms := []string{}
	users := []string{}
	collectingRooms := false
	collectingUsers := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "Available rooms:") && strings.Contains(line, "Select room") {
			for scanner.Scan() {
				roomLine := scanner.Text()
				if strings.Contains(roomLine, "Select room") {
					break
				}
				if strings.TrimSpace(roomLine) != "" {
					parts := strings.SplitN(roomLine, ". ", 2)
					if len(parts) == 2 {
						roomName := strings.TrimSpace(parts[1])
						if !strings.Contains(roomName, "[Create new room]") {
							c.ui.rooms = append(c.ui.rooms, roomName)
						}
					}
				}
			}
			c.ui.SetRooms(c.ui.rooms)
			continue
		}

		if line == "Available rooms:" {
			collectingRooms = true
			rooms = []string{}
			c.ui.AddChatMessage(line)
			continue
		}

		if collectingRooms {
			if strings.HasPrefix(line, "  ") && strings.Contains(line, "(") {
				parts := strings.Fields(line)
				if len(parts) >= 1 {
					roomName := strings.TrimSpace(parts[0])
					rooms = append(rooms, roomName)
				}
				c.ui.AddChatMessage(line)
				go func(r []string) {
					time.Sleep(50 * time.Millisecond)
					if collectingRooms {
						collectingRooms = false
						c.ui.rooms = r
						c.ui.UpdateRooms(r)
					}
				}(rooms)
				continue
			} else {
				collectingRooms = false
				if len(rooms) > 0 {
					c.ui.rooms = rooms
					c.ui.UpdateRooms(rooms)
				}
			}
		}

		if line == "All users:" {
			collectingUsers = true
			users = []string{}
			c.ui.AddChatMessage(line)
			continue
		}

		if collectingUsers {
			if strings.HasPrefix(line, "  [") {
				parts := strings.SplitN(line, "]: ", 2)
				if len(parts) == 2 {
					userList := strings.Split(parts[1], ", ")
					users = append(users, userList...)
				}
				c.ui.AddChatMessage(line)
				go func(u []string) {
					time.Sleep(50 * time.Millisecond)
					if collectingUsers {
						collectingUsers = false
						c.ui.UpdateUsers(u)
					}
				}(users)
				continue
			} else {
				collectingUsers = false
				if len(users) > 0 {
					c.ui.UpdateUsers(users)
				}
			}
		}

		c.ui.AddChatMessage(line)
	}

	if err := scanner.Err(); err != nil {
		c.ui.AddChatMessage(fmt.Sprintf("Connection error: %v", err))
	}
}
