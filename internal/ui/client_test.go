package ui

import (
	"bufio"
	"net"
	"testing"
	"time"
)

func TestNewUIClient(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err == nil {
			conn.Write([]byte("Welcome Banner\n[ENTER YOUR NAME]:"))
			conn.Close()
		}
	}()

	u, err := NewUI()
	if err != nil {
		t.Fatal(err)
	}
	defer u.g.Close()

	c, err := NewUIClient(l.Addr().String(), u)
	if err != nil {
		t.Fatalf("NewUIClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("Expected client, got nil")
	}
}

func TestUIClient_SendMethods(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	u, err := NewUI()
	if err != nil {
		t.Fatal(err)
	}
	defer u.g.Close()

	client := &UIClient{
		conn: clientConn,
		ui:   u,
	}
	client.username = "testuser"

	go func() {
		client.SendUsername()
		client.SendRoomSelection("room1")
		client.SendMessage("hello world")
	}()

	buf := make([]byte, 1024)

	// read username
	serverConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read username: %v", err)
	}
	if string(buf[:n]) != "testuser\n" {
		t.Errorf("Expected testuser\\n, got %q", string(buf[:n]))
	}

	// read room
	serverConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err = serverConn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read room: %v", err)
	}
	if string(buf[:n]) != "room1\n" {
		t.Errorf("Expected room1\\n, got %q", string(buf[:n]))
	}

	// read message
	serverConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err = serverConn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	if string(buf[:n]) != "hello world\n" {
		t.Errorf("Expected hello world\\n, got %q", string(buf[:n]))
	}
}

func TestUIClient_Reader(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	u, err := NewUI()
	if err != nil {
		t.Fatal(err)
	}
	defer u.g.Close()

	client := &UIClient{
		conn: clientConn,
		ui:   u,
		br:   bufio.NewReader(clientConn),
	}

	go client.reader()

	// send some lines from the "server"
	go func() {
		serverConn.Write([]byte("[ROOM_LIST_START]\nRoom1\nRoom2\n[ROOM_LIST_END]\n"))
		serverConn.Write([]byte("[USER_LIST_START]\nUser1\nUser2\n[USER_LIST_END]\n"))
		serverConn.Write([]byte("Switched to room: Room1\n"))
		serverConn.Write([]byte("Invalid username\n"))
		serverConn.Write([]byte("Goodbye!\n"))
	}()

	expectedEvents := []UIEvent{
		{Type: UIEventRoomsUpdate, Items: []string{"Room1", "Room2"}},
		{Type: UIEventUsersUpdate, Items: []string{"User1", "User2"}},
		{Type: UIEventChatMessage, Text: "Switched to room: Room1"},
		{Type: UIEventUsernameError, Text: "Invalid username"},
		{Type: UIEventChatMessage, Text: "Goodbye!"},
		{Type: UIEventQuit},
	}

	for i, expected := range expectedEvents {
		select {
		case ev := <-u.events:
			if ev.Type != expected.Type {
				t.Errorf("Event %d: expected type %v, got %v", i, expected.Type, ev.Type)
			}
			if ev.Text != expected.Text {
				t.Errorf("Event %d: expected text %q, got %q", i, expected.Text, ev.Text)
			}
			if expected.Type == UIEventRoomsUpdate || expected.Type == UIEventUsersUpdate {
				if len(ev.Items) != len(expected.Items) {
					t.Errorf("Event %d: expected items %v, got %v", i, expected.Items, ev.Items)
				} else {
					for j, item := range ev.Items {
						if item != expected.Items[j] {
							t.Errorf("Event %d item %d: expected %q, got %q", i, j, expected.Items[j], item)
						}
					}
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for event %d", i)
		}
	}
}

func TestUIClient_Reader_AwaitingRoomJoin(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	u, err := NewUI()
	if err != nil {
		t.Fatal(err)
	}
	defer u.g.Close()

	client := &UIClient{
		conn:             clientConn,
		ui:               u,
		br:               bufio.NewReader(clientConn),
		awaitingRoomJoin: true,
		username:         "myuser",
	}

	go client.reader()

	go func() {
		serverConn.Write([]byte("Welcome to the chat!\n"))
	}()

	expectedEvents := []UIEvent{
		{Type: UIEventUsersUpdate, Items: []string{"myuser"}},
		{Type: UIEventChatMessage, Text: "Welcome to TCP-Chat!\n         _nnnn_\n        dGGGGMMb\n       @p~qp~~qMb\n       M|@||@) M|\n       @,----.JM|\n      JS^\\__/  qKL\n     dZP        qKRb\n    dZP          qKKb\n   fZP            SMMb\n   HZM            MMMM\n   FqM            MMMM\n __| \".        |\\dS\"qML\n |    `.       | `' \\Zq\n_)      \\.___.,|     .'\n\\____   )MMMMMP|   .'\n     `-'       `--'"},
		{Type: UIEventChatMessage, Text: "Use /help for commands."},
		{Type: UIEventChatMessage, Text: "Welcome to the chat!"},
	}

	for i, expected := range expectedEvents {
		select {
		case ev := <-u.events:
			if ev.Type != expected.Type {
				t.Errorf("Event %d: expected type %v, got %v", i, expected.Type, ev.Type)
			}
			if ev.Text != expected.Text {
				t.Errorf("Event %d: expected text %q, got %q", i, expected.Text, ev.Text)
			}
			if expected.Type == UIEventUsersUpdate {
				if len(ev.Items) != len(expected.Items) || ev.Items[0] != expected.Items[0] {
					t.Errorf("Event %d: expected items %v, got %v", i, expected.Items, ev.Items)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for event %d", i)
		}
	}

	if client.awaitingRoomJoin {
		t.Errorf("Expected awaitingRoomJoin to be false")
	}
}
