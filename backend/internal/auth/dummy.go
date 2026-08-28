package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
)

// dummyHash returns a valid argon2id hash whose password nobody knows.
//
// Login verifies against it when the login id does not exist, so the "no such account"
// path costs one real argon2id derivation — the same as "wrong password". Without it,
// a caller could enumerate valid ids by timing alone, which is the only enumeration
// defense this two-user tool has (no rate limiting, plan 01 Non-goals).
//
// It is derived at first use rather than pasted in as a constant: a literal would
// freeze today's cost parameters, so the day argonTime/argonMemoryKiB are raised the
// dummy path would silently become the cheaper one and reopen the leak. Deriving it
// keeps the two paths matched by construction.
var dummyHash = sync.OnceValue(func() string {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// crypto/rand failing means the process has no usable entropy source; every
		// token and salt it goes on to mint would be unsafe.
		panic("auth: cannot read random bytes for the dummy hash: " + err.Error())
	}
	encoded, err := HashPassword(base64.RawStdEncoding.EncodeToString(secret))
	if err != nil {
		panic("auth: cannot build the dummy hash: " + err.Error())
	}
	return encoded
})
