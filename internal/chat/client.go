package chat

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"unicode/utf8"
)

type ClientSession struct {
	Conn      net.Conn
	Username  string
	Reader    *bufio.Reader
	Writer    *bufio.Writer
	WriteMu   sync.Mutex
	History   []string
	HistoryMu sync.Mutex
}

func NewClient(conn net.Conn, reader *bufio.Reader) {
	session := &ClientSession{
		Conn:    conn,
		Reader:  reader, // stdin reader
		Writer:  bufio.NewWriter(conn),
		History: make([]string, 0, MaxRoomHistory),
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
	session.WriteMu.Lock()
	_, err = session.Writer.WriteString(username + "\n")
	if err != nil {
		log.Fatalf("Failed to send username: %v\n", err)
	}
	session.Writer.Flush()
	session.WriteMu.Unlock()

	// Now start concurrent reader and writer for ongoing chat
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg.Add(2)
	go session.clientReader(ctx, cancel, &wg)
	go session.clientWriter(ctx, cancel, &wg)
	wg.Wait() // blocks until both goroutines finish
}

// clientReader continuously reads from the server and prints to stdout
func (s *ClientSession) clientReader(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	defer wg.Done()
	reader := bufio.NewReader(s.Conn) // network reader
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// line := scanner.Text()
		line, err := reader.ReadString('\n')
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("Read error: %v\n", err)
			}
			cancel()
		}

		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "[USER_LIST_START]" || trimmedLine == "[USER_LIST_END]" || trimmedLine == "[ROOM_LIST_START]" || trimmedLine == "[ROOM_LIST_END]" {
			continue
		}

		fmt.Print(line)
		// Keep last 64 messages in history (thread-safe)
		s.HistoryMu.Lock()
		s.History = append(s.History, line)
		if len(s.History) > MaxRoomHistory {
			s.History = s.History[1:]
		}
		s.HistoryMu.Unlock()
	}
}

// clientWriter continuously reads from stdin and sends to the server
func (s *ClientSession) clientWriter(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := s.Reader.ReadString('\n')
		if err != nil {
			cancel()
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

		if isServerCommand || !strings.HasPrefix(line, "/") {
			// Send server command to server
			s.WriteMu.Lock()
			_, err = s.Writer.WriteString(line + "\n")
			if err != nil {
				cancel()
				return
			}
			s.Writer.Flush()
			s.WriteMu.Unlock()

			// If /leave was sent, close connection and exit
			if strings.HasPrefix(line, "/leave") {
				cancel()
				return
			}
		} else {
			// Handle client-side commands locally: /history, /help
			s.handleCommand(line)
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
		fmt.Printf("=== Chat History (last %d messages) ===\n", MaxRoomHistory)
		for _, msg := range s.History {
			fmt.Print(msg)
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
		for _, line := range GetHelpCommands() {
			fmt.Println(line)
		}

	default:
		fmt.Printf("Unknown command: %s (try /help)\n", command)
	}
}
