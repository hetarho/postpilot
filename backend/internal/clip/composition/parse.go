package composition

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var seconds = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,3})?$`)
var styleNames = []string{"clean", "memo", "bold", "mark", "simple"}
var rowRoles = []string{"hook", "title", "mark", "body", "caption", "label", "badge"}

func issue(n *Node, reason string) *Problem {
	id := n.Attributes["id"]
	if id == "" {
		id = n.Name
	}
	return &Problem{id, n.Span.Line, reason}
}
func attrs(n *Node, allowed ...string) *Problem {
	for k := range n.Attributes {
		if !slices.Contains(allowed, k) {
			return issue(n, "unknown_attribute")
		}
	}
	return nil
}
func scalar(s string) int { return utf8.RuneCountInString(s) }
func content(n *Node) (string, *Problem) {
	var s strings.Builder
	for _, c := range n.Children {
		if c.Name != "#text" {
			return "", issue(n, "unexpected_child")
		}
		s.WriteString(c.Text)
	}
	return s.String(), nil
}
func nodes(n *Node) ([]*Node, *Problem) {
	var out []*Node
	for _, c := range n.Children {
		if c.Name == "#text" {
			if strings.TrimSpace(c.Text) != "" {
				return nil, issue(n, "unexpected_text")
			}
		} else {
			out = append(out, c)
		}
	}
	return out, nil
}
func optional(n *Node, key, fallback string) string {
	v, ok := n.Attributes[key]
	if !ok {
		return fallback
	}
	return v
}
func validLimits(l Limits) bool {
	for _, v := range []int{l.SourceChars, l.Nodes, l.Fields, l.Items, l.Cuts, l.Cues, l.LabelChars, l.PromptChars, l.AnswerChars, l.CopyChars, l.GuideChars, l.MaxDurationMS} {
		if v <= 0 {
			return false
		}
	}
	return l.AutoInsetMS >= 0
}

// Milliseconds parses decimal seconds without floating point rounding.
func Milliseconds(s string, max int) (int, bool) {
	if !seconds.MatchString(s) {
		return 0, false
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	p := strings.SplitN(s, ".", 2)
	whole, e := strconv.ParseInt(p[0], 10, 64)
	if e != nil || whole > int64(max)/1000+1 {
		return 0, false
	}
	ms := whole * 1000
	if len(p) == 2 {
		v, _ := strconv.Atoi(p[1] + strings.Repeat("0", 3-len(p[1])))
		ms += int64(v)
	}
	if ms > int64(max) {
		return 0, false
	}
	if neg {
		ms = -ms
	}
	return int(ms), true
}

func Parse(source string, limits Limits) (*Document, *Problem) {
	if !validLimits(limits) {
		return nil, &Problem{"clip", 1, "invalid_limits"}
	}
	if !utf8.ValidString(source) {
		return nil, &Problem{"clip", 1, "invalid_unicode"}
	}
	if scalar(source) > limits.SourceChars {
		return nil, &Problem{"clip", 1, "source_limit"}
	}
	for _, c := range source {
		if !xmlRune(c) {
			return nil, &Problem{"clip", 1, "invalid_unicode"}
		}
	}
	r := reader{source: []rune(source), limits: limits}
	r.skip()
	root, e := r.node()
	if e != nil {
		return nil, e
	}
	r.skip()
	if r.at != len(r.source) {
		return nil, r.problem("syntax", root)
	}
	if root.Name != "clip" {
		return nil, issue(root, "unknown_tag")
	}
	if e = attrs(root, "version", "styles", "accent", "pace"); e != nil {
		return nil, e
	}
	if root.Attributes["version"] != "1" {
		return nil, issue(root, "unknown_version")
	}
	d := &Document{Source: source, Root: root, Styles: strings.Fields(optional(root, "styles", "clean")), Accent: optional(root, "accent", ""), Pace: optional(root, "pace", "steady")}
	if len(d.Styles) == 0 {
		return nil, issue(root, "invalid_style")
	}
	seenStyle := map[string]bool{}
	for _, v := range d.Styles {
		if !slices.Contains(styleNames, v) || seenStyle[v] {
			return nil, issue(root, "invalid_style")
		}
		seenStyle[v] = true
	}
	if !slices.Contains([]string{"", "coral", "amber", "lime", "teal", "blue", "violet", "pink"}, d.Accent) {
		return nil, issue(root, "invalid_accent")
	}
	if d.Pace != "steady" && d.Pace != "rapid" {
		return nil, issue(root, "invalid_pace")
	}
	children, e := nodes(root)
	if e != nil {
		return nil, e
	}
	ids := map[string]bool{}
	claim := func(n *Node, prefix string) *Problem {
		id := n.Attributes["id"]
		if !identifier.MatchString(id) {
			return issue(n, "invalid_id")
		}
		key := prefix + id
		if ids[key] {
			return issue(n, "duplicate_id")
		}
		ids[key] = true
		return nil
	}
	readField := func(n *Node, group string) *Problem {
		if e := attrs(n, "id", "label", "required"); e != nil {
			return e
		}
		prefix := ""
		if group != "" {
			prefix = group + "."
		}
		if e := claim(n, prefix); e != nil {
			return e
		}
		prompt, e := content(n)
		if e != nil {
			return e
		}
		label := n.Attributes["label"]
		required := optional(n, "required", "false")
		if strings.TrimSpace(label) == "" || scalar(label) > limits.LabelChars || scalar(prompt) > limits.PromptChars {
			return issue(n, "field_limit")
		}
		if required != "true" && required != "false" {
			return issue(n, "invalid_required")
		}
		d.Fields = append(d.Fields, Field{n.Attributes["id"], group, label, prompt, required == "true", n.Span})
		if len(d.Fields) > limits.Fields {
			return issue(n, "field_limit")
		}
		return nil
	}
	// Declarations are read first so forward references have the same semantics.
	for _, n := range children {
		switch n.Name {
		case "field":
			if e = readField(n, ""); e != nil {
				return nil, e
			}
		case "group":
			if e = attrs(n, "id"); e != nil {
				return nil, e
			}
			if e = claim(n, ""); e != nil {
				return nil, e
			}
			group := n.Attributes["id"]
			if group == "scenes" {
				return nil, issue(n, "invalid_id")
			}
			d.Groups = append(d.Groups, group)
			fs, err := nodes(n)
			if err != nil {
				return nil, err
			}
			if len(fs) == 0 {
				return nil, issue(n, "empty_group")
			}
			for _, f := range fs {
				if f.Name != "field" {
					return nil, issue(f, "unknown_tag")
				}
				if e = readField(f, group); e != nil {
					return nil, e
				}
			}
		}
	}
	guide := func(n *Node) (string, *Problem) {
		if e := attrs(n); e != nil {
			return "", e
		}
		v, e := content(n)
		if e != nil {
			return "", e
		}
		if scalar(v) > limits.GuideChars {
			return "", issue(n, "guide_limit")
		}
		return v, nil
	}
	var section func(*Node, string) *Problem
	section = func(n *Node, repeat string) *Problem {
		if e := attrs(n, "id", "scope"); e != nil {
			return e
		}
		if e := claim(n, ""); e != nil {
			return e
		}
		scope := optional(n, "scope", "scene")
		if !slices.Contains([]string{"scene", "item", "context"}, scope) {
			return issue(n, "invalid_scope")
		}
		if repeat != "" && repeat != "scenes" && scope != "item" {
			return issue(n, "invalid_scope")
		}
		s := Section{ID: n.Attributes["id"], Scope: scope, Repeat: repeat, Span: n.Span}
		children, e := nodes(n)
		if e != nil {
			return e
		}
		for _, c := range children {
			switch c.Name {
			case "guide":
				v, e := guide(c)
				if e != nil {
					return e
				}
				s.Guidance = append(s.Guidance, v)
			case "text":
				if e := claim(c, ""); e != nil {
					return e
				}
				v, e := readElement(c, d, scope, repeat, true, limits)
				if e != nil {
					return e
				}
				s.Elements = append(s.Elements, v)
			default:
				return issue(c, "unknown_tag")
			}
		}
		d.Sections = append(d.Sections, s)
		return nil
	}
	for _, n := range children {
		switch n.Name {
		case "field", "group":
		case "guide":
			v, e := guide(n)
			if e != nil {
				return nil, e
			}
			d.Guidance = append(d.Guidance, v)
		case "text":
			if e = claim(n, ""); e != nil {
				return nil, e
			}
			v, e := readElement(n, d, "context", "", false, limits)
			if e != nil {
				return nil, e
			}
			d.Elements = append(d.Elements, v)
		case "scene":
			if e = section(n, ""); e != nil {
				return nil, e
			}
		case "repeat":
			if e = attrs(n, "for"); e != nil {
				return nil, e
			}
			over := n.Attributes["for"]
			if over != "scenes" && !slices.Contains(d.Groups, over) {
				return nil, issue(n, "unknown_repeat")
			}
			ns, e := nodes(n)
			if e != nil {
				return nil, e
			}
			if len(ns) == 0 {
				return nil, issue(n, "empty_repeat")
			}
			for _, c := range ns {
				if c.Name != "scene" {
					return nil, issue(c, "unknown_tag")
				}
				if e = section(c, over); e != nil {
					return nil, e
				}
			}
		default:
			return nil, issue(n, "unknown_tag")
		}
	}
	return d, nil
}

func readElement(n *Node, d *Document, scope, repeat string, inScene bool, l Limits) (Element, *Problem) {
	var t Element
	if e := attrs(n, "id", "kind", "role", "style", "position", "align", "basis", "start", "end"); e != nil {
		return t, e
	}
	t = Element{ID: n.Attributes["id"], Kind: n.Attributes["kind"], Role: n.Attributes["role"], Style: optional(n, "style", "auto"), Position: optional(n, "position", "auto"), Align: optional(n, "align", "center"), Basis: n.Attributes["basis"], Span: n.Span}
	if t.Kind != "fixed" && t.Kind != "ai" {
		return t, issue(n, "invalid_kind")
	}
	if !slices.Contains([]string{"caption", "info", "badge", "hook", "ending"}, t.Role) {
		return t, issue(n, "invalid_role")
	}
	if t.Style != "auto" && !slices.Contains(d.Styles, t.Style) {
		return t, issue(n, "invalid_style")
	}
	if !slices.Contains([]string{"auto", "top", "upper_mid", "lower_mid", "bottom", "header"}, t.Position) || t.Position == "header" && t.Role != "info" && t.Role != "badge" {
		return t, issue(n, "invalid_position")
	}
	if !slices.Contains([]string{"left", "center", "right"}, t.Align) {
		return t, issue(n, "invalid_align")
	}
	if !slices.Contains([]string{"whole", "output-start", "output-end", "cut"}, t.Basis) || t.Basis == "cut" && !inScene {
		return t, issue(n, "invalid_basis")
	}
	start, hasStart := n.Attributes["start"]
	end, hasEnd := n.Attributes["end"]
	if hasStart != hasEnd || t.Basis == "whole" && hasStart || (t.Basis == "output-start" || t.Basis == "output-end") && !hasStart {
		return t, issue(n, "invalid_interval")
	}
	if hasStart {
		a, ok := Milliseconds(start, l.MaxDurationMS)
		b, ok2 := Milliseconds(end, l.MaxDurationMS)
		if !ok || !ok2 || a >= b || t.Basis == "output-end" && b > 0 || t.Basis != "output-end" && a < 0 {
			return t, issue(n, "invalid_interval")
		}
		t.StartMS = &a
		t.EndMS = &b
	}
	parts := func(parent *Node) ([]Part, *Problem) {
		var out []Part
		for _, c := range parent.Children {
			if c.Name == "#text" {
				out = append(out, Part{Literal: c.Text})
				continue
			}
			if c.Name != "value" {
				return nil, issue(n, "unknown_tag")
			}
			if e := attrs(c, "field"); e != nil {
				return nil, issue(n, e.Reason)
			}
			if len(c.Children) != 0 {
				return nil, issue(n, "unexpected_child")
			}
			ref := c.Attributes["field"]
			found := false
			for _, f := range d.Fields {
				key := f.ID
				if f.Group != "" {
					key = f.Group + "." + f.ID
				}
				if key == ref {
					found = true
					if f.Group != "" && (scope != "item" || repeat != "" && repeat != "scenes" && repeat != f.Group) {
						return nil, issue(n, "binding_scope")
					}
				}
			}
			if !found {
				return nil, issue(n, "unknown_field")
			}
			out = append(out, Part{Field: ref})
		}
		return out, nil
	}
	hasRows := false
	for _, c := range n.Children {
		if c.Name == "row" {
			hasRows = true
		}
	}
	if hasRows {
		if t.Role != "hook" && t.Role != "ending" && t.Role != "info" {
			return t, issue(n, "invalid_rows")
		}
		children, e := nodes(n)
		if e != nil {
			return t, e
		}
		for _, c := range children {
			if c.Name != "row" {
				return t, issue(n, "invalid_rows")
			}
			if e := attrs(c, "role"); e != nil {
				return t, issue(n, e.Reason)
			}
			role := c.Attributes["role"]
			if !slices.Contains(rowRoles, role) {
				return t, issue(n, "invalid_row_role")
			}
			p, e := parts(c)
			if e != nil {
				return t, e
			}
			t.Rows = append(t.Rows, Row{role, p})
		}
	} else {
		p, e := parts(n)
		if e != nil {
			return t, e
		}
		t.Parts = p
	}
	max := l.CopyChars
	if t.Kind == "ai" {
		max = l.GuideChars
	}
	for _, p := range append([]Row{{Parts: t.Parts}}, t.Rows...) {
		count := 0
		for _, part := range p.Parts {
			count += scalar(part.Literal)
		}
		if count > max {
			return t, issue(n, "copy_limit")
		}
	}
	return t, nil
}
