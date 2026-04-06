package main

import (
	"fmt"
	"os"

	"netcat/internal/chat"
)

func main() {
	port := "8989"

	switch len(os.Args) {
	case 1:
		// No port specified — use default
	case 2:
		port = os.Args[1]
	default:
		fmt.Println("[USAGE]: ./TCPChat $port")
		os.Exit(1)
	}

	fmt.Printf("Listening on the port :%s\n", port)

	s := chat.NewServer()
	go s.Broadcaster()

	if err := s.ListenAndServe(port); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
