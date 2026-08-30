// Package purpose is the 용도 context: reusable account-owned briefs saying what kind of
// text a post is and how that kind must be written.
//
// A purpose is authored text and nothing else. Nothing here is learned, inferred, or
// written by a model ([I4] stays entirely with voice), and no behavior in this package
// enqueues work or calls a provider ([I5]). Voice decides how sentences sound; a purpose
// decides genre, structure and required content.
package purpose

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound covers unknown and foreign ids alike. A purpose belonging to another
	// account must not be distinguishable from one that never existed.
	ErrNotFound = errors.New("purpose not found")
	// ErrDuplicateName is a name another purpose of the same account already holds.
	ErrDuplicateName = errors.New("a purpose with that name already exists")
	// ErrNameRequired / ErrInstructionsRequired are the empty-after-trim cases. A brief
	// with no instructions would inject a heading and no content into every prompt.
	ErrNameRequired         = errors.New("purpose name is required")
	ErrInstructionsRequired = errors.New("purpose instructions are required")
)

// FieldTooLongError names the field and both counts so the handler can build one message
// without re-deriving which limit was hit.
type FieldTooLongError struct {
	Field string
	Chars int
	Max   int
}

func (e *FieldTooLongError) Error() string {
	return fmt.Sprintf("purpose %s has %d characters; at most %d are allowed", e.Field, e.Chars, e.Max)
}

// Limits are the configured field ceilings, counted in Unicode scalar values so a Hangul
// syllable counts as one character. They come from platform/config; this context holds no
// default of its own, because a silent fallback would let a misconfigured process accept
// briefs the frontend already refused.
type Limits struct {
	NameMaxChars         int
	DescriptionMaxChars  int
	InstructionsMaxChars int
}

func (l Limits) valid() bool {
	return l.NameMaxChars > 0 && l.DescriptionMaxChars > 0 && l.InstructionsMaxChars > 0
}

// Purpose is the aggregate. PostCount is a projection of how many posts currently point
// at it — read-only, shown before a delete, and never accepted on a write.
type Purpose struct {
	ID           string
	UserID       string
	Name         string
	Description  string
	Instructions string
	PostCount    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Brief is the prompt-facing projection: the three authored strings and nothing else. It
// is what generation freezes into a job payload, so it deliberately carries no id — a
// frozen brief must stay readable after the purpose it came from is renamed or deleted.
type Brief struct {
	Name         string
	Description  string
	Instructions string
}

func (p Purpose) Brief() Brief {
	return Brief{Name: p.Name, Description: p.Description, Instructions: p.Instructions}
}

// Patch is a presence-based update: a nil field is not part of the edit. This is what lets
// two fields edited from two tabs land without either overwriting the other.
type Patch struct {
	Name         *string
	Description  *string
	Instructions *string
}

func (p Patch) empty() bool {
	return p.Name == nil && p.Description == nil && p.Instructions == nil
}
