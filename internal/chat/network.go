package chat

import (
	"bufio"
	"net"
	"strings"
	"time"
)

const (
	logo              = "Welcome to TCP-Chat!\n         _nnnn_\n        dGGGGMMb\n       @p~qp~~qMb\n       M|@||@) M|\n       @,----.JM|\n      JS^\\__/  qKL\n     dZP        qKRb\n    dZP          qKKb\n   fZP            SMMb\n   HZM            MMMM\n   FqM            MMMM\n __| \".        |\\dS\"qML\n |    `.       | `' \\Zq\n_)      \\.___.,|     .'\n\\____   )MMMMMP|   .'\n     `-'       `--'\n[ENTER YOUR NAME]:"
	MaxMessageSize    = 128
	MessageCooldown   = 1 * time.Second
	MaxSpamAttempts   = 5
)

// Writer writes messages from the client's out channel to their TCP connection.
func (c *Client) Writer() {
	for msg := range c.Out {
		_, err := c.Conn.Write([]byte(msg + "\n"))
		if err != nil {
			return
		}
	}
}

// Reader reads incoming lines from the client's TCP connection.
func (c *Client) Reader(s *Server) {
	scanner := bufio.NewScanner(c.Conn)

	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if len(text) > MaxMessageSize {
			c.Out <- "Message too long. Maximum 128 characters allowed."
			continue
		}

		c.mu.Lock()
		if time.Since(c.lastMessageAt) < MessageCooldown {
			c.spamCount++
			if c.spamCount >= MaxSpamAttempts {
				c.mu.Unlock()
				c.Conn.Write([]byte("You have been disconnected for spamming.\n"))
				s.BanIP(c.Conn.RemoteAddr().String())
				c.Conn.Close()
				return
			}
			c.mu.Unlock()
			c.Out <- "Please wait 1 second between messages."
			continue
		}
		c.lastMessageAt = time.Now()
		c.spamCount = 0
		c.mu.Unlock()

		s.Messages <- ChatMessage{
			Timestamp: time.Now(),
			User:      c.Username,
			Text:      text,
		}
	}

	s.DisconnectClient(c)
}

// AcceptLoop accepts incoming connections continuously.
func (s *Server) AcceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}

		s.Mu.Lock()
		if len(s.Clients) >= MaxClients {
			s.Mu.Unlock()
			conn.Write([]byte("Chat is full. Try again later.\n"))
			conn.Close()
			continue
		}
		s.Mu.Unlock()

		go s.handleNewConnection(conn)
	}
}

func (s *Server) handleNewConnection(conn net.Conn) {
	if s.IsIPBanned(conn.RemoteAddr().String()) {
		conn.Write([]byte("You are banned from this server.\n"))
		conn.Close()
		return
	}

	conn.Write([]byte(logo))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		conn.Close()
		return
	}

	name := strings.TrimRight(scanner.Text(), "\r\n")
	if !ValidateUsername(name) {
		conn.Write([]byte("Invalid username\n"))
		conn.Close()
		return
	}

	c := &Client{
		Conn:     conn,
		Username: name,
		Out:      make(chan string, 32),
	}

	if err := s.RegisterClient(c); err != nil {
		conn.Write([]byte("Username already taken\n"))
		conn.Close()
		return
	}

	s.SendHistory(c)
	s.AnnounceJoin(c.Username)

	go c.Writer()
	go c.Reader(s)
}

// ListenAndServe starts the TCP listener on the given port.
func (s *Server) ListenAndServe(port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	s.AcceptLoop(ln)
	return nil
}
