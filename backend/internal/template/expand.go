package template

import (
	"strconv"
	"strings"
)

// The marked place a photo position renders as — bare for one photo, carrying the suggested group
// size above one (TMPL-21, TMPL-38) — and the marked part a photo repeat renders as. They are code,
// not author text, so the write legend can name them; the repeat marker carries no attribute for
// the same reason.
const (
	photoPlaceSingle = "{{사진 자리}}"
	photoPlacePrefix = "{{사진 자리 · "
	photoPlaceSuffix = "장 묶음}}"
	repeatOpen       = "<repeat>"
	repeatClose      = "</repeat>"
)

// The words a stored place/link position renders as when its label is empty (TMPL-37).
const (
	placeFallback = "지도"
	linkFallback  = "링크"
)

// factOpenPrefix … factClose fence one data field's value as DATA (TMPL-46). It is a
// sibling tag rather than an attribute on `<write>` or a line inside it: the legend has to be
// able to say "never output this tag", and an attribute would put user text where the model
// reads structure.
const (
	factOpenPrefix = "<facts label=\""
	factOpenSuffix = "\">"
	factClose      = "</facts>"
)

// PhotoPlace is the marked place a photo position renders as: `{{사진 자리}}` for a count of one,
// and `{{사진 자리 · n장 묶음}}` where the author suggests a group of n (TMPL-38). A suggestion,
// not a binding: the writer decides whether and how to group (GEN-77).
func PhotoPlace(count int) string {
	if count <= 1 {
		return photoPlaceSingle
	}
	return photoPlacePrefix + strconv.Itoa(count) + photoPlaceSuffix
}

// Render resolves a parsed template for one post and renders it into the text the write and
// revise prompts carry.
//
// It runs here rather than in the prompt builder so it can be FROZEN: the caller resolves once
// at enqueue, and an answer edited after the start can no longer change what the model was
// asked for.
//
// A photo place binds no photo (TMPL-21): it renders as a marked place with its row size, and
// the writer chooses which attached photos stand there. A `<repeat each="photo">` renders once,
// marked as a part the writer repeats per photo group. A post with no photo drops every repeat
// whole, literals included, and its photo places render nothing: a section that exists to
// describe photos has nothing to say about none. Nothing expands per photo, so no expansion
// bound exists.
func Render(name string, nodes []Node, hasPhotos bool, answers []Answer) Rendered {
	var body strings.Builder
	state := &renderState{
		hasPhotos: hasPhotos,
		facts:     make([]Fact, 0, 4),
		answers:   answersByLabel(answers),
	}
	renderNodes(&body, state, resolveAsks(nodes, answers))
	return Rendered{Name: name, Body: body.String(), Facts: state.facts}
}

// RenderTemplate renders a template's two areas for one post (TMPL-50). The body renders
// exactly as Render renders it. The title area renders with the same answers; it holds no photo
// position and no repeat. A title left blank once its asks drop is no title form at all, so it
// renders as "". The facts read in document order, the title's first (TMPL-55).
func RenderTemplate(name string, title, body []Node, hasPhotos bool, answers []Answer) Rendered {
	rendered := Render(name, body, hasPhotos, answers)
	heading := Render(name, title, false, answers)
	if !isBlank(heading.Body) {
		rendered.TitleArea = heading.Body
	}
	rendered.Facts = append(heading.Facts, rendered.Facts...)
	return rendered
}

// resolveAsks replaces every data field with what the post actually answered, and REMOVES the
// node of a field that is off, blank or unanswered.
//
// Removing the node rather than emptying it is the whole guarantee (TMPL-45): the frozen
// body then neither names the field nor leaves a gap, so a template with the field off
// produces a byte-identical prompt to one that never carried it. The value is carried on the
// node itself — a resolved ask keeps its Label and holds the ANSWER in Text — so rendering
// stays a single pass with no second lookup.
func resolveAsks(nodes []Node, answers []Answer) []Node {
	byLabel := answersByLabel(answers)
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Kind == NodeAsk && !byLabel[Decode(node.Label)].usable() {
			continue
		}
		out = append(out, node)
	}
	return out
}

// answersByLabel keys the post's answers the way the body names them. A label with no answer
// reads as the zero Answer, which is not usable — the same case as off or blank.
func answersByLabel(answers []Answer) map[string]Answer {
	byLabel := make(map[string]Answer, len(answers))
	for _, answer := range answers {
		byLabel[strings.TrimSpace(answer.Label)] = answer
	}
	return byLabel
}

// renderState is what one render carries besides its output: whether the post has a photo, the
// facts resolved so far, and the post's answers.
type renderState struct {
	hasPhotos bool
	facts     []Fact
	// answers is what the post supplied, by label. Nodes keep saying what the AUTHOR wrote;
	// the answer is looked up here, so nothing has to overload a parsed field to carry it.
	answers map[string]Answer
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
		case NodeAsk:
			// Only resolved fields reach here — resolveAsks dropped the rest. The verbatim
			// flavor IS literal text on the page, so it renders as exactly that; the write
			// flavor renders its instruction plus the value fenced as fact beside it.
			renderAsk(out, state, node)
		case NodeSlot:
			renderSlot(out, state, node)
		case NodeRepeat:
			// Once, marked, whatever the photo count: the writer repeats it per photo group.
			if state.hasPhotos {
				out.WriteString(repeatOpen)
				renderNodes(out, state, node.Children)
				out.WriteString(repeatClose)
			}
		}
	}
}

// renderAsk writes one RESOLVED data field: resolveAsks already dropped the ones with no
// usable answer, so every node reaching here has a value.
//
// The verbatim flavor IS literal text on the page, so it renders as exactly that. The write
// flavor renders the author's instruction as an ordinary `<write>` plus the value fenced as
// fact beside it — the value is authored fact and never an instruction (TMPL-46).
func renderAsk(out *strings.Builder, state *renderState, node Node) {
	label := Decode(node.Label)
	value := strings.TrimSpace(state.answers[label].Text)
	instruction := Decode(node.Text)
	if instruction == "" {
		out.WriteString(value)
		return
	}
	out.WriteString("<write>")
	out.WriteString(instruction)
	out.WriteString("</write>\n")
	out.WriteString(factOpenPrefix)
	// The RAW slice, which is already escaped: this is a quoted attribute, so a title holding
	// a quote has to keep its escape or the tag stops being readable at exactly the place the
	// legend told the model to read. The element's own text is plain, like every other text
	// the prompt carries.
	out.WriteString(node.Label)
	out.WriteString(factOpenSuffix)
	out.WriteString(value)
	out.WriteString(factClose)
	state.facts = append(state.facts, Fact{Label: label, Value: value})
}

// renderSlot writes a photo position as its marked place, or a legacy place/link position as
// its label. The place carries its row size and names no photo (TMPL-21, TMPL-40).
func renderSlot(out *strings.Builder, state *renderState, node Node) {
	// There is no place or link position (TMPL-37): a stored one is a 고정 문구 whose text is its
	// label, so no run carries a slot token, a slot block or a slot marker.
	if node.SlotKind != SlotPhoto {
		out.WriteString(legacySlotText(node))
		return
	}
	if state.hasPhotos {
		out.WriteString(PhotoPlace(node.Count))
	}
}

// legacySlotText is the literal text a stored place/link position reads as: its label, or 지도 ·
// 링크 when it has none — the same words the builder shows for it (TMPL-37).
func legacySlotText(node Node) string {
	if label := strings.TrimSpace(Decode(node.Label)); label != "" {
		return label
	}
	if node.SlotKind == SlotLink {
		return linkFallback
	}
	return placeFallback
}
