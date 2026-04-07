package chat

import (
	"net"
	"sync"
	"time"
)

type Client struct {
	Conn          net.Conn
	Username      string
	Out           chan string
	currentRoom   *Room
	closed        bool
	mu            sync.Mutex
	lastMessageAt time.Time
	spamCount     int
}

func (c *Client) SafeClose() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	c.closed = true
	return true
}
