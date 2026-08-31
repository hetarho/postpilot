// Package guideline is the 작문 지침 context: reusable account-owned rules about what a post
// must avoid or watch out for.
//
// A guideline is authored text and nothing else. Nothing here is learned, inferred, or
// written by a model ([I4] stays entirely with voice), and no behavior in this package
// enqueues work or calls a provider ([I5]). Voice decides how sentences sound and a purpose
// decides genre and required content; a guideline is a prohibition or a caution.
//
// Nothing references a guideline: jobs and experiment snapshots freeze the *texts*, never
// ids, so deleting one affects nothing already enqueued and detaches nothing.
package guideline

import (
	"errors"
	"fmt"
	"time"
)

// Scope decides which posts a guideline reaches. It is deliberately not a boolean: a
// `purposes` guideline with no links left is a real state (every purpose it named was
// deleted) and must stay distinguishable from a global one.
type Scope string

const (
	ScopeGlobal   Scope = "global"
	ScopePurposes Scope = "purposes"
)

func (s Scope) Valid() bool { return s == ScopeGlobal || s == ScopePurposes }

var (
	// ErrNotFound covers unknown and foreign ids alike. A guideline belonging to another
	// account must not be distinguishable from one that never existed.
	ErrNotFound = errors.New("guideline not found")
	// ErrPurposeNotFound is an unknown or foreign purpose id in a scope. It is reported as
	// not-found for the same reason, and nothing about the request is applied.
	ErrPurposeNotFound = errors.New("scoped purpose not found")
	// ErrDuplicateText is a text another guideline of the same account already holds. The
	// texts are the prompt lines, so two identical ones would inject the same rule twice.
	ErrDuplicateText = errors.New("a guideline with that text already exists")
	// ErrInvalidText is the empty-after-trim case: a blank line in the prompt section.
	ErrInvalidText = errors.New("guideline text is required")
	// ErrScopeShape is a scope whose kind and purpose set contradict each other — `global`
	// carrying purpose ids, or `purposes` carrying none. Silently repairing either would
	// save a scope the user did not ask for.
	ErrScopeShape = errors.New("guideline scope shape is invalid")
)

// TextTooLongError carries both counts so the handler can report the limit that was hit
// without re-deriving it.
type TextTooLongError struct {
	Chars int
	Max   int
}

func (e *TextTooLongError) Error() string {
	return fmt.Sprintf("guideline text has %d characters; at most %d are allowed", e.Chars, e.Max)
}

// AccountCapError refuses a create past the per-account ceiling. It names the cap because
// the message the user reads has to say the number.
type AccountCapError struct{ Max int }

func (e *AccountCapError) Error() string {
	return fmt.Sprintf("the account already holds the maximum of %d guidelines", e.Max)
}

// Limits are the configured ceilings, counted in Unicode scalar values so a Hangul syllable
// counts as one character. They come from platform/config; this context holds no default of
// its own, because a silent fallback would let a misconfigured process accept rules the
// frontend already refused.
type Limits struct {
	TextMaxChars  int
	MaxPerAccount int
}

func (l Limits) valid() bool { return l.TextMaxChars > 0 && l.MaxPerAccount > 0 }

// PurposeRef is a purpose as this context needs it: an id it can validate ownership of and a
// name it can show. Names are always a live projection through PurposeDirectory, never a
// column of this context's tables and never a SQL join (ARCHITECTURE §2.2).
type PurposeRef struct {
	ID   string
	Name string
}

// Guideline is the aggregate. PurposeIDs is stored scope state; Purposes is the name
// projection the service fills for reads and is never accepted on a write.
type Guideline struct {
	ID         string
	UserID     string
	Text       string
	Scope      Scope
	PurposeIDs []string
	Purposes   []PurposeRef
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ScopePatch is a whole scope as one value. A scope is a kind plus a set, so the two are
// only meaningful together: an edit either leaves the scope entirely alone or replaces it.
type ScopePatch struct {
	Scope      Scope
	PurposeIDs []string
}

// Patch is a presence-based update: a nil field is not part of the edit. A text-only edit
// therefore cannot disturb a scope saved concurrently from elsewhere.
type Patch struct {
	Text  *string
	Scope *ScopePatch
}

func (p Patch) empty() bool { return p.Text == nil && p.Scope == nil }
