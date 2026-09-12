package ai

import (
	"slices"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func copyKey(element, cut string) string { return element + "/" + cut }

func attachCompositionCopy(cfg Config, doc *composition.Document, generated []generatedJSON, timeline composition.Timeline, plan *clip.PortablePlan, bindings map[string]clip.ItemBinding, evidence map[string][]clip.ObservedEvidence) error {
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
	for _, g := range generated {
		key := copyKey(g.ElementID, g.CutID)
		element, exists := declared[key]
		if _, duplicate := entries[key]; duplicate || !exists || element.Kind != "ai" {
			return outputError("composition_generated_identity")
		}
		if !within(g.Text, 0, cfg.Template.Composition.CopyChars) || !within(g.ShortText, 0, cfg.Template.Composition.CopyChars) || !within(g.Keyword, 0, cfg.Template.Composition.LabelChars) || len(g.Facts) > cfg.Template.Composition.Fields || len(g.Observations) > 120 {
			return outputError("composition_generated_bounds")
		}
		if len(g.Rows) != len(element.Rows) || len(g.ShortRows) != 0 && len(g.ShortRows) != len(element.Rows) || len(element.Rows) > 0 && (g.Text != "" || g.ShortText != "") {
			return outputError("composition_generated_rows")
		}
		for _, row := range append(slices.Clone(g.Rows), g.ShortRows...) {
			if !within(row, 0, cfg.Template.Composition.CopyChars) {
				return outputError("composition_generated_bounds")
			}
		}
		entries[key] = g
	}
	used := map[string]bool{}
	for _, resolved := range timeline.Elements {
		key := copyKey(resolved.Element.ID, resolved.CutID)
		text := clip.PortableText{Resolved: resolved, Scope: scopes[key], Accent: doc.Accent, Pace: doc.Pace}
		for _, observed := range evidence[resolved.CutID] {
			text.Evidence = append(text.Evidence, observed.Source)
		}
		if resolved.Element.Kind == "fixed" {
			plan.Elements = append(plan.Elements, text)
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
