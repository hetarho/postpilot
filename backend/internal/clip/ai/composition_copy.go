package ai

import (
	"slices"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

func sentenceKey(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, text)
}

func resolveGeneratedCopy(doc *composition.Document, inputs clip.CompositionInputs, entry generatedJSON, binding clip.ItemBinding, observed []clip.ObservedEvidence, out *clip.PortableText, used map[string]bool, instructed bool) string {
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
		if reason := clip.GroundScopedText(combined, facts, inputs, binding, out.Scope, instructed); reason != "" {
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

// regionSelection is the preset a region entry is drawn in: the PROJECT's,
// never the body's, because a template declares no design (CLIP-14, CLIP-139).
func regionSelection(presets composition.DesignSelection, e composition.Element) (string, string) {
	switch e.Role {
	case "hook":
		return "intro", presets.Intro
	case "ending":
		return "outro", presets.Outro
	}
	return "", ""
}

// Ground and repair AI rows individually. A missing or malformed response can
// empty generated slots, but can never replace their fixed neighbours.
func attachRegionRows(presets composition.DesignSelection, ratio string, offset int, doc *composition.Document, inputs clip.CompositionInputs, entry generatedJSON, exists bool, binding clip.ItemBinding, observed []clip.ObservedEvidence, out *clip.PortableText, owner *clip.EditPlan, instructed bool) {
	e := out.Resolved.Element
	region, id := regionSelection(presets, e)
	out.Resolved.Rows = slices.Clone(out.Resolved.Rows)
	if generatesText(e) {
		out.Evidence = nil
	}
	for i, row := range e.Rows {
		if composition.RowKind(e, row) != "ai" {
			continue
		}
		value, action := "", "removal"
		spec, width, slotted := design.RegionSlotAt(region, id, ratio, offset+i)
		if exists && i < len(entry.Rows) && slotted {
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
			reason := resolveGeneratedCopy(doc, inputs, g, binding, observed, &one, map[string]bool{}, instructed)
			if reason == "" {
				// The slot's own fit, and the smaller bound this row declares
				// (CLIP-116); the parser has already refused a larger one.
				value, action = repairGeneratedSlot(one.Resolved.Text, one.Alternatives, spec, width, row.Chars)
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
