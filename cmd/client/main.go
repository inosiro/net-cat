package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"netcat/internal/ui"
	"os"
	"strings"
	"unicode/utf8"
)

var (
	verbose     = false
	quiet       = false
	uiMode      = false
	chatHistory []string
)

func main() {
	addr, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if uiMode {
		uiApp, err := ui.NewUI()
		if err != nil {
			log.Fatalf("Failed to create UI: %v\n", err)
		}
		if err := uiApp.Start(addr); err != nil {
			log.Fatalf("UI error: %v\n", err)
		}
		return
	}

	if !quiet {
		log.Printf("Connecting to TCP-Chat server at %s\n", addr)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Connection failed: %v\n", err)
	}
	defer conn.Close()

	if verbose {
		fmt.Println("[*] Connected to server")
	}

	reader := bufio.NewReader(os.Stdin)

	// Read and display welcome banner (skip if quiet)
	if !quiet {
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			log.Fatalf("Failed to read banner: %v\n", err)
		}
		fmt.Print(string(buf[:n]))
	} else {
		// Still need to read banner but don't display
		buf := make([]byte, 4096)
		_, err := conn.Read(buf)
		if err != nil {
			log.Fatalf("Failed to read banner: %v\n", err)
		}
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

	// Send username to server
	_, err = conn.Write([]byte(username + "\n"))
	if err != nil {
		log.Fatalf("Failed to send username: %v\n", err)
	}

	// Read room selection prompt and room list
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read room list: %v\n", err)
	}
	roomListText := string(buf[:n])
	fmt.Print(roomListText)

	// Get room selection from user
	selection, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read room selection: %v\n", err)
	}

	selection = strings.TrimRight(selection, "\r\n")

	// Send room selection
	_, err = conn.Write([]byte(selection + "\n"))
	if err != nil {
		log.Fatalf("Failed to send room selection: %v\n", err)
	}

	// Now start concurrent reader and writer for ongoing chat
	go clientReader(conn, username)
	clientWriter(conn, reader, username)
}

// clientReader continuously reads from the server and prints to stdout
func clientReader(conn net.Conn, username string) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		// Keep last 64 messages in history
		chatHistory = append(chatHistory, line)
		if len(chatHistory) > 64 {
			chatHistory = chatHistory[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Read error: %v\n", err)
	}
	os.Exit(0)
}

// clientWriter continuously reads from stdin and sends to the server
func clientWriter(conn net.Conn, reader *bufio.Reader, username string) {
	for {
		line, err := reader.ReadString('\n')
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
			_, err = conn.Write([]byte(line + "\n"))
			if err != nil {
				log.Printf("Write error: %v\n", err)
				return
			}
		} else if strings.HasPrefix(line, "/") {
			// Handle client-side commands locally
			handleCommand(line, username)
		} else {
			// Regular message
			_, err = conn.Write([]byte(line + "\n"))
			if err != nil {
				log.Printf("Write error: %v\n", err)
				return
			}
		}
	}
}

// handleCommand processes client-side commands
func handleCommand(cmd string, username string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	command := parts[0]

	switch command {
	case "/history":
		if len(chatHistory) == 0 {
			fmt.Println("[No history available]")
			return
		}
		fmt.Println("=== Chat History (last 64 messages) ===")
		for _, msg := range chatHistory {
			fmt.Println(msg)
		}
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

// parseFlags parses -v and -q flags and returns the server address
func parseFlags() (string, error) {
	args := os.Args[1:]
	flagEnd := 0

	for i, arg := range args {
		if arg == "-v" {
			if quiet {
				return "", fmt.Errorf("error: -v and -q are mutually exclusive")
			}
			verbose = true
			flagEnd = i + 1
		} else if arg == "-q" {
			if verbose {
				return "", fmt.Errorf("error: -v and -q are mutually exclusive")
			}
			quiet = true
			flagEnd = i + 1
		} else if arg == "-ui" {
			uiMode = true
			flagEnd = i + 1
		} else if strings.HasPrefix(arg, "-") {
			return "", fmt.Errorf("unknown flag: %s", arg)
		} else {
			// Rest are positional arguments
			break
		}
	}

	// Get remaining positional arguments
	remaining := args[flagEnd:]

	host := "localhost"
	port := "8989"

	if len(remaining) >= 1 {
		host = remaining[0]
	}
	if len(remaining) >= 2 {
		port = remaining[1]
	}
	if len(remaining) > 2 {
		return "", fmt.Errorf("too many arguments")
	}

	return host + ":" + port, nil
}
