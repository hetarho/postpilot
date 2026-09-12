package composition

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type reader struct {
	source    []rune
	at, nodes int
	limits    Limits
}

func (r *reader) problem(reason string, n *Node) *Problem {
	line, id := 1, "clip"
	for _, c := range r.source[:r.at] {
		if c == '\n' {
			line++
		}
	}
	if n != nil {
		line = n.Span.Line
		if v := n.Attributes["id"]; v != "" {
			id = v
		} else {
			id = n.Name
		}
	}
	return &Problem{id, line, reason}
}
func white(c rune) bool { return c == ' ' || c == '\n' || c == '\r' || c == '\t' }
func (r *reader) skip() {
	for r.at < len(r.source) && white(r.source[r.at]) {
		r.at++
	}
}
func (r *reader) has(s string) bool {
	// Grammar delimiters are ASCII. Avoid allocating the remaining source on
	// every lookahead, especially for long malformed attributes.
	if len(s) > len(r.source)-r.at {
		return false
	}
	for i := range len(s) {
		if r.source[r.at+i] != rune(s[i]) {
			return false
		}
	}
	return true
}
func nameChar(c rune, first bool) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || !first && (c >= '0' && c <= '9' || c == '-')
}
func (r *reader) name() string {
	start := r.at
	for r.at < len(r.source) && nameChar(r.source[r.at], r.at == start) {
		r.at++
	}
	return string(r.source[start:r.at])
}
func xmlRune(c rune) bool {
	return c == 9 || c == 10 || c == 13 || c >= 32 && c <= 0xd7ff || c >= 0xe000 && c <= 0xfffd || c >= 0x10000 && c <= 0x10ffff
}
func decode(s string) (string, bool) {
	var b strings.Builder
	for len(s) > 0 {
		if s[0] != '&' {
			c, n := utf8.DecodeRuneInString(s)
			if !xmlRune(c) {
				return "", false
			}
			b.WriteRune(c)
			s = s[n:]
			continue
		}
		i := strings.IndexByte(s, ';')
		if i < 0 {
			return "", false
		}
		e := s[1:i]
		s = s[i+1:]
		switch e {
		case "amp":
			b.WriteByte('&')
		case "lt":
			b.WriteByte('<')
		case "gt":
			b.WriteByte('>')
		case "quot":
			b.WriteByte('"')
		case "apos":
			b.WriteByte('\'')
		default:
			base, digits := 10, ""
			if strings.HasPrefix(e, "#x") {
				base, digits = 16, e[2:]
			} else if strings.HasPrefix(e, "#") {
				digits = e[1:]
			}
			if digits == "" {
				return "", false
			}
			for _, c := range digits {
				if !(c >= '0' && c <= '9' || base == 16 && (c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F')) {
					return "", false
				}
			}
			v, err := strconv.ParseInt(digits, base, 32)
			if err != nil || !xmlRune(rune(v)) {
				return "", false
			}
			b.WriteRune(rune(v))
		}
	}
	return b.String(), true
}
func (r *reader) node() (*Node, *Problem) {
	start := r.at
	line := 1
	for _, c := range r.source[:start] {
		if c == '\n' {
			line++
		}
	}
	if !r.has("<") {
		return nil, r.problem("syntax", nil)
	}
	r.at++
	name := r.name()
	if name == "" {
		return nil, r.problem("unsafe_construct", nil)
	}
	n := &Node{Name: name, Attributes: map[string]string{}, Span: Span{Start: start, Line: line}}
	r.nodes++
	if r.nodes > r.limits.Nodes {
		return nil, r.problem("node_limit", n)
	}
	for {
		before := r.at
		r.skip()
		spaced := r.at > before
		if r.has("/>") {
			r.at += 2
			n.Span.End = r.at
			return n, nil
		}
		if r.has(">") {
			r.at++
			break
		}
		if !spaced {
			return nil, r.problem("syntax", n)
		}
		key := r.name()
		if key == "" {
			return nil, r.problem("syntax", n)
		}
		if _, ok := n.Attributes[key]; ok {
			return nil, r.problem("duplicate_attribute", n)
		}
		r.skip()
		if !r.has("=") {
			return nil, r.problem("syntax", n)
		}
		r.at++
		r.skip()
		if r.at >= len(r.source) || (r.source[r.at] != '\'' && r.source[r.at] != '"') {
			return nil, r.problem("syntax", n)
		}
		quote := r.source[r.at]
		r.at++
		a := r.at
		for r.at < len(r.source) && r.source[r.at] != quote {
			if r.source[r.at] == '<' {
				return nil, r.problem("syntax", n)
			}
			r.at++
		}
		if r.at == len(r.source) {
			return nil, r.problem("syntax", n)
		}
		v, ok := decode(string(r.source[a:r.at]))
		if !ok {
			return nil, r.problem("invalid_entity", n)
		}
		n.Attributes[key] = v
		r.at++
	}
	for r.at < len(r.source) {
		if r.has("</") {
			r.at += 2
			end := r.name()
			r.skip()
			if end != name || !r.has(">") {
				return nil, r.problem("syntax", n)
			}
			r.at++
			n.Span.End = r.at
			return n, nil
		}
		if r.has("<") {
			child, e := r.node()
			if e != nil {
				return nil, e
			}
			n.Children = append(n.Children, child)
			continue
		}
		a := r.at
		for r.at < len(r.source) && r.source[r.at] != '<' {
			r.at++
		}
		raw := string(r.source[a:r.at])
		v, ok := decode(raw)
		if !ok || strings.Contains(raw, "]]>") {
			return nil, r.problem("invalid_entity", n)
		}
		n.Children = append(n.Children, &Node{Name: "#text", Text: v, Span: Span{Start: a, End: r.at, Line: lineAt(r.source, a)}})
	}
	return nil, r.problem("syntax", n)
}
func lineAt(s []rune, end int) int {
	n := 1
	for _, c := range s[:end] {
		if c == '\n' {
			n++
		}
	}
	return n
}
func escape(s string, attr bool) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	if attr {
		s = strings.ReplaceAll(s, "\"", "&quot;")
	}
	return s
}

