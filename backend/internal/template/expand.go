package template

import (
	"fmt"
	"strings"
)

// Copy tokens. A slot becomes a short token the model is asked to reproduce verbatim rather
// than a label or a sentence: copying twelve characters exactly is something a model does
// reliably, while reproducing prose is not (spec/legacy/tech/post-template-grammar.md §5).
const (
	slotTokenPrefix  = "{{slot:"
	photoTokenPrefix = "{{photo:"
	tokenSuffix      = "}}"
)

// SlotToken is the token slot n (1-based) is rendered as, and the exact string the
// post-processing pass looks for in the model's output.
func SlotToken(n int) string { return fmt.Sprintf("%s%d%s", slotTokenPrefix, n, tokenSuffix) }

// PhotoToken names the attachment a photo slot was bound to during expansion.
func PhotoToken(filename string) string { return photoTokenPrefix + filename + tokenSuffix }

// Render expands a parsed template for one post's photos and renders it into the text the
// write and revise prompts carry.
//
// Expansion happens here rather than in the prompt builder so it can be FROZEN: the caller
// resolves once at enqueue, and a photo attached after the start can no longer change what
// the model was asked for.
//
// The photos are consumed ONCE, in attachment order, across every photo position in body
// order (TEMPLATE-21): each position takes the next `count` unbound photos, a repeat runs
// as many iterations as its positions need to exhaust what is left, and a position with
// nothing left renders nothing. Zero photos drops every repeat block whole — including its
// literals — because a section that exists to describe photos has nothing to say about none.
func Render(name string, nodes []Node, filenames []string, maxIterations int) (Rendered, error) {
	if err := checkExpansion(nodes, len(filenames), maxIterations); err != nil {
		return Rendered{}, err
	}
	var body strings.Builder
	state := &renderState{remaining: filenames, slots: make([]Slot, 0, 4), rows: make([]PhotoRow, 0, 4)}
	renderNodes(&body, state, nodes)
	return Rendered{Name: name, Body: body.String(), Slots: state.slots, Rows: state.rows}, nil
}

// renderState is the one cursor over the post's photos. Binding is a single left-to-right
// pass, so the cursor — not the node tree — is what decides which photo a position gets.
type renderState struct {
	remaining []string
	slots     []Slot
	rows      []PhotoRow
}

// take returns the next min(n, remaining) filenames and advances past them.
func (s *renderState) take(n int) []string {
	if n > len(s.remaining) {
		n = len(s.remaining)
	}
	taken := s.remaining[:n]
	s.remaining = s.remaining[n:]
	return taken
}

// checkExpansion bounds the ITERATIONS an expansion produces rather than the resulting byte
// count: iterations are what multiply, and the bound has to be comparable across templates
// whose repeat bodies differ in size.
//
// It counts them exactly the way Render binds them, cursor and all, so a template can never
// be accepted here and then render more iterations than were priced.
func checkExpansion(nodes []Node, photos, maxIterations int) error {
	iterations := plannedIterations(nodes, photos)
	if iterations > maxIterations {
		return fmt.Errorf("%w: %d iterations exceed %d", ErrExpansionTooLarge, iterations, maxIterations)
	}
	return nil
}

func plannedIterations(nodes []Node, photos int) int {
	remaining := photos
	total := 0
	for _, node := range nodes {
		switch node.Kind {
		case NodeSlot:
			if node.SlotKind == SlotPhoto {
				remaining -= minInt(node.Count, remaining)
			}
		case NodeRepeat:
			group := groupSize(node.Children)
			count := iterationCount(group, remaining)
			total += count
			if group > 0 {
				remaining -= minInt(group*count, remaining)
			}
		}
	}
	return total
}

// groupSize is how many photos ONE iteration of a repeat asks for: the sum of its photo
// positions' counts. A repeat with no photo position asks for none.
func groupSize(children []Node) int {
	total := 0
	for _, child := range children {
		if child.Kind == NodeSlot && child.SlotKind == SlotPhoto {
			total += child.Count
		}
	}
	return total
}

// iterationCount is the ceiling of remaining ÷ group, with the two edges the grammar
// decided: no photos left drops the block whole, and a repeat that asks for no photos runs
// exactly once (it still has literals and write instructions to contribute).
func iterationCount(group, remaining int) int {
	if remaining == 0 {
		return 0
	}
	if group == 0 {
		return 1
	}
	return (remaining + group - 1) / group
}

func renderNodes(out *strings.Builder, state *renderState, nodes []Node) {
	for _, node := range nodes {
		switch node.Kind {
		case NodeLiteral:
			out.WriteString(Decode(node.Text))
		case NodeWrite:
			out.WriteString("<write>")
			out.WriteString(Decode(node.Text))
			out.WriteString("</write>")
		case NodeNote:
			out.WriteString("<note>")
			out.WriteString(Decode(node.Text))
			out.WriteString("</note>")
		case NodeSlot:
			renderSlot(out, state, node)
		case NodeRepeat:
			// The iteration count is read ONCE, before the first iteration consumes
			// anything: recomputing it from what is left would shrink the block as it ran.
			for n := iterationCount(groupSize(node.Children), len(state.remaining)); n > 0; n-- {
				renderNodes(out, state, node.Children)
			}
		}
	}
}

// renderSlot binds a photo position and writes its tokens, or numbers a legacy place/link
// position. A position's tokens are ADJACENT — joined by a single newline — which is how the
// interim contract says "these photos stand in one row" while every one of them is still an
// ordinary single-photo IMAGE block (TEMPLATE-40).
func renderSlot(out *strings.Builder, state *renderState, node Node) {
	if node.SlotKind != SlotPhoto {
		state.slots = append(state.slots, Slot{Kind: node.SlotKind, Label: Decode(node.Label)})
		out.WriteString(SlotToken(len(state.slots)))
		return
	}
	bound := state.take(node.Count)
	if len(bound) == 0 {
		return
	}
	for i, filename := range bound {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(PhotoToken(filename))
	}
	state.rows = append(state.rows, PhotoRow{Count: node.Count, Filenames: append([]string(nil), bound...)})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
