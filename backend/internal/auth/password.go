package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id cost parameters (plan 01). They are encoded into every hash, so raising
// them later only affects newly written hashes — existing ones keep verifying against
// the parameters they were created with, with no schema change and no forced reset.
const (
	argonTime       = 3
	argonMemoryKiB  = 64 * 1024
	argonThreads    = 2
	argonSaltLen    = 16
	argonKeyLen     = 32
	argonVersionTag = argon2.Version // 19
)

// errMalformedHash means a stored hash could not be parsed. It is never surfaced to a
// client — Login collapses every failure into ErrInvalidCredentials.
var errMalformedHash = errors.New("malformed password hash")

// HashPassword returns a PHC-format argon2id string:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<b64 salt>$<b64 key>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	return encodeHash(password, salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen), nil
}

// VerifyPassword reports whether password matches the PHC-encoded hash. A malformed
// hash returns an error rather than false so a corrupted row is distinguishable in
// logs from an ordinary wrong password.
func VerifyPassword(password, encoded string) (bool, error) {
	salt, want, time, memory, threads, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	// Constant-time: an early-exit compare leaks how many leading bytes matched, which
	// is enough to reconstruct the digest one byte at a time.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func encodeHash(password string, salt []byte, time, memory uint32, threads uint8, keyLen uint32) string {
	key := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersionTag, memory, time, threads, b64.EncodeToString(salt), b64.EncodeToString(key))
}

func decodeHash(encoded string) (salt, key []byte, time, memory uint32, threads uint8, err error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, key]
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, errMalformedHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, 0, 0, 0, errMalformedHash
	}
	if version != argonVersionTag {
		return nil, nil, 0, 0, 0, fmt.Errorf("%w: unsupported argon2 version %d", errMalformedHash, version)
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return nil, nil, 0, 0, 0, errMalformedHash
	}
	// argon2.IDKey PANICS on a zero time or thread count rather than returning an
	// error, so a hand-edited or badly restored row would crash the process instead of
	// taking Login's "treat it as a wrong password" path.
	if time < 1 || threads < 1 || memory < 1 {
		return nil, nil, 0, 0, 0, fmt.Errorf("%w: cost parameters out of range (m=%d,t=%d,p=%d)", errMalformedHash, memory, time, threads)
	}

	b64 := base64.RawStdEncoding
	if salt, err = b64.DecodeString(parts[4]); err != nil {
		return nil, nil, 0, 0, 0, errMalformedHash
	}
	if key, err = b64.DecodeString(parts[5]); err != nil {
		return nil, nil, 0, 0, 0, errMalformedHash
	}
	if len(salt) == 0 || len(key) == 0 {
		return nil, nil, 0, 0, 0, errMalformedHash
	}

	return salt, key, time, memory, threads, nil
}
