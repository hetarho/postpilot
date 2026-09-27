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

// resolveGeneratedCopy is one generated row's ladder: the full row, then the shorter one. It
// checks the row's form and nothing it states (CLIP-184, CDS-1): what copy may say is the
// 영상 지침's.
func resolveGeneratedCopy(entry generatedJSON, out *clip.PortableText) string {
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
		if strings.TrimSpace(strings.Join(texts, " ")) == "" {
			return "copy_omitted"
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
	out.Resolved.Text, out.Resolved.Rows = chosen.Text, chosen.Rows
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

// Fit and repair AI rows individually. A missing or malformed response can
// empty generated slots, but can never replace their fixed neighbours.
func attachRegionRows(presets composition.DesignSelection, ratio string, offset int, entry generatedJSON, exists bool, out *clip.PortableText, owner *clip.EditPlan) {
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
			if resolveGeneratedCopy(g, &one) == "" {
				// The slot's own fit, and the smaller bound this row declares
				// (CLIP-116); the parser has already refused a larger one.
				value, action = repairGeneratedSlot(one.Resolved.Text, one.Alternatives, spec, width, row.Chars)
				if action == "" && one.FallbackReason != "" {
					action = "repair"
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
