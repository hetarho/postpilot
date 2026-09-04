// Package template is the 템플릿 context: reusable account-owned documents that decide the
// SHAPE of a post — its literal text, the positions it reserves for content the app cannot
// invent, what repeats per photo, and where prose gets written.
//
// A template is authored text and nothing else. Nothing here is learned, inferred, or
// written by a model ([I4] stays entirely with voice), and no behavior in this package
// enqueues work or calls a provider ([I5]). Voice decides how sentences sound and a
// guideline decides what to avoid; a template decides the skeleton.
//
// The grammar this package parses is specified in spec/tech/post-template-grammar.md. Its
// Go parser and the frontend's TypeScript parser are tested against one shared fixture file
// under testdata/grammar, which is what keeps two implementations of one grammar honest.
package template

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound covers unknown and foreign ids alike. A template belonging to another
	// account must not be distinguishable from one that never existed.
	ErrNotFound = errors.New("template not found")
	// ErrDuplicateName is a name another template of the same account already holds.
	ErrDuplicateName = errors.New("a template with that name already exists")
	// ErrNameRequired / ErrBodyRequired are the empty-after-trim cases. A template with no
	// body would inject a heading and no shape into every prompt.
	ErrNameRequired = errors.New("template name is required")
	ErrBodyRequired = errors.New("template body is required")
	// ErrTooMany is the per-account cap. It is a storage guard rather than a prompt guard —
	// only the one assigned template ever reaches a prompt.
	ErrTooMany = errors.New("this account already holds the maximum number of templates")
	// ErrExpansionTooLarge is a repeat that would grow past the configured bound for the
	// post's photo count. It is refused at start rather than sent, so an unbounded prompt
	// never reaches a provider.
	ErrExpansionTooLarge = errors.New("expanding this template for that many photos exceeds the bound")
)

// FieldTooLongError names the field and both counts so the handler can build one message
// without re-deriving which limit was hit.
type FieldTooLongError struct {
	Field string
	Chars int
	Max   int
}

func (e *FieldTooLongError) Error() string {
	return fmt.Sprintf("template %s has %d characters; at most %d are allowed", e.Field, e.Chars, e.Max)
}

// Limits are the configured ceilings, counted in Unicode scalar values so a Hangul syllable
// counts as one character. They come from platform/config; this context holds no default of
// its own, because a silent fallback would let a misconfigured process accept templates the
// frontend already refused.
type Limits struct {
	NameMaxChars        int
	DescriptionMaxChars int
	BodyMaxChars        int
	MaxPerAccount       int
	MaxRepeatExpansion  int
}

func (l Limits) valid() bool {
	return l.NameMaxChars > 0 && l.DescriptionMaxChars > 0 && l.BodyMaxChars > 0 &&
		l.MaxPerAccount > 0 && l.MaxRepeatExpansion > 0
}

// Template is the aggregate. Body is the single source of truth for the template's shape:
// the parser keeps every literal's raw slice, so Serialize(Parse(Body)) == Body and the
// builder can round-trip a hand-written template without reformatting it.
//
// PostCount is a projection of how many posts currently point at it — read-only, shown
// before a delete, and never accepted on a write.
type Template struct {
	ID          string
	UserID      string
	Name        string
	Description string
	Body        string
	PostCount   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Patch is a presence-based update: a nil field is not part of the edit. This is what lets
// two fields edited from two tabs land without either overwriting the other.
type Patch struct {
	Name        *string
	Description *string
	Body        *string
}

func (p Patch) empty() bool {
	return p.Name == nil && p.Description == nil && p.Body == nil
}

// Rendered is the prompt-facing projection: one template expanded for one post's photos and
// rendered into the text the write and revise prompts carry, plus the slots it declared in
// document order.
//
// It deliberately carries no id. A frozen render must stay readable after the template it
// came from is renamed or deleted, which is also why Name is a copy rather than a lookup.
type Rendered struct {
	Name string
	Body string
	// Slots holds the UNFILLED kinds (place · link) in the order they appear, so index+1 is
	// the number the body's {{slot:n}} tokens carry and the post-processing pass can match
	// them back. Photo slots are absent by design: they render as their bound filename and
	// resolve through the attachment filter that already exists.
	Slots []Slot
}

// Slot is one reserved position the app cannot fill by itself. It stays honest rather than
// filled: the model is told not to write prose there, and a person fills it after export.
type Slot struct {
	Kind  SlotKind
	Label string
}
