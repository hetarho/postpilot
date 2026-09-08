package template

import (
	"fmt"
	"strconv"
	"strings"
)

// NodeKind is one of the grammar's six constructs (spec/legacy/tech/post-template-grammar.md §2).
type NodeKind string

const (
	NodeLiteral NodeKind = "literal"
	NodeWrite   NodeKind = "write"
	NodeSlot    NodeKind = "slot"
	NodeNote    NodeKind = "note"
	NodeRepeat  NodeKind = "repeat"
	// NodeAsk is a position whose facts the POST's author supplies rather than the model
	// inventing them (TEMPLATE-43). Its Label is the title the write screen shows over the
	// field; a blank element is the verbatim flavor and a text-holding one is the write
	// flavor. Nothing here resolves it — that happens at the freeze (TEMPLATE-45).
	NodeAsk NodeKind = "ask"
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
	Text     string   // write · note · ask (empty on an ask means the verbatim flavor)
	SlotKind SlotKind // slot
	Label    string   // slot · ask
	// Count is how many photos a photo position holds side by side (TEMPLATE-38). It is 1
	// when the attribute is absent and 0 on every node that is not a photo slot, so a
	// non-zero Count always means "this position binds this many photos".
	Count    int
	Each     string // repeat
	Children []Node // repeat
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
	// ReasonInvalidCount is a photo position's `count` that is not an integer in
	// 1 … PhotoRowMax. It is its own reason rather than malformed_tag because the attribute
	// parsed fine — it is the VALUE the author has to go fix.
	ReasonInvalidCount = "invalid_count"
	// The three ways an `ask` can be wrong (TEMPLATE-20). ReasonAskInRepeat is its own
	// reason rather than unknown_tag because the tag is real and it is the PLACE that is
	// wrong: how many fields a template asks for must not depend on how many photos this
	// post happens to carry.
	ReasonDuplicateAskLabel = "duplicate_ask_label"
	ReasonAskInRepeat       = "ask_in_repeat"
	ReasonTooManyAsks       = "too_many_asks"
)

// ParseOptions carries what the grammar cannot know by itself. The photo-row ceiling is
// configuration (TEMPLATE_PHOTO_ROW_MAX), and a parser that read it from the environment
// would make the shared fixture depend on the environment it runs in.
type ParseOptions struct {
	PhotoRowMax int
	// AskMaxPerBody bounds how many data fields one body may declare
	// (TEMPLATE_ASK_MAX_PER_BODY). It is configuration for the same reason PhotoRowMax is,
	// and the shared fixture declares its own so a case means one thing on both sides.
	AskMaxPerBody int
}

var tagNames = map[string]NodeKind{
	"write":  NodeWrite,
	"slot":   NodeSlot,
	"note":   NodeNote,
	"repeat": NodeRepeat,
	"ask":    NodeAsk,
}

// Parse turns a body into an ordered node list. A body that does not parse cannot be saved:
// there is no lenient fallback, because a template that half-parses would silently drop the
// structure the author asked for.
func Parse(body string, opts ParseOptions) ([]Node, error) {
	nodes, end, err := parseNodes(body, 0, false, opts)
	if err != nil {
		return nil, err
	}
	if end != len(body) {
		// parseNodes only stops early on a closing tag, and at the top level there is
		// nothing it could be closing.
		return nil, &ParseError{Line: lineAt(body, end), Reason: ReasonUnexpectedClose}
	}
	if err := checkAsks(nodes, opts.AskMaxPerBody); err != nil {
		return nil, err
	}
	return nodes, nil
}

