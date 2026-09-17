package composition

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip/design"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var itemCount = regexp.MustCompile(`^[0-9]+$`)
var seconds = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,3})?$`)
var rowRoles = []string{"caption", "label"}

// declaredChars reads a `chars` attribute (CLIP-116): a positive integer no
// larger than the count the position already imposes. Absent is 0, meaning the
// position keeps its derived cap. A limit of 0 is a position that imposes none.
// blame is the node the failure names, which for a row is the element that
// carries it — the same node every other row failure names.
func declaredChars(n, blame *Node, limit int) (int, *Problem) {
	raw, ok := n.Attributes["chars"]
	if !ok {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if !itemCount.MatchString(raw) || err != nil || value <= 0 || limit > 0 && value > limit {
		return 0, issue(blame, "invalid_max")
	}
	return value, nil
}

// regionSlotChars is the CDS-20 count of the preset slot a region row lands in.
// A ratio changes the hook's size but not its characters (CDS-46), so the
// selection alone answers it.
func regionSlotChars(selection DesignSelection, role string, index int) int {
	kind, id := "intro", selection.Intro
	if role == "ending" {
		kind, id = "outro", selection.Outro
	}
	preset, ok := design.Region(kind, id)
	if !ok || index < 0 || index >= len(preset.Slots) {
		return 0
	}
	return design.Type[preset.Slots[index].Type].Chars
}

// elementChars is the count an element's own text position imposes when it
// carries no rows: an information value reads as a caption, while a badge and a
// caption impose none of their own — their line and wrap rules are CDS-25's and
// the repair ladder's, not a character ceiling.
func elementChars(role string) int {
	if role == "info" {
		return design.Type["caption"].Chars
	}
	return 0
}

func issue(n *Node, reason string) *Problem {
	id := n.Attributes["id"]
	if id == "" {
		id = n.Name
	}
	return &Problem{ElementID: id, Line: n.Span.Line, Reason: reason}
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
	return parse(source, limits, false, false)
}

// ReadStored preserves the old role/style vocabulary solely to reopen saved
// drafts for correction. Every write, quote, preview and render uses strict Parse.
func ReadStored(source string, limits Limits) (*Document, *Problem) {
	return parse(source, limits, true, false)
}

// ParseTemplate is the grammar a TEMPLATE body must satisfy before it is saved
// (CLIP-4, CLIP-59): the fixed regions — intro and outro slots and the badge —
// plus fields, groups and invisible guides. Footage sections, scene-bound text
// and cut-relative timing belong to the flow and the narration now, so a body
// still declaring them is refused with the construct named. Frozen project
// snapshots keep reading through Parse and ReadStored exactly as before
// (CLIP-140), which is why this is a third entry and not a change to either.
func ParseTemplate(source string, limits Limits) (*Document, *Problem) {
	return parse(source, limits, false, true)
}

func parse(source string, limits Limits, stored, template bool) (*Document, *Problem) {
	if !validLimits(limits) {
		return nil, &Problem{ElementID: "clip", Line: 1, Reason: "invalid_limits"}
	}
	if !utf8.ValidString(source) {
		return nil, &Problem{ElementID: "clip", Line: 1, Reason: "invalid_unicode"}
	}
	if scalar(source) > limits.SourceChars {
		return nil, &Problem{ElementID: "clip", Line: 1, Reason: "source_limit"}
	}
	for _, c := range source {
		if !xmlRune(c) {
			return nil, &Problem{ElementID: "clip", Line: 1, Reason: "invalid_unicode"}
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
	rootAttrs := []string{"version", "accent", "pace", "intro", "caption", "outro"}
	if stored {
		rootAttrs = append(rootAttrs, "styles")
	}
	if e = attrs(root, rootAttrs...); e != nil {
		return nil, e
	}
	if root.Attributes["version"] != "1" {
		return nil, issue(root, "unknown_version")
	}
	d := &Document{Source: source, Root: root, Accent: optional(root, "accent", ""), Pace: optional(root, "pace", "steady")}
	d.Design = DesignSelection{Intro: optional(root, "intro", "b"), Caption: optional(root, "caption", "bold"), Outro: optional(root, "outro", "e")}
	if !slices.Contains([]string{"a", "b"}, d.Design.Intro) || d.Design.Caption != "bold" || !slices.Contains([]string{"b", "e"}, d.Design.Outro) {
		return nil, issue(root, "invalid_design")
	}
	if !stored {
		for _, attr := range []string{"intro", "caption", "outro"} {
			if _, present := root.Attributes[attr]; !present {
				return nil, issue(root, "invalid_design")
			}
		}
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
		if e := attrs(n, "id", "label", "required", "chars"); e != nil {
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
		chars, e := declaredChars(n, n, limits.AnswerChars)
		if e != nil {
			return e
		}
		d.Fields = append(d.Fields, Field{ID: n.Attributes["id"], Group: group, Label: label, Prompt: prompt, Required: required == "true", Chars: chars, Span: n.Span})
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
			if e = attrs(n, "id", "label", "min", "max"); e != nil {
				return nil, e
			}
			if e = claim(n, ""); e != nil {
				return nil, e
			}
			group := n.Attributes["id"]
			if group == "scenes" {
				return nil, issue(n, "invalid_id")
			}
			declaration := Group{ID: group, Label: n.Attributes["label"], Max: limits.Items, Span: n.Span}
			if scalar(declaration.Label) > limits.LabelChars {
				return nil, issue(n, "field_limit")
			}
			for _, bound := range []struct {
				key   string
				value *int
			}{{"min", &declaration.Min}, {"max", &declaration.Max}} {
				if raw, ok := n.Attributes[bound.key]; ok {
					value, err := strconv.Atoi(raw)
					if !itemCount.MatchString(raw) || err != nil || value > limits.Items {
						return nil, issue(n, "invalid_item_bounds")
					}
					*bound.value = value
				}
			}
			if declaration.Min > declaration.Max {
				return nil, issue(n, "invalid_item_bounds")
			}
			d.Groups = append(d.Groups, declaration)
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
				v, e := readElement(c, d, scope, repeat, true, limits, stored, template)
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
			v, e := readElement(n, d, "context", "", false, limits, stored, template)
			if e != nil {
				return nil, e
			}
			d.Elements = append(d.Elements, v)
		case "scene":
			if template {
				return nil, issue(n, "unsupported_section")
			}
			if e = section(n, ""); e != nil {
				return nil, e
			}
		case "repeat":
			if template {
				return nil, issue(n, "unsupported_section")
			}
			if e = attrs(n, "for"); e != nil {
				return nil, e
			}
			over := n.Attributes["for"]
			if over != "scenes" && !slices.ContainsFunc(d.Groups, func(g Group) bool { return g.ID == over }) {
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
	if !stored {
		for _, role := range []string{"hook", "ending"} {
			count := 0
			for _, element := range d.Elements {
				if element.Role == role {
					count++
				}
				if element.Role == role && count > 1 {
					return nil, &Problem{ElementID: element.ID, Line: element.Span.Line, Reason: "invalid_skeleton"}
				}
			}
			if count != 1 {
				return nil, issue(root, "invalid_skeleton")
			}
		}
	}
	d.Maxima = fieldMaxima(d, limits)
	d.Minima = groupMinima(d)
	return d, nil
}

// groupMinima gives every repeated item group the number of items it actually
// admits (CLIP-119): its declared minimum, else one when any field of the group
// is required, else zero. A required field that no item carries is satisfied by
// nothing, so a group holding one admits at least one item even when the
// template that saved it declared no minimum.
func groupMinima(d *Document) map[string]int {
	required := map[string]bool{}
	for _, f := range d.Fields {
		if f.Group != "" && f.Required {
			required[f.Group] = true
		}
	}
	out := make(map[string]int, len(d.Groups))
	for _, g := range d.Groups {
		value := g.Min
		if value == 0 && required[g.ID] {
			value = 1
		}
		out[g.ID] = value
	}
	return out
}

// fieldMaxima folds every position a field's value reaches into one number
// (CLIP-117). A position contributes the maximum it actually enforces — its
// authored value when it declares one, otherwise the count its rendered
// position imposes — and a position that imposes none contributes nothing.
func fieldMaxima(d *Document, l Limits) map[string]int {
	out := make(map[string]int, len(d.Fields))
	for _, f := range d.Fields {
		value := l.AnswerChars
		if f.Chars > 0 && f.Chars < value {
			value = f.Chars
		}
		out[fieldKey(f)] = value
	}
	fold := func(parts []Part, limit int) {
		if limit <= 0 {
			return
		}
		for _, p := range parts {
			if current, ok := out[p.Field]; p.Field != "" && ok && limit < current {
				out[p.Field] = limit
			}
		}
	}
	// Only a position the ANSWER reaches contributes (CLIP-117). In an `ai` position the
	// answer is material the writer reads, not text that lands there: the position's own
	// bound belongs to what the model writes and is stated to it under CLIP-118. Folding it
	// into the field capped 기타 정보 at the 18 characters of the line the model writes FROM
	// it, so an answer the control had accepted was refused at generation.
	visit := func(t Element) {
		if len(t.Rows) == 0 {
			if t.Kind == "ai" {
				return
			}
			limit := t.Chars
			if limit == 0 {
				limit = elementChars(t.Role)
			}
			fold(t.Parts, limit)
			return
		}
		for i, row := range t.Rows {
			if RowKind(t, row) == "ai" {
				continue
			}
			limit := row.Chars
			if limit == 0 {
				limit = regionSlotChars(d.Design, t.Role, i)
				if t.Role == "info" {
					limit = design.Type[row.Role].Chars
				}
			}
			fold(row.Parts, limit)
		}
	}
	for _, t := range d.Elements {
		visit(t)
	}
	for _, section := range d.Sections {
		for _, t := range section.Elements {
			visit(t)
		}
	}
	return out
}

func readElement(n *Node, d *Document, scope, repeat string, inScene bool, l Limits, stored, template bool) (Element, *Problem) {
	var t Element
	allowed := []string{"id", "kind", "role", "position", "align", "basis", "start", "end", "chars"}
	if stored {
		allowed = append(allowed, "style")
	}
	if e := attrs(n, allowed...); e != nil {
		return t, e
	}
	t = Element{ID: n.Attributes["id"], Kind: n.Attributes["kind"], Role: n.Attributes["role"], Style: "auto", Position: optional(n, "position", "auto"), Align: optional(n, "align", "center"), Basis: n.Attributes["basis"], Span: n.Span}
	if t.Kind != "fixed" && t.Kind != "ai" {
		return t, issue(n, "invalid_kind")
	}
	if !slices.Contains([]string{"caption", "info", "badge", "hook", "ending"}, t.Role) {
		return t, issue(n, "invalid_role")
	}
	// A template carries no caption or information element of its own any
	// more: captions are the narration's and cut-bound information went with
	// the sections (CLIP-4, CLIP-65). The role is named before the basis so a
	// legacy caption is refused for what it is, not for when it showed.
	if template && (t.Role == "caption" || t.Role == "info") {
		return t, issue(n, "unsupported_role")
	}
	if template && t.Basis == "cut" {
		return t, issue(n, "unsupported_basis")
	}
	region := t.Role == "hook" || t.Role == "ending"
	if region && !stored {
		_, hasPosition := n.Attributes["position"]
		_, hasAlign := n.Attributes["align"]
		if inScene || hasPosition || hasAlign || t.Role == "hook" && t.Basis != "output-start" || t.Role == "ending" && t.Basis != "output-end" {
			return t, issue(n, "invalid_skeleton")
		}
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
	// The element's own text position. A position with no count of its own is
	// still bounded by the grammar's copy ceiling.
	limit := elementChars(t.Role)
	if limit == 0 {
		limit = l.CopyChars
	}
	chars, problem := declaredChars(n, n, limit)
	if problem != nil {
		return t, problem
	}
	t.Chars = chars
	start, hasStart := n.Attributes["start"]
	end, hasEnd := n.Attributes["end"]
	if hasStart != hasEnd || t.Basis == "whole" && hasStart || (t.Basis == "output-start" || t.Basis == "output-end") && !hasStart && !region {
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
	if region && !hasStart && (t.Basis == "output-start" || t.Basis == "output-end") {
		a, b := 0, int(design.Timing.IntroDefaultS*1000)
		if t.Role == "ending" {
			a, b = -int(design.Timing.OutroDefaultS*1000), 0
		}
		t.StartMS, t.EndMS = &a, &b
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
			rowAttrs := []string{"role", "chars"}
			if region {
				rowAttrs = append(rowAttrs, "kind")
			}
			if region && !stored {
				rowAttrs = []string{"kind", "chars"}
			}
			if e := attrs(c, rowAttrs...); e != nil {
				if region {
					return t, issue(n, "invalid_skeleton")
				}
				return t, issue(n, e.Reason)
			}
			role := c.Attributes["role"]
			if region {
				role = ""
			} else if stored && role != "label" {
				role = "caption"
			} else if !slices.Contains(rowRoles, role) {
				return t, issue(n, "invalid_row_role")
			}
			kind := optional(c, "kind", t.Kind)
			if kind != "fixed" && kind != "ai" {
				return t, issue(n, "invalid_kind")
			}
			p, e := parts(c)
			if e != nil {
				return t, e
			}
			// A row's position is its preset slot when the element is a region
			// block, and its own row role inside an information pair.
			slot := regionSlotChars(d.Design, t.Role, len(t.Rows))
			if !region {
				slot = design.Type[role].Chars
			}
			if slot == 0 {
				slot = l.CopyChars
			}
			rowChars, e := declaredChars(c, n, slot)
			if e != nil {
				return t, e
			}
			t.Rows = append(t.Rows, Row{Role: role, Kind: kind, Chars: rowChars, Parts: p})
		}
	} else {
		p, e := parts(n)
		if e != nil {
			return t, e
		}
		t.Parts = p
	}
	if region && !stored {
		kind, id := "intro", d.Design.Intro
		if t.Role == "ending" {
			kind, id = "outro", d.Design.Outro
		}
		preset, _ := design.Region(kind, id)
		if len(t.Rows) > len(preset.Slots) {
			return t, issue(n, "invalid_skeleton")
		}
		for _, p := range t.Parts {
			if p.Field != "" || strings.TrimSpace(p.Literal) != "" {
				return t, issue(n, "invalid_skeleton")
			}
		}
	}
	for _, p := range append([]Row{{Kind: t.Kind, Parts: t.Parts}}, t.Rows...) {
		max := l.CopyChars
		if p.Kind == "ai" {
			max = l.GuideChars
		}
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
