package clip

import (
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// NoticeRegionLineSurplus is a region line the preset the project chose cannot
// draw. The SERVER dropped it, which is what CLIP-138 admits a notice for.
const NoticeRegionLineSurplus = "region_line_surplus"

// regionKind is the design region a role's lines are drawn in, or "" for a role
// that is drawn in none.
func regionKind(role string) string {
	switch role {
	case "hook":
		return "intro"
	case "ending":
		return "outro"
	}
	return ""
}

// regionLines is how many lines one region entry carries: its rows, or the one
// line its own text is. An empty line keeps its slot (CDS-73).
func regionLines(e composition.ResolvedElement) int {
	if n := len(e.Rows); n > 0 {
		return n
	}
	return 1
}

// RegionPlacement is where one region entry's lines land in the preset the
// PROJECT chose: the slot its first line takes, how many of its lines that
// preset can draw, and whether this entry is the one that paints the preset's
// own rules — the first entry of its region carrying a line to paint, so two
// entries never overdraw one rule (CDS-73).
type RegionPlacement struct {
	Offset, Drawn int
	Rules         bool
}

// RegionPlacements places every region entry, keyed by resolved instance id.
// Entries of the same region fill its slots in the order they stand in, so two
// entries of one line each fill exactly what one entry of two lines fills, and
// a line past the last slot is drawn by nobody (CLIP-147).
func RegionPlacements(elements []composition.ResolvedElement, presets composition.DesignSelection) map[string]RegionPlacement {
	out, taken, ruled := map[string]RegionPlacement{}, map[string]int{}, map[string]bool{}
	for _, e := range elements {
		kind := regionKind(e.Element.Role)
		if kind == "" {
			continue
		}
		slots := 0
		if preset, ok := design.Region(kind, RegionPresetID(presets, kind)); ok {
			slots = len(preset.Slots)
		}
		placement := RegionPlacement{Offset: taken[kind], Drawn: max(0, min(regionLines(e), slots-taken[kind]))}
		if !ruled[kind] && regionPaints(e, placement.Drawn) {
			placement.Rules, ruled[kind] = true, true
		}
		out[e.InstanceID] = placement
		taken[kind] += regionLines(e)
	}
	return out
}

// regionPaints reports whether any line this entry actually draws carries text.
func regionPaints(e composition.ResolvedElement, drawn int) bool {
	if len(e.Rows) == 0 {
		return drawn > 0 && strings.TrimSpace(e.Text) != ""
	}
	for i, row := range e.Rows {
		if i >= drawn {
			break
		}
		if strings.TrimSpace(row.Text) != "" {
			return true
		}
	}
	return false
}

// RegionPresetID is the preset a region renders in: the project's selection, or
// the shared default where it made none (CLIP-139).
func RegionPresetID(presets composition.DesignSelection, kind string) string {
	if kind == "outro" {
		return presets.Outro
	}
	return presets.Intro
}

// ResolvedElements is the resolved half of a plan's declared text, which is
// what a placement is computed over.
func ResolvedElements(texts []PortableText) []composition.ResolvedElement {
	out := make([]composition.ResolvedElement, 0, len(texts))
	for _, text := range texts {
		out = append(out, text.Resolved)
	}
	return out
}

// RegionSurplusNotices is one notice per region entry the chosen preset cannot
// draw in full (CLIP-108, CLIP-147). They are derived rather than stored so a
// preset the owner changes in ① never leaves one behind (CLIP-139).
func RegionSurplusNotices(plan EditPlan, presets composition.DesignSelection) []PlanNotice {
	if plan.Portable == nil {
		return nil
	}
	placements := RegionPlacements(ResolvedElements(plan.Portable.Elements), presets)
	var out []PlanNotice
	for _, text := range plan.Portable.Elements {
		if regionKind(text.Resolved.Element.Role) == "" {
			continue
		}
		if placements[text.Resolved.InstanceID].Drawn < regionLines(text.Resolved) {
			out = append(out, PlanNotice{
				CopyFallback: CopyFallback{ElementID: text.Resolved.Element.ID, CutID: text.Resolved.CutID, Reason: NoticeRegionLineSurplus},
				Action:       "removal",
			})
		}
	}
	return out
}

// DeclaredCaptions is the outline's own caption entries, in the order the body
// carries them (CLIP-112). Where each one plays is the narration's to decide
// over the resolved flow, so none of them reaches a plan from the flow call.
// A caption bound to a cut belongs to a frozen legacy snapshot and is not one
// of these (CLIP-140).
func DeclaredCaptions(timeline composition.Timeline) []composition.ResolvedElement {
	var out []composition.ResolvedElement
	for _, e := range timeline.Elements {
		if e.Element.Role == "caption" && e.CutID == "" {
			out = append(out, e)
		}
	}
	return out
}
