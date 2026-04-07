package chat_test

import (
	"testing"

	"netcat/internal/chat"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		valid    bool
	}{
		{
			name:     "Valid Name",
			username: "Yenlik",
			valid:    true,
		},
		{
			name:     "Empty String",
			username: "",
			valid:    false,
		},
		{
			name:     "Over 32 Characters",
			username: "thisUsernameIsWayTooLongAndExceeds32Chars",
			valid:    false,
		},
		{
			name:     "Contains Control Characters Newline",
			username: "John\nDoe",
			valid:    false,
		},
		{
			name:     "Contains Control Characters Tab",
			username: "John\tDoe",
			valid:    false,
		},
		{
			name:     "Contains Spaces",
			username: "John Doe",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := chat.ValidateUsername(tt.username)
			if valid != tt.valid {
				t.Errorf("Expected valid=%v for username %q, got %v", tt.valid, tt.username, valid)
			}
		})
	}
}
