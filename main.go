package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"netcat/internal/chat"
	"netcat/internal/ui"
	"os"
)

func main() {
	// ui and address flag for client mode
	var uiFlag = flag.Bool("ui", false, "Enable console user interface (client)")
	var addrFlag = flag.String("s", "127.0.0.1", "Specify source `IP address` to use")

	// output log and listen flag for server mode
	var outputFlag = flag.String("o", "", "Dump session data to a `filename` (server)")
	var listenFlag = flag.Bool("l", false, "Bind and listen for incoming connections (server)")

	flag.Parse()
	argsAfterFlags := flag.Args()
	port := "8989"
	if len(argsAfterFlags) > 1 {
		fmt.Fprintln(os.Stderr, "[USAGE]: ./TCPChat $port")
		os.Exit(1)
	}
	if len(argsAfterFlags) == 1 {
		port = argsAfterFlags[0]
	}

	if *listenFlag { // server mode

		if len(*outputFlag) > 0 {
			f, err := os.OpenFile(*outputFlag, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				log.Fatalf("Error opening file: %v", err)
			}
			defer f.Close()
			log.SetOutput(f)
		}

		log.Printf("Starting TCP-Chat server on port %s\n", port)

		s := chat.NewServer()

		// Start broadcaster for Main Room
		mainRoom, _ := s.GetRoom("Main Room")
		go mainRoom.RoomBroadcaster()

		if err := s.ListenAndServe(port); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	} else { // client mode

		if *uiFlag {
			uiApp, err := ui.NewUI()
			if err != nil {
				log.Fatalf("Failed to create UI: %v\n", err)
			}
			if err := uiApp.Start(*addrFlag + ":" + port); err != nil {
				log.Fatalf("UI error: %v\n", err)
			}
			return
		} else {
			log.Printf("Connecting to TCP-Chat server at %s\n", *addrFlag)
			conn, err := net.Dial("tcp", *addrFlag+":"+port)
			if err != nil {
				log.Fatalf("Connection failed: %v\n", err)
			}
			defer conn.Close()
			fmt.Println("[*] Connected to server")
			reader := bufio.NewReader(os.Stdin)

			buf := make([]byte, 4096)
			n, err := conn.Read(buf)
			if err != nil {
				log.Fatalf("Failed to read banner: %v\n", err)
			}
			fmt.Print(string(buf[:n]))
			chat.NewClient(conn, reader)
			return
		}
	}
}
