package validate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// Hostname validates a hostname string (1–253 chars, RFC-ish).
func Hostname(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("hostname is required")
	}
	if len(s) > 253 {
		return fmt.Errorf("hostname exceeds 253 characters")
	}
	if !hostnameRe.MatchString(s) {
		return fmt.Errorf("hostname contains invalid characters")
	}
	return nil
}

// Alias validates a drive alias (max 128 chars, printable).
// An empty alias is valid (used to clear).
func Alias(s string) error {
	if len(s) > 128 {
		return fmt.Errorf("alias exceeds 128 characters")
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return fmt.Errorf("alias contains non-printable characters")
		}
	}
	return nil
}

// Username validates a username (3–64 chars, alphanumeric + _.-).
func Username(s string) error {
	s = strings.TrimSpace(s)
	if len(s) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if len(s) > 64 {
		return fmt.Errorf("username exceeds 64 characters")
	}
	if !usernameRe.MatchString(s) {
		return fmt.Errorf("username may only contain letters, numbers, underscores, dots, and hyphens")
	}
	return nil
}

// Name validates a generic name field (1–max chars, non-empty after trim).
func Name(s string, max int) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("name is required")
	}
	if len(s) > max {
		return fmt.Errorf("name exceeds %d characters", max)
	}
	return nil
}
