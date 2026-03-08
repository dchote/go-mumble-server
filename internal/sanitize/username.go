package sanitize

import (
	"regexp"
	"strings"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// Username trims and validates a username. Returns empty string if invalid.
func Username(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return ""
	}
	if !usernameRegex.MatchString(s) {
		return ""
	}
	return s
}