// checkAsks enforces the two body-WIDE rules a single tag cannot see: labels are unique and
// there are at most AskMaxPerBody of them (TEMPLATE-43).
//
// One pass in body order, duplicate before count on the same node: a duplicate names the
// exact thing to go fix, while the count only says there is one field too many. Both parsers
// walk it identically, which is what keeps the shared fixture meaningful.
func checkAsks(nodes []Node, maxPerBody int) error {
	seen := map[string]bool{}
	var walk func([]Node) error
	walk = func(list []Node) error {
		for _, node := range list {
			if node.Kind == NodeRepeat {
				if err := walk(node.Children); err != nil {
					return err
				}
				continue
			}
			if node.Kind != NodeAsk {
				continue
			}
			label := Decode(node.Label)
			if seen[label] {
				return &ParseError{Line: node.Line, Reason: ReasonDuplicateAskLabel}
			}
			seen[label] = true
			if len(seen) > maxPerBody {
				return &ParseError{Line: node.Line, Reason: ReasonTooManyAsks}
			}
		}
		return nil
	}
	return walk(nodes)
}

// Asks returns the body's data fields in body order. It is how the save path reaches their
// labels without re-walking the tree itself.
func Asks(nodes []Node) []Node {
	out := make([]Node, 0, 4)
	for _, node := range nodes {
		if node.Kind == NodeAsk {
			out = append(out, node)
		}
	}
	return out
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
func parseNodes(body string, from int, inRepeat bool, opts ParseOptions) ([]Node, int, error) {
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
		node, after, err := parseTag(body, at, name, inRepeat, opts)
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
func parseTag(body string, at int, name string, inRepeat bool, opts ParseOptions) (Node, int, error) {
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
		count, err := slotCount(attrs, kind, line, opts.PhotoRowMax)
		if err != nil {
			return Node{}, 0, err
		}
		return Node{
			Kind: NodeSlot, Source: body[at:afterOpen], Line: line,
			SlotKind: kind, Label: attrs["label"], Count: count,
		}, afterOpen, nil

	case NodeAsk:
		// The PLACE is checked before the attributes: a field inside a repeat is wrong
		// wherever its label is.
		if inRepeat {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonAskInRepeat}
		}
		rawLabel, ok := attrs["label"]
		if !ok || isBlank(Decode(rawLabel)) {
			return Node{}, 0, &ParseError{Line: line, Reason: ReasonMissingAttribute}
		}
		if selfClosing {
			return Node{
				Kind: NodeAsk, Source: body[at:afterOpen], Line: line, Label: rawLabel,
			}, afterOpen, nil
		}
		inner, afterClose, err := readTextBody(body, afterOpen, name, line)
		if err != nil {
			return Node{}, 0, err
		}
		// A blank element is the verbatim flavor, and it is normalized to an empty Text so
		// "no instruction" is one value rather than three whitespace variants. The Source
		// slice keeps the author's own bytes, so serialization is still exact.
		if isBlank(Decode(inner)) {
			inner = ""
		}
		return Node{
			Kind: NodeAsk, Source: body[at:afterClose], Line: line, Label: rawLabel, Text: inner,
		}, afterClose, nil

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
		children, stopped, err := parseNodes(body, afterOpen, true, opts)
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

// slotCount resolves a photo position's row size. An absent attribute is one photo, which
// is what every body written before TEMPLATE-38 means.
//
// The value must be plain ASCII digits after trimming: `+2` and `2.0` are refused rather
// than coerced, because the TypeScript parser reads the same bodies and the two languages'
// number parsers disagree about exactly those forms.
func slotCount(attrs map[string]string, kind SlotKind, line, photoRowMax int) (int, error) {
	raw, present := attrs["count"]
	if !present {
		if kind == SlotPhoto {
			return 1, nil
		}
		return 0, nil
	}
	// A count on a retired kind is not a bad number, it is an attribute that kind never
	// had — the same class of mistake as any other unparsable attribute list.
	if kind != SlotPhoto {
		return 0, &ParseError{Line: line, Reason: ReasonMalformedTag}
	}
	value := strings.TrimFunc(Decode(raw), func(r rune) bool { return blankRunes[r] })
	if value == "" || !allDigits(value) {
		return 0, &ParseError{Line: line, Reason: ReasonInvalidCount}
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 || count > photoRowMax {
		return 0, &ParseError{Line: line, Reason: ReasonInvalidCount}
	}
	return count, nil
}

func allDigits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
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