// SerializeNode is used only for an edited subtree; untouched source stays exact.
func SerializeNode(n *Node) string {
	if n.Name == "#text" {
		return escape(n.Text, false)
	}
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(n.Name)
	keys := make([]string, 0, len(n.Attributes))
	for k := range n.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteByte(' ')
		b.WriteString(k)
		b.WriteString("=\"")
		b.WriteString(escape(n.Attributes[k], true))
		b.WriteByte('"')
	}
	if len(n.Children) == 0 {
		b.WriteString("/>")
		return b.String()
	}
	b.WriteByte('>')
	for _, c := range n.Children {
		b.WriteString(SerializeNode(c))
	}
	b.WriteString("</" + n.Name + ">")
	return b.String()
}
func ReplaceNode(d *Document, id string, replacement *Node, limits Limits) (*Document, *Problem) {
	var target *Node
	var walk func(*Node, string)
	walk = func(n *Node, group string) {
		key := n.Attributes["id"]
		if n.Name == "field" && group != "" {
			key = group + "." + key
		}
		if key == id {
			target = n
		}
		if n.Name == "group" {
			group = n.Attributes["id"]
		}
		for _, c := range n.Children {
			walk(c, group)
		}
	}
	walk(d.Root, "")
	if target == nil {
		return nil, &Problem{id, 1, "unknown_element"}
	}
	return ReplaceSpan(d, target.Span, replacement, limits)
}

// ReplaceSpan also addresses unlabelled guides/rows, using a span from this exact
// document. It refuses stale/arbitrary ranges rather than slicing through nodes.
func ReplaceSpan(d *Document, span Span, replacement *Node, limits Limits) (*Document, *Problem) {
	found := false
	var walk func(*Node)
	walk = func(n *Node) {
		if n.Span == span {
			found = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(d.Root)
	if !found {
		return nil, &Problem{"clip", span.Line, "unknown_element"}
	}
	s := []rune(d.Source)
	return Parse(string(s[:span.Start])+SerializeNode(replacement)+string(s[span.End:]), limits)
}
