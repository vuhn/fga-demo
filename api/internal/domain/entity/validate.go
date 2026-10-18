package entity

import (
	"regexp"
	"strings"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidUUID(s string) bool { return uuidRe.MatchString(s) }

// RequireUUID returns a validation error when s is not a UUID.
func RequireUUID(field, s string) error {
	if !ValidUUID(s) {
		return Invalid("%s must be a UUID", field)
	}
	return nil
}

// CleanName trims a display name and enforces 1 to 200 characters.
func CleanName(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	if n := len([]rune(s)); n < 1 || n > 200 {
		return "", Invalid("%s length must be 1 to 200", field)
	}
	return s, nil
}
