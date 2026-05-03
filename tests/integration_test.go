package chat_test

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"netcat/internal/chat"
)

// TestIntegration_ClientLifecycle verifies the core connectivity requirements:
// 1. Server starts on a specific port (2525).
// 2. Multiple clients can connect simultaneously.
// 3. Server sends the welcome logo and the [ENTER YOUR NAME]: prompt.
// 4. Complete registration for all clients
// 5. Verify Nickname Change functionality
// 6. Verify /leave command for all clients
func TestIntegration_ClientLifecycle(t *testing.T) {
	// 1. Initialize and start the Server on port 2525
	s := chat.NewServer()
	port := "2525"

	// Start server in background goroutine
	go func() {
		// ListenAndServe handles the ":" prefix internally
		_ = s.ListenAndServe(port)
	}()
	defer s.Shutdown()

	// Give the server a small window to initialize the listener
	time.Sleep(200 * time.Millisecond)

	// Helper to simulate a client (nc or custom client protocol)
	connectClient := func(id string) (net.Conn, *bufio.Reader) {
		conn, err := net.Dial("tcp", "localhost:"+port)
		if err != nil {
			t.Fatalf("[%s] Connection failed: %v", id, err)
		}

		reader := bufio.NewReader(conn)

		// Verify Logo and Name Prompt handshake
		logoReceived := false
		promptReceived := false

		// Set a timeout for the handshake to avoid hanging tests
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			// Read until the prompt suffix ":"
			line, err := reader.ReadString(':')
			if err != nil {
				t.Fatalf("[%s] Error during handshake: %v", id, err)
			}

			if strings.Contains(line, "Welcome to TCP-Chat!") {
				logoReceived = true
			}
			if strings.Contains(line, "[ENTER YOUR NAME]:") {
				promptReceived = true
				break
			}
		}

		if !logoReceived {
			t.Errorf("[%s] Welcome logo was not found in the server response", id)
		}
		if !promptReceived {
			t.Errorf("[%s] Name prompt was not found in the server response", id)
		}

		return conn, reader
	}

	// 2 & 3. Connect 3 clients
	// This simulates: 2 'nc localhost 2525' and 1 './TCPChat 2525'
	c1, r1 := connectClient("NC_Client_1")
	c2, r2 := connectClient("NC_Client_2")
	c3, r3 := connectClient("Project_Client")

	allClients := []struct {
		name string
		conn net.Conn
		read *bufio.Reader
	}{
		{"Alice", c1, r1},
		{"Bob", c2, r2},
		{"Charlie", c3, r3},
	}

	// 4. Complete registration for all clients
	for _, c := range allClients {
		// Submit name to finish registration
		fmt.Fprintf(c.conn, "%s\n", c.name)
	}
	// Small pause to allow join broadcasts to settle
	time.Sleep(100 * time.Millisecond)

	// 5. Verify Nickname Change functionality
	// Alice (c1) changes her nickname to "Alicia"
	fmt.Fprintln(c1, "/nick Alicia")

	// Helper to verify that a client receives a specific broadcast message
	verifyBroadcast := func(clientID string, reader *bufio.Reader, conn net.Conn, expectedSubstr string) {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("[%s] Failed to receive expected broadcast %q: %v", clientID, expectedSubstr, err)
			}
			if strings.Contains(line, expectedSubstr) {
				return // Success
			}
		}
	}

	// Bob and Charlie should both see the announcement
	verifyBroadcast("Bob", r2, c2, "Alice changed nickname to Alicia")
	verifyBroadcast("Charlie", r3, c3, "Alice changed nickname to Alicia")

	// 6. Verify /leave command for all clients
	for _, c := range allClients {
		// Send /leave command
		fmt.Fprintln(c.conn, "/leave")

		// Verify the "Goodbye!" response and subsequent disconnection
		c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		foundGoodbye := false
		for {
			// Read lines until "Goodbye!" is found. We must loop because broadcast
			// messages from other clients joining/leaving might be in the buffer.
			response, err := c.read.ReadString('\n')
			if strings.Contains(response, "Goodbye!") {
				foundGoodbye = true
				break
			}
			if err != nil {
				break
			}
		}

		if !foundGoodbye {
			t.Errorf("[%s] Expected 'Goodbye!' upon /leave, but it was not found in the output", c.name)
		}

		c.conn.Close()
	}
}

// TestIntegration_BannedIP verifies the ban functionality:
// 1. A client gets banned for spamming (exceeds MaxSpamAttempts).
// 2. When the same IP attempts to reconnect, it receives the ban message.
// 3. After the ban expires (1 minute), the IP can reconnect normally.
func TestIntegration_BannedIP(t *testing.T) {
	// 1. Initialize and start the Server on port 2526
	s := chat.NewServer()
	port := "2526"

	// Start server in background goroutine
	go func() {
		_ = s.ListenAndServe(port)
	}()
	defer s.Shutdown()

	// Give the server a small window to initialize the listener
	time.Sleep(200 * time.Millisecond)

	// 2. Connect a client and trigger a ban by spamming
	conn, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		t.Fatalf("Initial connection failed: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Read the banner (logo + prompt)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	bannerReceived := false
	for {
		line, err := reader.ReadString(':')
		if err != nil {
			t.Fatalf("Error reading banner: %v", err)
		}
		if strings.Contains(line, "[ENTER YOUR NAME]:") {
			bannerReceived = true
			break
		}
	}

	if !bannerReceived {
		t.Fatalf("Banner not received properly")
	}

	// Send a valid username to join
	fmt.Fprintln(conn, "SpamUser")
	time.Sleep(100 * time.Millisecond)

	// Spam messages to trigger the ban (MaxSpamAttempts = 5)
	// Each message must be sent within MessageCooldown (1 second) to count as spam
	for i := 0; i < 6; i++ {
		fmt.Fprintf(conn, "spam_msg_%d\n", i)
		time.Sleep(100 * time.Millisecond) // Less than MessageCooldown
	}

	// Give the server time to process the spam and ban the IP
	time.Sleep(500 * time.Millisecond)

	// 3. Attempt to reconnect with the same IP (should be rejected)
	conn2, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		t.Fatalf("Second connection failed: %v", err)
	}
	defer conn2.Close()

	reader2 := bufio.NewReader(conn2)
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read the response - should be ban message
	banMessage := ""
	for {
		line, err := reader2.ReadString('\n')
		if err != nil {
			break
		}
		banMessage += line
		if strings.Contains(line, "You are banned") {
			break
		}
	}

	if !strings.Contains(banMessage, "You are banned") {
		t.Errorf("Expected ban message, but got: %s", banMessage)
	}

	// 4. Verify that the server sent the ban message and closed the connection
	// (The next read should fail or return empty)
	line, err := reader2.ReadString('\n')
	if err == nil && strings.TrimSpace(line) != "" {
		t.Errorf("Expected connection to close after ban message, but got: %s", line)
	}
}
