package template

import (
	"fmt"
	"strings"
)

// Copy tokens. A slot becomes a short token the model is asked to reproduce verbatim rather
// than a label or a sentence: copying twelve characters exactly is something a model does
// reliably, while reproducing prose is not (spec/tech/post-template-grammar.md §5).
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
// Zero photos drops every repeat block whole — including its literals — because a section
// that exists to describe photos has nothing to say about none.
func Render(name string, nodes []Node, filenames []string, maxIterations int) (Rendered, error) {
	if err := checkExpansion(nodes, len(filenames), maxIterations); err != nil {
		return Rendered{}, err
	}
	var body strings.Builder
	slots := make([]Slot, 0, 4)
	renderNodes(&body, &slots, nodes, filenames)
	return Rendered{Name: name, Body: body.String(), Slots: slots}, nil
}

// checkExpansion bounds the ITERATIONS an expansion produces rather than the resulting byte
// count: iterations are what multiply, and the bound has to be comparable across templates
// whose repeat bodies differ in size.
func checkExpansion(nodes []Node, photos, maxIterations int) error {
	iterations := 0
	for _, node := range nodes {
		if node.Kind == NodeRepeat {
			iterations += photos
		}
	}
	if iterations > maxIterations {
		return fmt.Errorf("%w: %d iterations exceed %d", ErrExpansionTooLarge, iterations, maxIterations)
	}
	return nil
}

func renderNodes(out *strings.Builder, slots *[]Slot, nodes []Node, filenames []string) {
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
			// A photo slot outside a repeat has no iteration to bind it, so it takes the
			// first attachment; with no attachments at all it renders nothing rather than
			// naming a file the post does not have.
			renderSlot(out, slots, node, firstFilename(filenames))
		case NodeRepeat:
			for _, filename := range filenames {
				renderRepeatIteration(out, slots, node.Children, filename)
			}
		}
	}
}

func renderRepeatIteration(out *strings.Builder, slots *[]Slot, children []Node, filename string) {
	for _, child := range children {
		switch child.Kind {
		case NodeSlot:
			renderSlot(out, slots, child, filename)
		case NodeLiteral:
			out.WriteString(Decode(child.Text))
		case NodeWrite:
			out.WriteString("<write>")
			out.WriteString(Decode(child.Text))
			out.WriteString("</write>")
		case NodeNote:
			out.WriteString("<note>")
			out.WriteString(Decode(child.Text))
			out.WriteString("</note>")
		}
	}
}

func renderSlot(out *strings.Builder, slots *[]Slot, node Node, filename string) {
	if node.SlotKind == SlotPhoto {
		if filename == "" {
			return
		}
		out.WriteString(PhotoToken(filename))
		return
	}
	*slots = append(*slots, Slot{Kind: node.SlotKind, Label: Decode(node.Label)})
	out.WriteString(SlotToken(len(*slots)))
}

func firstFilename(filenames []string) string {
	if len(filenames) == 0 {
		return ""
	}
	return filenames[0]
}
