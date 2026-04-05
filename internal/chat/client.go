package chat

import (
	"net"
	"sync"
)

type Client struct {
	conn     net.Conn
	username string
	out      chan string
	closed   bool
	mu       sync.Mutex
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
