package main

import (
	"fmt"
	"log"
	"os"

	"netcat/internal/chat"
)

func main() {
	port := getPort()

	log.Printf("Starting TCP-Chat server on port %s\n", port)

	s := chat.NewServer()

	// Start broadcaster for Main Room
	mainRoom, _ := s.GetRoom("Main Room")
	go mainRoom.RoomBroadcaster()

	if err := s.ListenAndServe(port); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}

func getPort() string {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "[USAGE]: ./TCPChat $port")
		os.Exit(1)
	}

	if len(os.Args) == 2 {
		return os.Args[1]
	}

	return "8989"
}
