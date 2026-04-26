package chat

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

type ClientSession struct {
	Conn      net.Conn
	Username  string
	Reader    *bufio.Reader
	History   []string
	HistoryMu sync.Mutex
}

func NewClient(conn net.Conn, reader *bufio.Reader) {
	session := &ClientSession{
		Conn:    conn,
		Reader:  reader,
		History: make([]string, 0, 64),
	}

	// Read username from user
	username, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read username: %v\n", err)
	}

	username = strings.TrimRight(username, "\r\n")
	if !utf8.ValidString(username) {
		log.Fatalf("No valid utf8 username\n")
	}
	if strings.HasPrefix(username, "/") {
		log.Fatalf("Username cannot start with /\n")
	}

	session.Username = username

	// Send username to server
	_, err = conn.Write([]byte(username + "\n"))
	if err != nil {
		log.Fatalf("Failed to send username: %v\n", err)
	}

	// Now start concurrent reader and writer for ongoing chat
	go session.clientReader()
	session.clientWriter()
}

// clientReader continuously reads from the server and prints to stdout
func (s *ClientSession) clientReader() {
	scanner := bufio.NewScanner(s.Conn)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		// Keep last 64 messages in history (thread-safe)
		s.HistoryMu.Lock()
		s.History = append(s.History, line)
		if len(s.History) > MaxRoomHistory {
			s.History = s.History[1:]
		}
		s.HistoryMu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Read error: %v\n", err)
	}
	os.Exit(0)
}

// clientWriter continuously reads from stdin and sends to the server
func (s *ClientSession) clientWriter() {
	for {
		line, err := s.Reader.ReadString('\n')
		if err != nil {
			log.Printf("Read error: %v\n", err)
			return
		}

		line = strings.TrimRight(line, "\r\n")

		// Commands that are server-side (send to server)
		serverCommands := []string{"/nick", "/stats", "/switch", "/rooms", "/users", "/leave", "/dm"}
		isServerCommand := false
		for _, cmd := range serverCommands {
			if strings.HasPrefix(line, cmd) {
				isServerCommand = true
				break
			}
		}

		if isServerCommand {
			// Send server command to server
			_, err = s.Conn.Write([]byte(line + "\n"))
			if err != nil {
				log.Printf("Write error: %v\n", err)
				return
			}
		} else if strings.HasPrefix(line, "/") {
			// Handle client-side commands locally
			s.handleCommand(line)
		} else {
			// Regular message
			_, err = s.Conn.Write([]byte(line + "\n"))
			if err != nil {
				log.Printf("Write error: %v\n", err)
				return
			}
		}
	}
}

// handleCommand processes client-side commands
func (s *ClientSession) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	command := parts[0]

	switch command {
	case "/history":
		s.HistoryMu.Lock()
		if len(s.History) == 0 {
			s.HistoryMu.Unlock()
			fmt.Println("[No history available]")
			return
		}
		fmt.Println("=== Chat History (last 64 messages) ===")
		for _, msg := range s.History {
			fmt.Println(msg)
		}
		s.HistoryMu.Unlock()
		fmt.Println("=== End History ===")

	case "/switch":
		fmt.Println("Usage: /switch <roomname>")
		fmt.Println("(Sending to server...)")

	case "/nick":
		fmt.Println("Usage: /nick <newname>")
		fmt.Println("(Sending to server...)")

	case "/help":
		fmt.Println("=== Available Commands ===")
		fmt.Println("/nick <name>   - Change your nickname")
		fmt.Println("/switch <room> - Switch to a different room")
		fmt.Println("/history       - Show last 64 messages")
		fmt.Println("/stats         - Show server statistics")
		fmt.Println("/rooms         - List rooms")
		fmt.Println("/users         - List users in room")
		fmt.Println("/leave         - Leave chat and disconnect")
		fmt.Println("/help          - Show this help message")
		fmt.Println("=== End Help ===")

	default:
		fmt.Printf("Unknown command: %s (try /help)\n", command)
	}
}
