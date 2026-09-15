package ai

import (
	"slices"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

func copyKey(element, cut string) string { return element + "/" + cut }

func attachCompositionCopy(cfg Config, doc *composition.Document, generated []generatedJSON, timeline composition.Timeline, plan *clip.PortablePlan, bindings map[string]clip.ItemBinding, evidence map[string][]clip.ObservedEvidence, owner *clip.EditPlan) error {
	declared, scopes := map[string]composition.Element{}, map[string]string{}
	for _, element := range doc.Elements {
		key := copyKey(element.ID, "")
		declared[key], scopes[key] = element, "context"
	}
	for _, cut := range plan.Cuts {
		for _, section := range doc.Sections {
			if section.ID != cut.SectionID {
				continue
			}
			for _, element := range section.Elements {
				key := copyKey(element.ID, cut.ID)
				declared[key], scopes[key] = element, section.Scope
			}
		}
		// Output-level context can cite only selected footage, never a source
		// range that the finished timeline does not contain.
		evidence[""] = append(evidence[""], evidence[cut.ID]...)
	}

	entries := map[string]generatedJSON{}
	removed := map[string]string{}
	seen := map[string]bool{}
	for index, g := range generated {
		key := copyKey(g.ElementID, g.CutID)
		element, exists := declared[key]
		reason := ""
		switch {
		case seen[key] || !exists || !generatesText(element):
			reason = "composition_generated_identity"
		case index >= cfg.Template.Composition.Cues || !within(g.Text, 0, cfg.Template.Composition.CopyChars) || !within(g.ShortText, 0, cfg.Template.Composition.CopyChars) || !within(g.Keyword, 0, cfg.Template.Composition.LabelChars) || len(g.Facts) > cfg.Template.Composition.Fields || len(g.Observations) > 120:
			reason = "composition_generated_bounds"
		case len(g.Rows) != len(element.Rows) || len(g.ShortRows) != 0 && len(g.ShortRows) != len(element.Rows) || len(element.Rows) > 0 && (g.Text != "" || g.ShortText != ""):
			reason = "composition_generated_rows"
		}
		seen[key] = true
		for _, row := range append(slices.Clone(g.Rows), g.ShortRows...) {
			if !within(row, 0, cfg.Template.Composition.CopyChars) {
				reason = "composition_generated_bounds"
			}
		}
		if reason != "" {
			delete(entries, key)
			removed[key] = reason
			f := clip.CopyFallback{ElementID: g.ElementID, CutID: g.CutID, Reason: reason}
			if !slices.Contains(plan.Fallbacks, f) {
				plan.Fallbacks = append(plan.Fallbacks, f)
			}
			continue
		}
		if removed[key] == "" {
			entries[key] = g
		}
	}
	used := map[string]bool{}
	for _, resolved := range timeline.Elements {
		key := copyKey(resolved.Element.ID, resolved.CutID)
		text := clip.PortableText{Resolved: resolved, Scope: scopes[key], Accent: doc.Accent, Pace: doc.Pace}
		for _, observed := range evidence[resolved.CutID] {
			text.Evidence = append(text.Evidence, observed.Source)
		}
		if region, _ := regionSelection(doc, resolved.Element); region != "" && len(resolved.Element.Rows) > 0 {
			entry, exists := entries[key]
			attachRegionRows(doc, plan.Inputs, entry, exists && removed[key] == "", bindings[resolved.CutID], evidence[resolved.CutID], &text, owner)
			if slices.ContainsFunc(text.Resolved.Rows, func(row composition.ResolvedRow) bool { return strings.TrimSpace(row.Text) != "" }) {
				plan.Elements = append(plan.Elements, text)
			}
			continue
		}
		if resolved.Element.Kind == "fixed" {
			plan.Elements = append(plan.Elements, text)
			continue
		}
		if removed[key] != "" {
			continue
		}
		entry, exists := entries[key]
		reason := "copy_not_generated"
		if exists {
			reason = resolveGeneratedCopy(doc, plan.Inputs, entry, bindings[resolved.CutID], evidence[resolved.CutID], &text, used)
		}
		if reason != "" {
			plan.Fallbacks = append(plan.Fallbacks, clip.CopyFallback{ElementID: resolved.Element.ID, CutID: resolved.CutID, Reason: reason})
			continue
		}
		if text.FallbackReason != "" {
			plan.Fallbacks = append(plan.Fallbacks, clip.CopyFallback{ElementID: resolved.Element.ID, CutID: resolved.CutID, Reason: text.FallbackReason})
		}
		plan.Elements = append(plan.Elements, text)
	}
	return nil
}

func sentenceKey(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, text)
}

