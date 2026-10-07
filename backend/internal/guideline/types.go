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
	// ErrFieldNotFound is a 분야 in a fields scope that is not on the product's list. Like a
	// foreign template, nothing about the request is applied.
	ErrFieldNotFound = errors.New("guideline blog field not found")
	// ErrDefaultNotFound is a 기본 지침 key the product does not carry for that kind.
	ErrDefaultNotFound = errors.New("default guideline not found")
)

// DefaultState is one 기본 지침 with the account's switch (GUIDE-43).
type DefaultState struct {
	Default DefaultGuideline
	Enabled bool
}

// PromptGuidelines are the texts one run is given, in injection order (GUIDE-14): the enabled
// 기본 지침 of the kind in the product's order and the target language, then the owner's own.
type PromptGuidelines struct {
	Defaults []string
	Owner    []string
}

// TextTooLongError carries both counts so the handler can report the limit that was hit
// without re-deriving it.
type TextTooLongError struct {
	Chars int
	Max   int
}

func (e *TextTooLongError) Error() string {
	return fmt.Sprintf("guideline text has %d characters; at most %d are allowed", e.Chars, e.Max)
}

// TitleTooLongError is a title past the bound (GUIDE-46). An empty title is not an error: it is
// the defined "no title".
type TitleTooLongError struct {
	Chars int
	Max   int
}

func (e *TitleTooLongError) Error() string {
	return fmt.Sprintf("guideline title has %d characters; at most %d are allowed", e.Chars, e.Max)
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
	TitleMaxChars int
	MaxPerAccount int
}

func (l Limits) valid() bool {
	return l.TextMaxChars > 0 && l.TitleMaxChars > 0 && l.MaxPerAccount > 0
}

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
	ID     string
	UserID string
	// Kind is the writer the guideline is for, a post's or a clip's, for good (GUIDE-2). A clip
	// guideline's TemplateIDs name video templates.
	Kind Kind
	// Title is the owner's name for the rule in the list, empty for none (GUIDE-46). It is never
	// frozen or injected: only Text reaches a prompt.
	Title       string
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
	Title *string
	Text  *string
	Scope *ScopePatch
}

func (p Patch) empty() bool { return p.Title == nil && p.Text == nil && p.Scope == nil }

// TestedPublication publishes an already validated frozen setting, without model work.
type TestedPublication struct {
	UserID, TestID, WinnerID, Action, RequestKey, Fingerprint string
	Name, Scope                                               string
	ScopeIDs                                                  []string
	FrozenContent                                             []byte
	MakeDefault                                               bool
}
type TestedPublicationReceipt struct{ TargetID, RequestKey string }
