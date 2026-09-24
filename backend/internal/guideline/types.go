// Package guideline is the 작문 지침 context: reusable account-owned rules about what a post
// must avoid or watch out for.
//
// A guideline is authored text and nothing else. Nothing here is learned, inferred, or
// written by a model ([I4] stays entirely with voice), and no behavior in this package
// enqueues work or calls a provider ([I5]). Voice decides how sentences sound and a template
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
// `templates` guideline with no links left is a real state (every template it named was
// deleted) and must stay distinguishable from a global one.
type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeTemplates Scope = "templates"
	// ScopeFields reaches the posts whose 분야 is one of the guideline's (GUIDE-5).
	ScopeFields Scope = "fields"
)

func (s Scope) Valid() bool { return s == ScopeGlobal || s == ScopeTemplates || s == ScopeFields }

var (
	// ErrNotFound covers unknown and foreign ids alike. A guideline belonging to another
	// account must not be distinguishable from one that never existed.
	ErrNotFound = errors.New("guideline not found")
	// ErrTemplateNotFound is an unknown or foreign template id in a scope. It is reported as
	// not-found for the same reason, and nothing about the request is applied.
	ErrTemplateNotFound = errors.New("scoped template not found")
	// ErrDuplicateText is a text another guideline of the same account already holds. The
	// texts are the prompt lines, so two identical ones would inject the same rule twice.
	ErrDuplicateText = errors.New("a guideline with that text already exists")
	// ErrInvalidText is the empty-after-trim case: a blank line in the prompt section.
	ErrInvalidText = errors.New("guideline text is required")
	// ErrScopeShape is a scope whose kind and template set contradict each other — `global`
	// carrying template ids, or `templates` carrying none. Silently repairing either would
	// save a scope the user did not ask for.
	ErrScopeShape = errors.New("guideline scope shape is invalid")
	// ErrFieldNotFound is a 분야 in a scope or in the preset's set that is not on the
	// product's list. Like a foreign template, nothing about the request is applied.
	ErrFieldNotFound = errors.New("guideline blog field not found")
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

// TemplateRef is a template as this context needs it: an id it can validate ownership of and a
// name it can show. Names are always a live projection through TemplateDirectory, never a
// column of this context's tables and never a SQL join (ARCHITECTURE §2.2).
type TemplateRef struct {
	ID   string
	Name string
}

// Guideline is the aggregate. TemplateIDs and Fields are stored scope state, Fields as the
// ASCII 분야 ids in id order; Templates is the name projection the service fills for reads and
// is never accepted on a write.
type Guideline struct {
	ID          string
	UserID      string
	Text        string
	Scope       Scope
	TemplateIDs []string
	Fields      []string
	Templates   []TemplateRef
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ScopePatch is a whole scope as one value. A scope is a kind plus a set, so the two are
// only meaningful together: an edit either leaves the scope entirely alone or replaces it.
type ScopePatch struct {
	Scope       Scope
	TemplateIDs []string
	Fields      []string
}

// Patch is a presence-based update: a nil field is not part of the edit. A text-only edit
// therefore cannot disturb a scope saved concurrently from elsewhere.
type Patch struct {
	Text  *string
	Scope *ScopePatch
}

func (p Patch) empty() bool { return p.Text == nil && p.Scope == nil }

// Preset is the account's state of the product's 상위 노출 단어 사용 preset: whether it is on and
// the 분야 it applies to. It is not a guideline row — its text is a product constant and never
// stored — so it spends neither the account cap nor text uniqueness (GUIDE-39). An account that
// never touched it holds the zero value, off with no 분야 (GUIDE-34).
type Preset struct {
	Enabled bool
	Fields  []string
}

// PresetPatch is a presence-based edit like Patch: a nil field is not part of the edit, so
// switching the preset keeps its 분야 and editing the 분야 keeps the switch.
type PresetPatch struct {
	Enabled *bool
	Fields  *[]string
}

// PromptTexts is what a prompt builder receives: the owner's texts that apply, in injection
// order (global, template, 분야), and the product preset's line — set when the preset is on for
// the post's 분야 and this is not a revision, "" otherwise. The line is apart so the caller that
// knows whether its phrases froze decides whether it rides (GUIDE-40), and appends it last
// (GUIDE-14, GUIDE-37).
type PromptTexts struct {
	Owner  []string
	Preset string
}
