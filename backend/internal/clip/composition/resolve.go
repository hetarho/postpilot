package composition

import (
	"maps"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/design"
)

func fieldKey(f Field) string {
	if f.Group != "" {
		return f.Group + "." + f.ID
	}
	return f.ID
}
func fail(id string, line int, reason string) *Problem {
	return &Problem{ElementID: id, Line: line, Reason: reason}
}

// Resolve binds already-selected real cuts and explicit answers. It never selects
// footage, invents an item association, or calls a writer. maxExpandedBytes is the
// caller's remaining request budget; the final provider envelope needs its own check.
func Resolve(d *Document, in Inputs, l Limits, maxExpandedBytes int) (Timeline, *Problem) {
	var out Timeline
	if !validLimits(l) || maxExpandedBytes <= 0 {
		return out, fail("clip", 1, "invalid_limits")
	}
	if len(in.Cuts) > l.Cuts {
		return out, fail("clip", 1, "cut_limit")
	}
	fields := map[string]Field{}
	for _, f := range d.Fields {
		fields[fieldKey(f)] = f
	}
	validateValues := func(values map[string]string, group, id string) *Problem {
		for _, k := range slices.Sorted(maps.Keys(values)) {
			v := values[k]
			key := k
			if group != "" {
				key = group + "." + k
			}
			f, ok := fields[key]
			if !ok {
				return fail(id, 1, "unknown_field")
			}
			// Two ceilings, in the units each is stated in: the grammar's own
			// scalar bound on a stored answer, then the field's effective
			// maximum, which is a CDS-20 character count (CLIP-117). The second
			// names the field, because that is the control the owner typed into.
			if scalar(v) > l.AnswerChars {
				return fail(id, 1, "answer_limit")
			}
			if bound, capped := d.Maxima[key]; capped && design.Chars(v) > bound {
				return &Problem{ElementID: key, Line: f.Span.Line, Reason: "answer_limit", Label: f.Label, Max: bound, Actual: design.Chars(v)}
			}
		}
		for _, f := range d.Fields {
			if f.Group == group && f.Required && strings.TrimSpace(values[f.ID]) == "" {
				return fail(fieldKey(f), f.Span.Line, "required_binding")
			}
		}
		return nil
	}
	if e := validateValues(in.Values, "", "clip"); e != nil {
		return out, e
	}
	items := map[string]Item{}
	for _, group := range slices.Sorted(maps.Keys(in.Items)) {
		values := in.Items[group]
		index := slices.IndexFunc(d.Groups, func(g Group) bool { return g.ID == group })
		if index < 0 {
			return out, fail(group, 1, "unknown_group")
		}
		if len(values) > min(l.Items, d.Groups[index].Max) {
			return out, fail(group, 1, "item_limit")
		}
		for _, item := range values {
			key := group + "/" + item.ID
			if !identifier.MatchString(item.ID) {
				return out, fail(group, 1, "invalid_id")
			}
			if _, ok := items[key]; ok {
				return out, fail(item.ID, 1, "duplicate_item")
			}
			if e := validateValues(item.Values, group, item.ID); e != nil {
				return out, e
			}
			items[key] = item
		}
	}
	sections := map[string]Section{}
	for _, s := range d.Sections {
		sections[s.ID] = s
	}
	starts := make([]int, len(in.Cuts))
	seen := map[string]bool{}
	lastDuration := 0
	for i, c := range in.Cuts {
		if !identifier.MatchString(c.ID) || c.SourceID == "" || seen[c.ID] {
			return out, fail(c.ID, 1, "invalid_cut")
		}
		seen[c.ID] = true
		if c.SectionID != "" {
			s, ok := sections[c.SectionID]
			if !ok {
				return out, fail(c.ID, 1, "unknown_section")
			}
			if s.Repeat != "" && s.Repeat != "scenes" && c.GroupID != s.Repeat {
				return out, fail(c.ID, 1, "binding_scope")
			}
		}
		if (c.ItemID == "") != (c.GroupID == "") {
			return out, fail(c.ID, 1, "binding_scope")
		}
		if c.ItemID != "" {
			if _, ok := items[c.GroupID+"/"+c.ItemID]; !ok {
				return out, fail(c.ID, 1, "unknown_item")
			}
		}
		// Every interval below resolves on the rate-transformed OUTPUT timeline
		// (CDS-62); the source span keeps its own original timestamps.
		duration, ok := TransformedDurationMS(c.EndMS-c.StartMS, c.Rate())
		if !ok || c.StartMS < 0 || c.EndMS <= c.StartMS || duration > l.MaxDurationMS || c.TransitionMS < 0 || i == 0 && c.TransitionMS != 0 || i > 0 && (c.TransitionMS >= duration || c.TransitionMS >= lastDuration) {
			return out, fail(c.ID, 1, "invalid_cut")
		}
		starts[i] = out.DurationMS - c.TransitionMS
		out.DurationMS = starts[i] + duration
		lastDuration = duration
		if out.DurationMS > l.MaxDurationMS {
			return out, fail(c.ID, 1, "duration_limit")
		}
	}
	if out.DurationMS <= 0 {
		return out, fail("clip", 1, "missing_footage")
	}
	budget := 0
	for _, g := range d.Guidance {
		budget += len(g)
	}
	for _, s := range d.Sections {
		for _, g := range s.Guidance {
			budget += len(g)
		}
	}
	appendElement := func(t Element, c *Cut, cutStart int) *Problem {
		id := t.ID
		group, itemID, cutID := "", "", ""
		values := map[string]string{}
		if c != nil {
			group, itemID, cutID = c.GroupID, c.ItemID, c.ID
			id += "/" + cutID
			if itemID != "" {
				values = items[group+"/"+itemID].Values
				id += "/" + group + "/" + itemID
			}
		}
		r := ResolvedElement{InstanceID: id, CutID: cutID, GroupID: group, ItemID: itemID, Element: t, AuthoredTiming: t.Basis != "cut" || t.StartMS != nil}
		omit := false
		bind := func(parts []Part, kind string) (string, *Problem) {
			var b strings.Builder
			for _, p := range parts {
				if p.Field == "" {
					b.WriteString(p.Literal)
					continue
				}
				f := fields[p.Field]
				v := in.Values[f.ID]
				if f.Group != "" {
					if f.Group != group || itemID == "" {
						return "", fail(t.ID, t.Span.Line, "binding_scope")
					}
					v = values[f.ID]
				}
				if strings.TrimSpace(v) == "" {
					if f.Required {
						return "", fail(t.ID, t.Span.Line, "required_binding")
					}
					omit = true
				}
				b.WriteString(v)
				r.Facts = append(r.Facts, Fact{f.ID, f.Group, func() string {
					if f.Group != "" {
						return itemID
					}
					return ""
				}(), v})
			}
			text := b.String()
			max := l.CopyChars
			if kind == "ai" {
				max = l.GuideChars
			}
			if scalar(text) > max {
				return "", fail(t.ID, t.Span.Line, "copy_limit")
			}
			return text, nil
		}
		var e *Problem
		r.Text, e = bind(t.Parts, t.Kind)
		if e != nil {
			return e
		}
		region := t.Role == "hook" || t.Role == "ending"
		for _, row := range t.Rows {
			text, e := bind(row.Parts, RowKind(t, row))
			if e != nil {
				return e
			}
			if region && omit {
				text = ""
				omit = false
			}
			r.Rows = append(r.Rows, ResolvedRow{row.Role, text})
		}
		if region && strings.TrimSpace(r.Text) == "" && !slices.ContainsFunc(r.Rows, func(row ResolvedRow) bool { return strings.TrimSpace(row.Text) != "" }) {
			return nil
		}
		if omit {
			return nil
		}
		duration := 0
		if c != nil {
			// A cut-relative interval is measured against the cut's transformed
			// output length, not against the source span it was taken from.
			duration, _ = TransformedDurationMS(c.EndMS-c.StartMS, c.Rate())
		}
		a, b, intervalProblem := ResolveInterval(t, out.DurationMS, duration, cutStart, l.AutoInsetMS)
		if intervalProblem != nil {
			return intervalProblem
		}
		r.StartMS, r.EndMS = a, b
		budget += len(r.Text)
		for _, row := range r.Rows {
			budget += len(row.Text)
		}
		for _, f := range r.Facts {
			budget += len(f.Value)
		}
		if budget > maxExpandedBytes {
			return fail(t.ID, t.Span.Line, "expansion_limit")
		}
		out.Elements = append(out.Elements, r)
		if len(out.Elements) > l.Cues {
			return fail(t.ID, t.Span.Line, "cue_limit")
		}
		return nil
	}
	for _, t := range d.Elements {
		if e := appendElement(t, nil, 0); e != nil {
			return Timeline{}, e
		}
	}
	for i, c := range in.Cuts {
		for _, t := range sections[c.SectionID].Elements {
			if e := appendElement(t, &c, starts[i]); e != nil {
				return Timeline{}, e
			}
		}
	}
	if budget > maxExpandedBytes {
		return Timeline{}, fail("clip", 1, "expansion_limit")
	}
	return out, nil
}

// AdmittedSection is one section a project's own answers admit, with the number
// of instances those answers give it (CLIP-103).
type AdmittedSection struct {
	ID, Scope, Repeat string
	Instances         int
}

// AdmittedSections is every section a project's answers actually admit, in
// document order (CLIP-103). A section repeating a declared group takes one
// instance per item the answers hold, so a group the answers left empty admits
// none and drops out; a section repeating `scenes` is bound to no item and
// takes the grammar's own cut ceiling; every other section takes one. A blank
// optional field omits its dependent element under CLIP-60 without removing the
// section that holds it — the scene still selects footage.
//
// It is a bound on what the writer may fill, never a quota to reach.
func AdmittedSections(d *Document, items map[string][]Item, l Limits) []AdmittedSection {
	out := []AdmittedSection{}
	for _, s := range d.Sections {
		instances := 1
		switch {
		case s.Repeat == "scenes":
			instances = l.Cuts
		case s.Repeat != "":
			instances = len(items[s.Repeat])
		}
		if instances <= 0 {
			continue
		}
		out = append(out, AdmittedSection{ID: s.ID, Scope: s.Scope, Repeat: s.Repeat, Instances: instances})
	}
	return out
}
