package template

import (
	"fmt"
	"strings"
)

// NodeKind is one of the grammar's five constructs (spec/legacy/tech/post-template-grammar.md §2).
type NodeKind string

const (
	NodeLiteral NodeKind = "literal"
	NodeWrite   NodeKind = "write"
	NodeSlot    NodeKind = "slot"
	NodeNote    NodeKind = "note"
	NodeRepeat  NodeKind = "repeat"
)

// SlotKind is what a reserved position holds. photo is the only kind the app can resolve by
// itself; the others stay unfilled until a person fills them.
type SlotKind string

const (
	SlotPhoto SlotKind = "photo"
	SlotPlace SlotKind = "place"
	SlotLink  SlotKind = "link"
)

// EachPhoto is the only iterator. Attached photos are the only countable material a post
// has — a per-tag or numeric repeat would be counting something the post does not carry.
const EachPhoto = "photo"

// Node is one parsed construct. Source is the node's exact source slice, and serialization
// re-emits it verbatim: that is what makes Serialize(Parse(body)) == body by construction,
// for a hand-written body as much as for a builder-produced one, with no canonical
// formatting pass that would reflow the author's own spacing.
//
// Text and Label hold RAW inner slices. Entity decoding happens when a node's text is read
// for a prompt or for the builder (Decode), never in the stored slice, so a bare & in prose
// round-trips unchanged.
type Node struct {
	Kind     NodeKind
	Source   string
	Line     int
	Text     string   // write · note
	SlotKind SlotKind // slot
	Label    string   // slot
	Each     string   // repeat
	Children []Node   // repeat
}

// ParseError names a 1-based line and one reason, both of which the editor shows on the
// offending line. Every reason is listed in the grammar spec §4.
type ParseError struct {
	Line   int
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("template body line %d: %s", e.Line, e.Reason)
}

const (
	ReasonUnknownTag       = "unknown_tag"
	ReasonUnclosedTag      = "unclosed_tag"
	ReasonUnexpectedClose  = "unexpected_close"
	ReasonMalformedTag     = "malformed_tag"
	ReasonMissingAttribute = "missing_attribute"
	ReasonUnknownSlotKind  = "unknown_slot_kind"
	ReasonUnknownEach      = "unknown_repeat_each"
	ReasonNestedRepeat     = "nested_repeat"
	ReasonEmptyWrite       = "empty_write"
	ReasonEmptyNote        = "empty_note"
)

var tagNames = map[string]NodeKind{
	"write":  NodeWrite,
	"slot":   NodeSlot,
	"note":   NodeNote,
	"repeat": NodeRepeat,
}

// Parse turns a body into an ordered node list. A body that does not parse cannot be saved:
// there is no lenient fallback, because a template that half-parses would silently drop the
// structure the author asked for.
func Parse(body string) ([]Node, error) {
	nodes, end, err := parseNodes(body, 0, false)
	if err != nil {
		return nil, err
	}
	if end != len(body) {
		// parseNodes only stops early on a closing tag, and at the top level there is
		// nothing it could be closing.
		return nil, &ParseError{Line: lineAt(body, end), Reason: ReasonUnexpectedClose}
	}
	return nodes, nil
}

// Serialize is Parse's exact inverse for anything Parse accepted.
func Serialize(nodes []Node) string {
	var out strings.Builder
	for _, node := range nodes {
		out.WriteString(node.Source)
	}
	return out.String()
}

// blankRunes is the ONE definition of "this text says nothing", shared by both parsers.
//
// It cannot be `strings.TrimSpace` on one side and JavaScript's `.trim()` on the other: the two
// disagree in both directions — Go's table has U+0085 and not U+FEFF, JavaScript's has U+FEFF
// and not U+0085 — so `<write>\uFEFF</write>` was accepted here and refused in the browser.
// The set below is the union of the two, plus the zero-width characters a paste can carry,
// written out so the TypeScript parser can hold the identical list and the shared fixtures pin
// any drift.
var blankRunes = map[rune]bool{
	0x09: true, 0x0A: true, 0x0B: true, 0x0C: true, 0x0D: true, 0x20: true,
	0x85: true, 0xA0: true, 0x1680: true,
	0x2000: true, 0x2001: true, 0x2002: true, 0x2003: true, 0x2004: true, 0x2005: true,
	0x2006: true, 0x2007: true, 0x2008: true, 0x2009: true, 0x200A: true,
	0x200B: true, 0x200C: true, 0x200D: true,
	0x2028: true, 0x2029: true, 0x202F: true, 0x205F: true, 0x3000: true, 0xFEFF: true,
}

