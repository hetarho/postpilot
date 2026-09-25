// Package composition defines the portable, source-preserving clip language.
// It has no storage, transport, model, or rendering dependency.
package composition

import "strconv"

type Limits struct {
	SourceChars, Nodes, Fields, Items, Cuts, Cues, Stages       int
	LabelChars, PromptChars, AnswerChars, CopyChars, GuideChars int
	MaxDurationMS, AutoInsetMS                                  int
}

// Span offsets count Unicode scalar values, not bytes or UTF-16 code units.
// End is exclusive; Line is one-based. The original Source is never normalized.
type Span struct{ Start, End, Line int }
type Problem struct {
	ElementID string
	Line      int
	Reason    string
	// The refused input's owner-visible label and counts. answer_limit carries
	// Max, items_required carries Min, so a refusal can say what was asked for
	// and what was given rather than point at a line the owner never sees
	// (CLIP-102).
	Label            string
	Min, Max, Actual int
}

func (p *Problem) Error() string { return p.Reason }

// FailureParams is the ONE mapping of a problem to the stable failure contract,
// so the durable generation failure and the RPC boundary cannot describe the
// same refusal differently (CLIP-102).
func (p *Problem) FailureParams() map[string]string {
	params := map[string]string{"element_id": p.ElementID, "line": strconv.Itoa(p.Line), "reason": p.Reason}
	if p.Label != "" {
		switch p.Reason {
		case "answer_limit":
			params["label"], params["max"], params["actual"] = p.Label, strconv.Itoa(p.Max), strconv.Itoa(p.Actual)
		case "items_required":
			params["label"], params["min"], params["actual"] = p.Label, strconv.Itoa(p.Min), strconv.Itoa(p.Actual)
		}
	}
	return params
}

type Node struct {
	Name       string
	Attributes map[string]string
	Children   []*Node
	Text       string
	Span       Span
}
type Field struct {
	ID, Group, Label, Prompt string
	Required                 bool
	// Authored maximum character count (CLIP-116), 0 when undeclared. The
	// number a position actually enforces is Document.Maxima, which also folds
	// in the caps of the positions this field's answer reaches.
	Chars int
	Span  Span
}
type Group struct {
	ID, Label string
	Min, Max  int
	Span      Span
}

// Stage is one named composition stage (CLIP-141): a short name and one line of
// intent, kept in document order. It is guidance for the flow call only — it
// admits no footage and forbids none.
type Stage struct {
	Name, Intent string
	Span         Span
}
type Part struct{ Literal, Field string }
type Row struct {
	Role, Kind string
	// Authored maximum for this slot, 0 when undeclared (CLIP-116).
	Chars int
	Parts []Part
}
type Element struct {
	ID, Kind, Role, Style, Position, Align, Basis string
	StartMS, EndMS                                *int
	// Authored maximum for the element's own text, 0 when undeclared
	// (CLIP-116). An element with rows carries its maxima on the rows.
	Chars int
	Parts []Part
	Rows  []Row
	Span  Span
}
type Section struct {
	ID, Scope, Repeat string
	Guidance          []string
	Elements          []Element
	Span              Span
}

// DesignSelection is the pair of region presets a clip RENDERS in. It is the
// project's (CLIP-139) and reaches this package as an argument, never as a
// property of a document: a template declares no design at all (CLIP-14), and a
// frozen snapshot's leftover attributes are ignored (CLIP-144).
type DesignSelection struct{ Intro, Caption, Outro string }

// DefaultDesign is what a new project starts in (CLIP-111): intro A and outro B.
func DefaultDesign() DesignSelection { return DesignSelection{Intro: "a", Caption: "bold", Outro: "b"} }

// UnchosenDesign is what an empty preset id renders in wherever it is stored — a
// project, a plan, a revision or a queued generation that never named one: intro
// B and outro E, so nothing that exists changes its look (CLIP-111).
func UnchosenDesign() DesignSelection {
	return DesignSelection{Intro: "b", Caption: "bold", Outro: "e"}
}

// Entry is one position in the body's ordered outline (CLIP-112): the order an
// entry stands in is the only position it declares. Kind is "stage" or "text"
// and Index points into the document's own Stages or Elements.
type Entry struct {
	Kind  string
	Index int
}

type Document struct {
	Source       string
	Root         *Node
	Accent, Pace string
	Fields       []Field
	Groups       []Group
	Guidance     []string
	Stages       []Stage
	Sections     []Section
	Elements     []Element
	// The root's stages and visible entries in document order (CLIP-112). The
	// per-kind slices above stay beside it so a reader that needs only one kind
	// does not walk the outline it has no use for.
	Outline []Entry
	// Each field's effective maximum, keyed the way fieldKey keys a field
	// (CLIP-117): the smallest of its authored maximum, the cap of every
	// position its value reaches, and the grammar's AnswerChars. Computed once
	// here so the input control, the admission check and the writer read one
	// number rather than three derivations of it.
	Maxima map[string]int
	// Each group's effective minimum, keyed by group ID (CLIP-119): the
	// declared min, else one when any field of that group is required, else
	// zero. Computed here for the same reason Maxima is — the owner's item
	// controls and the admission check read one number, not two rules.
	Minima map[string]int
}
type Item struct {
	ID     string
	Values map[string]string
}
type Cut struct {
	ID, SectionID, SourceID, GroupID, ItemID string
	StartMS, EndMS, TransitionMS             int
	// The ONE fixed rate this cut plays at, as permille (CLIP-98). Zero is a
	// binding written before rates existed and means 1x; the clip domain
	// normalizes it on decode, so nothing downstream reads a bare zero.
	PlaybackRatePermille int
}

// Rate is the cut's fixed playback rate, reading a pre-rate binding as 1x.
func (c Cut) Rate() int {
	if c.PlaybackRatePermille == 0 {
		return RateUnitPermille
	}
	return c.PlaybackRatePermille
}

type Inputs struct {
	Values map[string]string
	Items  map[string][]Item
	Cuts   []Cut
}
type Fact struct{ FieldID, GroupID, ItemID, Value string }
type ResolvedRow struct{ Role, Text string }
type ResolvedElement struct {
	InstanceID, CutID, GroupID, ItemID string
	Element                            Element
	Text                               string // Fixed output text, or AI guidance when Element.Kind == "ai".
	Rows                               []ResolvedRow
	Facts                              []Fact
	StartMS, EndMS                     int
	AuthoredTiming                     bool
}
type Timeline struct {
	DurationMS int
	Elements   []ResolvedElement
}

// RowKind also preserves the meaning of frozen rows saved before row-level kinds.
func RowKind(element Element, row Row) string {
	if row.Kind != "" {
		return row.Kind
	}
	return element.Kind
}
