package chat

import (
	"fmt"
	"time"
)

type ChatMessage struct {
	Timestamp time.Time
	User      string
	Text      string
}

func (m ChatMessage) FormatMessage() string {
	if m.User == "SERVER" {
		return m.Text
	}
	return fmt.Sprintf("[%s][%s]:%s", m.Timestamp.Format("2006-01-02 15:04:05"), m.User, m.Text)
}
