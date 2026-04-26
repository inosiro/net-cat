package chat_test

import (
	"testing"
	"time"

	"netcat/internal/chat"
)

func TestFormatMessage(t *testing.T) {
	ts := time.Date(2020, 1, 20, 16, 3, 43, 0, time.UTC)

	tests := []struct {
		name     string
		msg      chat.ChatMessage
		expected string
	}{
		{
			name: "Regular Message",
			msg: chat.ChatMessage{
				Timestamp: ts,
				User:      "Yenlik",
				Text:      "hello",
			},
			expected: "[2020-01-20 16:03:43][Yenlik]: hello",
		},
		{
			name: "Join Event Message",
			msg: chat.ChatMessage{
				Timestamp: ts,
				User:      "SERVER",
				Text:      "Lee has joined our chat...",
			},
			expected: "[2020-01-20 16:03:43] Lee has joined our chat...",
		},
		{
			name: "Leave Event Message",
			msg: chat.ChatMessage{
				Timestamp: ts,
				User:      "SERVER",
				Text:      "Lee has left our chat...",
			},
			expected: "[2020-01-20 16:03:43] Lee has left our chat...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.msg.FormatMessage()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
