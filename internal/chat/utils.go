package chat

import (
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
