package auth

import (
	"errors"
	"strings"
	"unicode"
)

const maxEmailBytes = 254

// NormalizeEmail produces the account's unique lookup key. It intentionally enforces only
// the small shape this product needs; delivery remains the provider's authority.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", errors.New("email is required")
	}
	if len(email) > maxEmailBytes {
		return "", errors.New("email is longer than 254 bytes")
	}
	for _, r := range email {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", errors.New("email contains whitespace or a control character")
		}
	}
	if strings.Count(email, "@") != 1 {
		return "", errors.New("email must contain exactly one @")
	}
	local, domain, _ := strings.Cut(email, "@")
	if local == "" {
		return "", errors.New("email local part is required")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", errors.New("email domain must contain a dot")
	}
	for _, label := range labels {
		if label == "" {
			return "", errors.New("email domain labels must not be empty")
		}
	}
	return email, nil
}
