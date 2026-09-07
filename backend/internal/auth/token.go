package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// sessionTokenBytes is the raw entropy behind a session cookie. 256 bits is far past
// brute force, which matters because a session is the only credential a request
// carries and it lives for 30 days.
const sessionTokenBytes = 32

// Link lifetimes are product rules rather than deployment settings.
const (
	VerifyLinkTTL = 24 * time.Hour
	ResetLinkTTL  = time.Hour
)

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

// NewLinkToken mints the credential placed in a verification or reset URL. It has the
// same entropy and storage treatment as a session token: only hex(sha256(raw)) is stored.
func NewLinkToken() (raw, hashed string, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate auth link token: %w", err)
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
