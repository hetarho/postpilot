// Package template is the 템플릿 context: reusable account-owned documents that decide the
// SHAPE of a post — its literal text, the positions it reserves for content the app cannot
// invent, what repeats per photo, and where prose gets written.
//
// A template is authored text and nothing else. Nothing here is learned, inferred, or
// written by a model ([I4] stays entirely with voice), and no behavior in this package
// enqueues work or calls a provider ([I5]). Voice decides how sentences sound and a
// guideline decides what to avoid; a template decides the skeleton.
//
// The grammar this package parses is specified in spec/legacy/tech/post-template-grammar.md. Its
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

// NumberOutOfRangeError is a generation number a template may not hold. The bounds are the
// POST option's own (TEMPLATE-6): a template's number only ever lands in a post's option, so
// one the post would refuse must not be storable here either.
//
// Max 0 means the product sets no ceiling - the target length is any positive number
// (POST-20), and inventing one here would be a rule the post itself does not have.
type NumberOutOfRangeError struct {
	Field string
	Value int
	Min   int
	Max   int
}

func (e *NumberOutOfRangeError) Error() string {
	if e.Max <= 0 {
		return fmt.Sprintf("template %s is %d; it must be at least %d", e.Field, e.Value, e.Min)
	}
	return fmt.Sprintf("template %s is %d; it must be between %d and %d", e.Field, e.Value, e.Min, e.Max)
}

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
	// PhotoRowMax is the largest `count` a photo position may carry (TEMPLATE-38). It bounds
	// a row's width rather than a total: four thumbnails is what still reads on a 360 px
	// phone, and the browser mirrors the same number.
	PhotoRowMax int
	// AskLabelMaxChars bounds a data field's title and AskMaxPerBody how many one body may
	// declare (TEMPLATE-43). The title is bounded like a name rather than like prose: it is
	// a question the write screen puts over a textarea, and a form long enough to push the
	// memo off a phone costs more than the invented sentence it prevents.
	AskLabelMaxChars int
	AskMaxPerBody    int
	// TargetLengthMin and TagCountMin/Max bound the two generation numbers a template may
	// author (TEMPLATE-47). They are the POST option's bounds, passed in rather than owned
	// here: the number is a SEED for that option, and a template able to store one the post
	// refuses would make an assignment fail at a place the user never typed anything. The
	// length has a floor and no ceiling, exactly as the post's own option does.
	TargetLengthMin int
	TagCountMin     int
	TagCountMax     int
}

func (l Limits) valid() bool {
	return l.NameMaxChars > 0 && l.DescriptionMaxChars > 0 && l.BodyMaxChars > 0 &&
		l.MaxPerAccount > 0 && l.MaxRepeatExpansion > 0 && l.PhotoRowMax > 0 &&
		l.AskLabelMaxChars > 0 && l.AskMaxPerBody > 0 &&
		l.TargetLengthMin > 0 && l.TagCountMin > 0 && l.TagCountMax >= l.TagCountMin
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
	// TargetLength and TagCount are what the posts this template shapes usually want
	// (TEMPLATE-47). nil is "no opinion": assigning the template then leaves the post's own
	// option alone. Neither ever reaches a prompt - they are seeds for the post's options and
	// a run keeps freezing the post's values.
	TargetLength *int
	TagCount     *int
	PostCount    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Patch is a presence-based update: a nil field is not part of the edit. This is what lets
// two fields edited from two tabs land without either overwriting the other.
type Patch struct {
	Name        *string
	Description *string
	Body        *string
	// The two numbers break the presence rule on purpose (TEMPLATE-8): Numbers present means
	// "write both", each member nil meaning no opinion, because the template screen holds
	// both and sends both on every save. A second meaning for an absent number would only
	// give an unset one two ways to be written.
	Numbers *Numbers
}

// Numbers is the pair a save writes together.
type Numbers struct {
	TargetLength *int
	TagCount     *int
}

func (p Patch) empty() bool {
	return p.Name == nil && p.Description == nil && p.Body == nil && p.Numbers == nil
}

// Answer is what one post supplies for one data field, handed in by the caller at enqueue.
//
// Off and blank are ONE case here (TEMPLATE-45): the switch says "I have nothing for this"
// and a field left empty says the same thing, so both drop the whole position rather than
// asking the model to write a section it has no facts for.
type Answer struct {
	Label   string
	Text    string
	Enabled bool
}

// usable reports whether this answer survives to the prompt.
func (a Answer) usable() bool { return a.Enabled && !isBlank(a.Text) }

// Fact is one data field that DID survive, in body order: the title the author asked under
// and the text the post's author typed. It is frozen beside the body so the prompt builder
// can tell whether the rendered text carries any fact at all without re-parsing it.
type Fact struct {
	Label string
	Value string
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
	// Rows records what each photo position actually bound, in body order — one entry per
	// position that bound at least one photo, iterations of a repeat included. It is frozen
	// beside the body so the author's row intent survives to whatever finally carries a row
	// downstream (→TEMPLATE-39); the rendered body itself stays n adjacent single-photo
	// tokens in the meantime (TEMPLATE-40).
	Rows []PhotoRow
	// Facts are the data fields this render resolved, in body order. Empty means the body
	// declared none or every one of them was off or blank — which is the same thing to
	// everything downstream (TEMPLATE-45).
	Facts []Fact
}

// PhotoRow is one photo position after binding: how many photos it asked for and the ones
// it got. A short last group makes len(Filenames) < Count, which is the row the author
// asked for as far as the photos went.
type PhotoRow struct {
	Count     int
	Filenames []string
}

// Slot is one reserved position the app cannot fill by itself. It stays honest rather than
// filled: the model is told not to write prose there, and a person fills it after export.
type Slot struct {
	Kind  SlotKind
	Label string
}
