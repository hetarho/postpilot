// Package ids contains format-specific random identifiers with no domain lifecycle.
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

// NewHex128 returns 128 random bits encoded as 32 lowercase hexadecimal characters.
func NewHex128() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