// isBlank reports whether every rune of the value is in blankRunes.
func isBlank(value string) bool {
	for _, r := range value {
		if !blankRunes[r] {
			return false
		}
	}
	return true
}

// Decode resolves the entities the grammar recognizes. &amp; is resolved last so an
// escaped escape (&amp;lt;) decodes to the literal text &lt; rather than to <.
//
// &quot; is in the set because an attribute value is quoted: a slot label like `네이버 "지도"`
// is ordinary free text a person types, and without an escape for it the builder would emit a
// body its own parser refuses.
func Decode(raw string) string {
	out := strings.ReplaceAll(raw, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", "\"")
	return strings.ReplaceAll(out, "&amp;", "&")
}

// parseNodes reads nodes until the body ends or an unconsumed closing tag is reached. It
// returns the offset it stopped at so a caller parsing a repeat's children can check which
// closing tag stopped it.
func parseNodes(body string, from int, inRepeat bool) ([]Node, int, error) {
	var nodes []Node
	literalStart := from
	i := from

	flushLiteral := func(upTo int) {
		if upTo > literalStart {
			nodes = append(nodes, Node{
				Kind:   NodeLiteral,
				Source: body[literalStart:upTo],
				Line:   lineAt(body, literalStart),
				Text:   body[literalStart:upTo],
			})
		}
	}

	for i < len(body) {
		next := strings.IndexByte(body[i:], '<')
		if next < 0 {
			break
		}
		at := i + next
		name, isClose, ok := tagNameAt(body, at)
		if !ok {
			// A `<` followed by whitespace, a digit or punctuation is literal prose, so
			// `3 < 5` needs no escape.
			i = at + 1
			continue
		}
		if _, known := tagNames[name]; !known {
			return nil, 0, &ParseError{Line: lineAt(body, at), Reason: ReasonUnknownTag}
		}
		if isClose {
			flushLiteral(at)
			return nodes, at, nil
		}
		flushLiteral(at)
		node, after, err := parseTag(body, at, name, inRepeat)
		if err != nil {
			return nil, 0, err
		}
		nodes = append(nodes, node)
		i = after
		literalStart = after
	}
	flushLiteral(len(body))
	return nodes, len(body), nil
}

// parseTag reads one opening tag and, for the container kinds, everything up to its close.
func parseTag(body string, at int, name string, inRepeat bool) (Node, int, error) {
	line := lineAt(body, at)
	attrs, selfClosing, afterOpen, err := parseTagHead(body, at, name)
	if err != nil {
		return Node{}, 0, err
	}

	switch tagNames[name] {
	case NodeSlot:
		if !selfClosing {
			// `<slot ...></slot>` — a slot reserves a position, it does not wrap content,
			// so a closing tag means the author expected different semantics.
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
		}
		rawKind, ok := attrs["kind"]
		if !ok {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonMissingAttribute}
		}
		kind := SlotKind(Decode(rawKind))
		if kind != SlotPhoto && kind != SlotPlace && kind != SlotLink {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonUnknownSlotKind}
		}
		return Node{
			Kind: NodeSlot, Source: body[at:afterOpen], Line: line,
			SlotKind: kind, Label: attrs["label"],
		}, afterOpen, nil

	case NodeWrite, NodeNote:
		if selfClosing {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
		}
		inner, afterClose, err := readTextBody(body, afterOpen, name, line)
		if err != nil {
			return Node{}, 0, err
		}
		if isBlank(Decode(inner)) {
			reason := ReasonEmptyWrite
			if tagNames[name] == NodeNote {
				reason = ReasonEmptyNote
			}
			return Node{}, 0, &ParseError{Line: line, Reason: reason}
		}
		return Node{
			Kind: tagNames[name], Source: body[at:afterClose], Line: line, Text: inner,
		}, afterClose, nil

	case NodeRepeat:
		if selfClosing {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
		}
		if inRepeat {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonNestedRepeat}
		}
		rawEach, ok := attrs["each"]
		if !ok {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonMissingAttribute}
		}
		each := Decode(rawEach)
		if each != EachPhoto {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonUnknownEach}
		}
		children, stopped, err := parseNodes(body, afterOpen, true)
		if err != nil {
			return Node{}, 0, err
		}
		afterClose, err := consumeClose(body, stopped, name, line)
		if err != nil {
			return Node{}, 0, err
		}
		return Node{
			Kind: NodeRepeat, Source: body[at:afterClose], Line: line,
			Each: each, Children: children,
		}, afterClose, nil
	}
	return Node{}, 0, &ParseError{Line: line, Reason: ReasonUnknownTag}
}

