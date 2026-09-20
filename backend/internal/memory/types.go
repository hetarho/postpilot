// Package memory is the 기억 context: short atomic facts about the author's world that no
// single post carries.
//
// A memory is authored and nothing else. Nothing here is learned, inferred, scored, ranked
// or written by a model (MEM-23): the user approves an extracted candidate or writes a fact
// by hand, and this package stores exactly what they approved. No behaviour in this package
// enqueues work or calls a provider ([I5]), and deduplication is exact after trim — there is
// no similarity, fuzzy or semantic matching anywhere in it (MEM-9).
//
// Nothing references a memory: a generation freezes the selected TEXTS into its payload at
// enqueue (MEM-19), never ids, so editing or deleting one changes nothing already in flight.
package memory

import (
	"errors"
	"fmt"
	"time"
)

// Kind is the closed five (MEM-5). It is a typed string whose zero value is invalid: an
// absent kind is a request that did not say what it was storing, and defaulting it would
// decide the retrieval half — always-candidate or tag-gated — on the user's behalf.
type Kind string

const (
	// KindPreference 취향 and KindPersona 설정 are candidates for every post regardless of
	// tags: a standing fact about the author is what lets a post leave the frame (MEM-6).
	KindPreference Kind = "preference"
	KindPersona    Kind = "persona"
	// KindPlace 장소, KindPerson 인물 and KindHistory 이력 are candidates only on tag
	// overlap: an unfiltered place or person fact staples an unrelated shop to a post.
	KindPlace   Kind = "place"
	KindPerson  Kind = "person"
	KindHistory Kind = "history"
)

// Kinds is the enum in its declared order, for callers that need to enumerate it.
var Kinds = []Kind{KindPreference, KindPersona, KindPlace, KindPerson, KindHistory}

func (k Kind) Valid() bool {
	switch k {
	case KindPreference, KindPersona, KindPlace, KindPerson, KindHistory:
		return true
	default:
		return false
	}
}

// AlwaysCandidate is MEM-6's split, asked of the kind itself so retrieval never re-derives
// it from a list that could fall out of step with this one.
func (k Kind) AlwaysCandidate() bool { return k == KindPreference || k == KindPersona }

// ParseKind refuses an unknown string rather than mapping it to a default. The zero value
// is not a kind, so a caller that forgot the field is refused too.
func ParseKind(value string) (Kind, error) {
	kind := Kind(value)
	if !kind.Valid() {
		return "", ErrInvalidKind
	}
	return kind, nil
}

var (
	// ErrNotFound covers unknown and foreign ids alike. A memory belonging to another
	// account must not be distinguishable from one that never existed.
	ErrNotFound = errors.New("memory not found")
	// ErrInvalidText is the empty-after-trim case: a memory with no fact in it.
	ErrInvalidText = errors.New("memory text is required")
	// ErrInvalidKind is an absent or unknown kind. Never defaulted (MEM-12).
	ErrInvalidKind = errors.New("memory kind is invalid")
	// ErrDuplicateText is an EDIT into a text another memory of the account already holds.
	// A create never reports it — an identical text there is a second sighting and links
	// the post instead (MEM-9) — but two rows cannot be merged behind the user's back, so
	// the edit is refused and they decide which one to keep (MEM-10).
	ErrDuplicateText = errors.New("a memory with that text already exists")
	// ErrInvalidTag is an empty-after-trim tag. The tag set is collapsed, not repaired:
	// dropping a blank silently would save a set the user did not send.
	ErrInvalidTag = errors.New("memory tag is empty")
)

// TextTooLongError carries both counts so the handler can report the limit that was hit
// without re-deriving it.
type TextTooLongError struct {
	Chars int
	Max   int
}

func (e *TextTooLongError) Error() string {
	return fmt.Sprintf("memory text has %d characters; at most %d are allowed", e.Chars, e.Max)
}

// TooManyTagsError is a tag set past the ceiling, counted AFTER duplicates collapse so a
// request is never refused for repeating one tag.
type TooManyTagsError struct {
	Count int
	Max   int
}

func (e *TooManyTagsError) Error() string {
	return fmt.Sprintf("memory carries %d tags; at most %d are allowed", e.Count, e.Max)
}

// AccountCapError refuses a create past the per-account ceiling. It names the cap because
// the message the user reads has to say the number, and because nothing is evicted to make
// room: at the cap the account keeps exactly what it chose to keep (MEM-11).
type AccountCapError struct{ Max int }

func (e *AccountCapError) Error() string {
	return fmt.Sprintf("the account already holds the maximum of %d memories", e.Max)
}

// Limits are the configured ceilings, counted in Unicode scalar values so a Hangul syllable
// counts as one character. They come from platform/config; this context holds no default of
// its own, because a silent fallback would let a misconfigured process accept facts the
// frontend already refused.
type Limits struct {
	TextMaxChars  int
	TagsMax       int
	MaxPerAccount int
}

func (l Limits) valid() bool {
	return l.TextMaxChars > 0 && l.TagsMax > 0 && l.MaxPerAccount > 0
}

// Memory is the aggregate: one atomic fact, one kind, its tags, the posts it was approved
// from and its timestamps. Nothing else is authored and nothing is scored (MEM-4).
type Memory struct {
	ID     string
	UserID string
	Text   string
	Kind   Kind
	Tags   []string
	// SourcePostSlugs is every post this fact was approved from, oldest first. Empty for a
	// memory written by hand.
	SourcePostSlugs []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// LastSeenAt advances when the same text is approved again. Separate from UpdatedAt,
	// which belongs to an authored edit: retrieval breaks ties by use, not by edit.
	LastSeenAt time.Time
}

// Patch is a presence-based update: a nil field is not part of the edit. A text-only edit
// therefore cannot disturb tags saved concurrently from elsewhere.
type Patch struct {
	Text *string
	Kind *Kind
	Tags *[]string
}

func (p Patch) empty() bool { return p.Text == nil && p.Kind == nil && p.Tags == nil }
