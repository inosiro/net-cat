package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"unicode/utf8"
)

func main() {
	addr := getAddress()

	log.Printf("Connecting to TCP-Chat server at %s\n", addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Connection failed: %v\n", err)
	}
	defer conn.Close()

	// Read and display welcome banner
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	// Read username from user
	reader := bufio.NewReader(os.Stdin)
	username, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read username: %v\n", err)
	}

	username = strings.TrimRight(username, "\r\n")
	if !utf8.ValidString(username) {
		log.Fatalf("No valid utf8 username: %s\n", err)
	}

	// Send username to server
	_, err = conn.Write([]byte(username + "\n"))
	if err != nil {
		log.Fatalf("Failed to send username: %v\n", err)
	}

	// Run concurrent reader and writer goroutines
	go clientReader(conn)
	clientWriter(conn, reader)
}

// clientReader continuously reads from the server and prints to stdout
func clientReader(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Read error: %v\n", err)
	}
	os.Exit(0)
}

// clientWriter continuously reads from stdin and sends to the server
func clientWriter(conn net.Conn, reader *bufio.Reader) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Read error: %v\n", err)
			return
		}

		_, err = conn.Write([]byte(line))
		if err != nil {
			log.Printf("Write error: %v\n", err)
			return
		}
	}
}

// getAddress returns the server address from CLI args or defaults
func getAddress() string {
	if len(os.Args) > 3 {
		fmt.Fprintln(os.Stderr, "[USAGE]: ./TCPChatClient [host] [port]")
		os.Exit(1)
	}

	host := "localhost"
	port := "8989"

	if len(os.Args) >= 2 {
		host = os.Args[1]
	}
	if len(os.Args) == 3 {
		port = os.Args[2]
	}

	return host + ":" + port
}
