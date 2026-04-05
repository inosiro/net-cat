package chat

import "unicode"

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
