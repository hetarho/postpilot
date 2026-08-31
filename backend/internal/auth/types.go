// Package auth is the identity context: accounts, password hashing, and the sessions
// every other RPC is scoped to. Accounts are provisioned by the operator, never over
// the wire (PRD F-1).
//
// The package is flat on purpose (ARCHITECTURE §2.1 — split only once it is noisy):
// domain types and use-cases here, persistence in store/, transport in rpc/, operator
// provisioning in provision/.
package auth

import (
	"errors"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// ErrInvalidCredentials is the single failure Login ever reports. Callers must not
// refine it: distinguishing "no such account" from "wrong password" is exactly the
// id-enumeration leak the timing equalization in Service.Login is there to prevent.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrDuplicateUser is returned when provisioning an id that already exists.
var ErrDuplicateUser = errors.New("user already exists")

// ErrUserNotFound is what the store reports for an unknown login id. Login must map
// it to ErrInvalidCredentials — it may never reach a client.
var ErrUserNotFound = errors.New("user not found")

// ErrNoSession means the presented token matched no live session — absent, unknown,
// or expired. The three are indistinguishable to the caller by design.
var ErrNoSession = errors.New("no session")

// User is an account. The password hash never leaves this package's boundary: the
// store loads it for verification and the rpc layer maps only the id outward.
type User struct {
	ID           string
	PasswordHash string
	Plan         plan.Plan
	CreatedAt    time.Time
}

// ErrLastMaster refuses the demotion that would leave the deployment with no operator
// account — and therefore no way to promote anyone back.
var ErrLastMaster = errors.New("cannot demote the last remaining master account")

// Session is a login that has not been revoked or expired.
type Session struct {
	// Token is the sha256 hex of the raw cookie value, never the raw value itself.
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Expired reports whether the session is past its fixed lifetime. Sessions do not
// slide: the PRD fixes the window at 30 days from login (plan 01, Non-goals).
func (s Session) Expired(now time.Time) bool { return !now.Before(s.ExpiresAt) }
