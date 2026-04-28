package ui

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type UIClient struct {
	conn             net.Conn
	username         string
	ui               *UI
	awaitingRoomJoin bool
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
	c.awaitingRoomJoin = true
}

func (c *UIClient) SendRoomSelection(selection string) {
	c.conn.Write([]byte(selection + "\n"))
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

		if line == "[ROOM_LIST_START]" {
			collectingRooms = true
			rooms = []string{}
			continue
		}
		if line == "[ROOM_LIST_END]" {
			collectingRooms = false
			c.ui.PostRooms(rooms)
			continue
		}
		if collectingRooms {
			if strings.TrimSpace(line) != "" {
				rooms = append(rooms, strings.TrimSpace(line))
			}
			continue
		}

		if line == "[USER_LIST_START]" {
			collectingUsers = true
			users = []string{}
			continue
		}
		if line == "[USER_LIST_END]" {
			collectingUsers = false
			c.ui.PostUsers(users)
			continue
		}
		if collectingUsers {
			if strings.TrimSpace(line) != "" {
				users = append(users, strings.TrimSpace(line))
			}
			continue
		}

		if strings.HasPrefix(line, "Switched to room:") {
			c.ui.PostChatMessage(line)
			continue
		}

		if line == "Invalid username" || line == "Chat is full. Try again later." || strings.HasPrefix(line, "You are banned") {
			c.ui.PostUsernameError(line)
			continue
		}

		if line == "Goodbye!" {
			c.ui.PostChatMessage(line)
			c.ui.Quit()
			return
		}

		if c.awaitingRoomJoin {
			if line == "Invalid username" || line == "Chat is full. Try again later." || strings.HasPrefix(line, "You are banned") || line == "Username already taken in this room" {
				c.awaitingRoomJoin = false
				c.ui.PostUsernameError(line)
				continue
			}

			// First non-error message indicates successful join
			c.awaitingRoomJoin = false
			c.ui.PostUsers([]string{c.username})
			c.ui.showChatLayout()
			const uiLogo = "Welcome to TCP-Chat!\n         _nnnn_\n        dGGGGMMb\n       @p~qp~~qMb\n       M|@||@) M|\n       @,----.JM|\n      JS^\\__/  qKL\n     dZP        qKRb\n    dZP          qKKb\n   fZP            SMMb\n   HZM            MMMM\n   FqM            MMMM\n __| \".        |\\dS\"qML\n |    `.       | `' \\Zq\n_)      \\.___.,|     .'\n\\____   )MMMMMP|   .'\n     `-'       `--'"
			c.ui.PostChatMessage(uiLogo)
			c.ui.PostChatMessage("Use /help for commands.")
		}

		c.ui.PostChatMessage(line)
	}

	if err := scanner.Err(); err != nil {
		c.ui.PostChatMessage(fmt.Sprintf("Connection error: %v", err))
	}
}