// parseTagHead reads the attribute list of one opening tag. A bare `key=value` is refused:
// accepting it would make `kind=photo/>` ambiguous about whether the slash is the value.
func parseTagHead(body string, at int, name string) (map[string]string, bool, int, error) {
	line := lineAt(body, at)
	attrs := map[string]string{}
	i := at + 1 + len(name)
	for i < len(body) {
		for i < len(body) && isSpace(body[i]) {
			i++
		}
		if i >= len(body) {
			break
		}
		if body[i] == '>' {
			return attrs, false, i + 1, nil
		}
		if body[i] == '/' {
			if i+1 < len(body) && body[i+1] == '>' {
				return attrs, true, i + 2, nil
			}
			return nil, false, 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
		}
		keyStart := i
		for i < len(body) && isAttrNameByte(body[i]) {
			i++
		}
		if i == keyStart || i >= len(body) || body[i] != '=' {
			return nil, false, 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
		}
		key := body[keyStart:i]
		i++ // '='
		if i >= len(body) || (body[i] != '"' && body[i] != '\'') {
			return nil, false, 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
		}
		quote := body[i]
		i++
		valueStart := i
		for i < len(body) && body[i] != quote {
			i++
		}
		if i >= len(body) {
			return nil, false, 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
		}
		attrs[key] = body[valueStart:i]
		i++ // closing quote
	}
	return nil, false, 0, &ParseError{Line: line, Reason: ReasonUnclosedTag}
}

// readTextBody reads the inner text of a write or note. A known tag inside it is a malformed
// tag rather than a nested node: neither construct wraps content, so `<write>a <write>b` is
// a mistake with no reasonable reading.
func readTextBody(body string, from int, name string, openLine int) (string, int, error) {
	i := from
	for i < len(body) {
		next := strings.IndexByte(body[i:], '<')
		if next < 0 {
			break
		}
		at := i + next
		found, isClose, ok := tagNameAt(body, at)
		if !ok {
			i = at + 1
			continue
		}
		if !isClose {
			return "", 0, &ParseError{Line: lineAt(body, at), Reason: ReasonMalformedTag}
		}
		if found != name {
			return "", 0, &ParseError{Line: lineAt(body, at), Reason: ReasonUnexpectedClose}
		}
		return body[from:at], at + len(found) + 3, nil
	}
	return "", 0, &ParseError{Line: openLine, Reason: ReasonUnclosedTag}
}

// consumeClose steps over the closing tag that stopped a child parse, and reports the
// opening tag's line when nothing closed it — the line the author has to go fix.
func consumeClose(body string, at int, name string, openLine int) (int, error) {
	if at >= len(body) {
		return 0, &ParseError{Line: openLine, Reason: ReasonUnclosedTag}
	}
	found, isClose, ok := tagNameAt(body, at)
	if !ok || !isClose {
		return 0, &ParseError{Line: openLine, Reason: ReasonUnclosedTag}
	}
	if found != name {
		return 0, &ParseError{Line: lineAt(body, at), Reason: ReasonUnexpectedClose}
	}
	return at + len(found) + 3, nil
}

// tagNameAt decides whether the `<` at `at` opens or closes one of the grammar's tags. A
// name must be followed by a delimiter, so `<writer>` reads as an unknown tag rather than as
// `write` plus stray text.
func tagNameAt(body string, at int) (string, bool, bool) {
	i := at + 1
	isClose := false
	if i < len(body) && body[i] == '/' {
		isClose = true
		i++
	}
	start := i
	for i < len(body) && isTagNameByte(body[i]) {
		i++
	}
	if i == start {
		return "", false, false
	}
	name := body[start:i]
	if i >= len(body) {
		return "", false, false
	}
	if isClose {
		if body[i] != '>' {
			return "", false, false
		}
		return name, true, true
	}
	if !isSpace(body[i]) && body[i] != '>' && body[i] != '/' {
		return "", false, false
	}
	return name, false, true
}

func lineAt(body string, offset int) int {
	if offset > len(body) {
		offset = len(body)
	}
	return 1 + strings.Count(body[:offset], "\n")
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isTagNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isAttrNameByte(c byte) bool {
	return isTagNameByte(c) || c == '_' || c == '-'
}
