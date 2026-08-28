package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// sessionTokenBytes is the raw entropy behind a session cookie. 256 bits is far past
// brute force, which matters because a session is the only credential a request
// carries and it lives for 30 days.
const sessionTokenBytes = 32

// newSessionToken mints a session token, returning the raw value for the cookie and
// the hashed value for the database. The raw value is never persisted.
func newSessionToken() (raw, hashed string, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

// hashToken maps a raw cookie value to its stored form.
//
// A plain sha256 is deliberate where passwords get argon2id: the input is 256 bits of
// uniform randomness, so there is no dictionary to attack and no reason to pay a KDF
// on every single request. What this buys is that a leaked database yields no usable
// cookies.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
