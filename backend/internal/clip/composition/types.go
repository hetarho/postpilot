// Package composition defines the portable, source-preserving clip language.
// It has no storage, transport, model, or rendering dependency.
package composition

type Limits struct {
	SourceChars, Nodes, Fields, Items, Cuts, Cues               int
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
}

func (p *Problem) Error() string { return p.Reason }

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
	Span                     Span
}
type Part struct{ Literal, Field string }
type Row struct {
	Role  string
	Parts []Part
}
type Element struct {
	ID, Kind, Role, Style, Position, Align, Basis string
	StartMS, EndMS                                *int
	Parts                                         []Part
	Rows                                          []Row
	Span                                          Span
}
type Section struct {
	ID, Scope, Repeat string
	Guidance          []string
	Elements          []Element
	Span              Span
}
type Document struct {
	Source       string
	Root         *Node
	Styles       []string
	Accent, Pace string
	Fields       []Field
	Groups       []string
	Guidance     []string
	Sections     []Section
	Elements     []Element
}
type Item struct {
	ID     string
	Values map[string]string
}
type Cut struct {
	ID, SectionID, SourceID, GroupID, ItemID string
	StartMS, EndMS, TransitionMS             int
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
