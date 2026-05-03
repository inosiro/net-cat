package chat

import (
	"fmt"
	"net"
	"strings"
	"unicode"
)

func ValidateUsername(name string) bool {
	if len(name) == 0 || len(name) > 32 {
		return false
	}
	for _, ch := range name {
		if unicode.IsControl(ch) || unicode.IsSpace(ch) {
			return false
		}
	}
	return true
}

func ReadStartupBanner(conn net.Conn) (string, error) {
	buf := make([]byte, 1024)
	var b strings.Builder

	for {
		n, err := conn.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
			if strings.Contains(b.String(), namePrompt) {
				return b.String(), nil
			}
			// Check if we received a ban message
			if strings.Contains(b.String(), "You are banned") {
				return b.String(), nil
			}
		}
		if err != nil {
			return b.String(), err
		}
	}
}

// GetHelpCommands returns the list of available commands for display in help menus
func GetHelpCommands() []string {
	return []string{
		"=== Available Commands ===",
		"/nick <name>   - Change your nickname",
		"/switch <room> - Switch to a different room",
		fmt.Sprintf("/history       - Show last %d messages", MaxRoomHistory),
		"/stats         - Show server statistics",
		"/rooms         - List rooms",
		"/users         - List users in room",
		"/dm <user> <message> - Send a direct message to a user",
		"/leave         - Leave chat and disconnect",
		"/help          - Show this help message",
		"=== End Help ===",
	}
}