func resolveGeneratedCopy(doc *composition.Document, inputs clip.CompositionInputs, entry generatedJSON, binding clip.ItemBinding, observed []clip.ObservedEvidence, out *clip.PortableText, used map[string]bool) string {
	evidence, valid := selectedReferences(entry.Observations, observed, false)
	if !valid || out.Scope != "context" && len(evidence) == 0 || len(evidence) == 0 && len(entry.Facts) == 0 {
		return "missing_scene_evidence"
	}
	var facts []composition.Fact
	seen := map[factJSON]bool{}
	for _, ref := range entry.Facts {
		fact, valid := clip.ScopedFact(doc, inputs, clip.FactReference{FieldID: ref.FieldID, GroupID: ref.GroupID, ItemID: ref.ItemID}, out.Scope, binding)
		if !valid || seen[ref] {
			return "unavailable_scoped_fact"
		}
		seen[ref] = true
		facts = append(facts, fact)
	}
	alternative := func(text string, rows []string) clip.CopyAlternative {
		candidate := clip.CopyAlternative{Text: text}
		for i, row := range rows {
			candidate.Rows = append(candidate.Rows, composition.ResolvedRow{Role: out.Resolved.Element.Rows[i].Role, Text: row})
		}
		return candidate
	}
	check := func(candidate clip.CopyAlternative) string {
		texts := []string{candidate.Text}
		for _, row := range candidate.Rows {
			texts = append(texts, row.Text)
		}
		combined := strings.Join(texts, " ")
		if strings.TrimSpace(combined) == "" {
			return "copy_omitted"
		}
		if reason := clip.GroundScopedText(combined, facts, inputs, binding, out.Scope); reason != "" {
			return reason
		}
		if out.Resolved.Element.Role == "caption" && used[sentenceKey(combined)] {
			return "repeated_copy"
		}
		return ""
	}
	full, short := alternative(entry.Text, entry.Rows), alternative(entry.ShortText, entry.ShortRows)
	reason, shortReason := check(full), check(short)
	chosen := full
	if reason != "" {
		if shortReason != "" {
			return reason
		}
		chosen, out.FallbackReason = short, reason
	} else if shortReason == "" && (short.Text != full.Text || !slices.Equal(short.Rows, full.Rows)) {
		out.Alternatives = []clip.CopyAlternative{short}
	}
	out.Resolved.Text, out.Resolved.Rows, out.Resolved.Facts = chosen.Text, chosen.Rows, facts
	out.Evidence = evidence
	combined := chosen.Text
	for _, row := range chosen.Rows {
		combined += " " + row.Text
	}
	if entry.Keyword != "" && strings.Contains(combined, entry.Keyword) {
		out.Keyword = entry.Keyword
	}
	if out.Resolved.Element.Role == "caption" {
		used[sentenceKey(combined)] = true
	}
	return ""
}

func generatesText(e composition.Element) bool {
	if len(e.Rows) == 0 {
		return e.Kind == "ai"
	}
	return slices.ContainsFunc(e.Rows, func(row composition.Row) bool { return composition.RowKind(e, row) == "ai" })
}

func regionSelection(doc *composition.Document, e composition.Element) (string, string) {
	switch e.Role {
	case "hook":
		return "intro", doc.Design.Intro
	case "ending":
		return "outro", doc.Design.Outro
	}
	return "", ""
}

// Ground and repair AI rows individually. A missing or malformed response can
// empty generated slots, but can never replace their fixed neighbours.
func attachRegionRows(doc *composition.Document, inputs clip.CompositionInputs, entry generatedJSON, exists bool, binding clip.ItemBinding, observed []clip.ObservedEvidence, out *clip.PortableText, owner *clip.EditPlan) {
	e := out.Resolved.Element
	region, id := regionSelection(doc, e)
	preset, _ := design.Region(region, id)
	out.Resolved.Rows = slices.Clone(out.Resolved.Rows)
	if generatesText(e) {
		out.Evidence = nil
	}
	for i, row := range e.Rows {
		if composition.RowKind(e, row) != "ai" {
			continue
		}
		value, action := "", "removal"
		if exists && i < len(entry.Rows) && i < len(preset.Slots) {
			one := *out
			one.Resolved.Element.Rows = nil
			one.Resolved.Rows = nil
			one.Alternatives = nil
			one.FallbackReason = ""
			g := entry
			g.Text, g.Rows = entry.Rows[i], nil
			g.ShortText, g.ShortRows = "", nil
			if i < len(entry.ShortRows) {
				g.ShortText = entry.ShortRows[i]
			}
			reason := resolveGeneratedCopy(doc, inputs, g, binding, observed, &one, map[string]bool{})
			if reason == "" {
				value, action = repairGeneratedSlot(one.Resolved.Text, one.Alternatives, design.Type[preset.Slots[i].Type].Chars)
				if action == "" && one.FallbackReason != "" {
					action = "repair"
				}
				for _, ref := range one.Evidence {
					if !slices.Contains(out.Evidence, ref) {
						out.Evidence = append(out.Evidence, ref)
					}
				}
				for _, fact := range one.Resolved.Facts {
					if !slices.Contains(out.Resolved.Facts, fact) {
						out.Resolved.Facts = append(out.Resolved.Facts, fact)
					}
				}
			}
		}
		out.Resolved.Rows[i].Text = value
		if action != "" {
			suffix := "slot_shortened"
			if action == "removal" {
				suffix = "slot_omitted"
			}
			clip.AddPlanNotice(owner, region+"_"+suffix, out.Resolved.CutID, e.ID, action)
		}
	}
	out.Resolved.Text = ""
	out.Keyword = ""
	out.Alternatives = nil
}
