package chat

import (
	"fmt"
	"time"
)

const SystemUser = "SERVER"

type ChatMessage struct {
	Timestamp time.Time
	User      string
	Text      string
}

func (m ChatMessage) FormatMessage() string {
	ts := m.Timestamp.Format("2006-01-02 15:04:05")
	if m.User == SystemUser {
		return fmt.Sprintf("[%s] %s", ts, m.Text)
	}
	return fmt.Sprintf("[%s][%s]: %s", ts, m.User, m.Text)
}
