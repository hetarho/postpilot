package clip

import (
	"reflect"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// RegionElementID is the one plan element a project region draws from
// (CLIP-147, CLIP-188). Its rows are the region's active slots in slot order,
// so the n-th row is always the n-th slot whatever the others hold — a blank
// slot keeps its row — and no template entry of that region is drawn beside it.
func RegionElementID(kind string) string { return "project-" + kind }

func regionRole(kind string) string {
	if kind == "outro" {
		return "ending"
	}
	return "hook"
}

// regionBasis is the output-relative scope a region's interval is held in: the
// output's opening for the intro, its end for the outro (CLIP-66).
func regionBasis(kind string) string {
	if kind == "outro" {
		return "output-end"
	}
	return "output-start"
}

// regionPlanLines is every line a plan's entries of one region carry, in the
// slot order RegionPlacements fills — lines past the preset's slots included —
// whether the plan holds any entry of that region, and whether one of them is
// the projected one.
func regionPlanLines(texts []PortableText, kind string) (lines []string, present, projected bool) {
	for _, t := range texts {
		if regionKind(t.Resolved.Element.Role) != kind {
			continue
		}
		present = true
		projected = projected || t.Resolved.InstanceID == RegionElementID(kind)
		lines = append(lines, regionRowTexts(t.Resolved)...)
	}
	return lines, present, projected
}

// regionSlot is the region's slot at index, created with its ordinal id where
// the draft holds fewer.
func regionSlot(region *ProjectRegion, kind string, index int) *RegionSlot {
	for len(region.Slots) <= index {
		region.Slots = append(region.Slots, RegionSlot{ID: RegionSlotID(kind, len(region.Slots))})
	}
	return &region.Slots[index]
}

// authoredSlot reports whether a slot's text is the owner's or an answer's
// rather than a writer's: such text is drawn as written and never repaired.
func authoredSlot(s RegionSlot) bool { return s.OwnerFixed || len(s.Binding) > 0 }

// AbsorbWrittenRegions carries the words a writer left in a plan's own region
// entries into the project's generated slots (CLIP-190): a plan the projection
// has not written yet holds them in its template entries. A slot the owner
// fixed or an answer binds keeps its own text, and a region that is off takes
// nothing, since the writer drafts enabled slots only (CLIP-187).
func AbsorbWrittenRegions(r *ProjectRegions, plan EditPlan) {
	if plan.Portable == nil {
		return
	}
	for _, kind := range []string{"intro", "outro"} {
		region := r.Region(kind)
		lines, present, projected := regionPlanLines(plan.Portable.Elements, kind)
		if !region.Enabled || !present || projected {
			continue
		}
		for i, line := range lines {
			if slot := regionSlot(region, kind, i); !authoredSlot(*slot) {
				slot.Text = line
			}
		}
	}
}

// ReconcileRegionCorrection carries what a correction did to region entries
// into the project's slots (CLIP-188): a line it changed becomes that slot's
// owner-fixed text, removing a region's entry switches the region off and
// restoring it switches the region back on, neither touching a draft
// (CLIP-189). before is the plan the correction was applied to, and every
// region entry after it names an identity that plan archived. A correction
// adds and removes no line of an entry and gives an entry with lines no text of
// its own.
func ReconcileRegionCorrection(r *ProjectRegions, before PortablePlan, after []PortableText) error {
	known := map[string]PortableText{}
	for _, t := range slices.Concat(before.RetiredElements, before.Elements) {
		known[t.Resolved.InstanceID] = t
	}
	for _, kind := range []string{"intro", "outro"} {
		region := r.Region(kind)
		old, wasPresent, _ := regionPlanLines(before.Elements, kind)
		// A restored entry is compared with its own archived lines, so bringing
		// it back is not an edit of its words.
		var restored []string
		present := false
		for _, t := range after {
			if regionKind(t.Resolved.Element.Role) != kind {
				continue
			}
			prior, ok := known[t.Resolved.InstanceID]
			if !ok || len(t.Resolved.Rows) != len(prior.Resolved.Rows) || len(t.Resolved.Rows) > 0 && t.Resolved.Text != "" {
				return ErrInvalid
			}
			present = true
			restored = append(restored, regionRowTexts(prior.Resolved)...)
		}
		if present != wasPresent {
			region.Enabled = present
		}
		if !present {
			continue
		}
		if !wasPresent {
			old = restored
		}
		// A removed entry's lines leave the block and the lines under it move
		// up, exactly as the draft preview drew them.
		lines, _, _ := regionPlanLines(after, kind)
		for i := range max(len(lines), len(old)) {
			line, prior := "", ""
			if i < len(lines) {
				line = lines[i]
			}
			if i < len(old) {
				prior = old[i]
			}
			if line != prior {
				slot := regionSlot(region, kind, i)
				slot.Text, slot.OwnerFixed, slot.Binding = line, true, nil
			}
		}
	}
	return nil
}

// regionSlotFit is why a slot of the chosen preset cannot draw text, or "" when
// it can: wider than its width at its floor, or holding a character its face
// lacks (CDS-77, CDS-86).
func regionSlotFit(kind, preset, ratio string, index int, text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if strings.ContainsAny(text, "\r\n") {
		return "copy_limit"
	}
	spec, width, ok := design.RegionSlotAt(kind, preset, ratio, index)
	if !ok {
		return "invalid_design"
	}
	if !design.FitRegionSlot(spec, text, width).Over {
		return ""
	}
	if !design.Covers(spec.Face, spec.Weight, text) {
		return "unsupported_glyph"
	}
	return "copy_limit"
}

// regionCapacity is how many slots the chosen preset draws.
func regionCapacity(kind string, presets composition.DesignSelection) int {
	preset, ok := design.Region(kind, RegionPresetID(presets, kind))
	if !ok {
		return 0
	}
	return len(preset.Slots())
}

// ValidateOwnerRegions refuses active slot text the owner fixed that its slot
// cannot draw, naming that slot (CDS-64, CDS-77, CLIP-189): an owner's words are
// never shortened or dropped to fit. A generated text that does not fit is left
// undrawn and noticed instead, text a template or an answer supplies is drawn
// as written and refused where the plan is laid out, and a region that is off
// draws nothing to check.
func ValidateOwnerRegions(r ProjectRegions, presets composition.DesignSelection, ratio string) error {
	for _, kind := range []string{"intro", "outro"} {
		region := r.Region(kind)
		if !region.Enabled {
			continue
		}
		for i, slot := range region.Slots[:min(regionCapacity(kind, presets), len(region.Slots))] {
			if !slot.OwnerFixed {
				continue
			}
			if reason := regionSlotFit(kind, RegionPresetID(presets, kind), ratio, i, slot.Text); reason != "" {
				return &composition.Problem{ElementID: slot.ID, Line: 1, Reason: reason}
			}
		}
	}
	return nil
}

// regionDrawnRows is what an enabled region's active slots draw in the chosen
// preset: each slot's final text in slot order, a generated text its slot
// cannot fit left out while the draft keeps it (CDS-77). drawable is whether
// any row carries text, authored whether every drawn row is the owner's or an
// answer's, and owned whether the owner fixed any active slot.
func regionDrawnRows(region ProjectRegion, kind string, presets composition.DesignSelection, ratio string) (rows []string, drawable, authored, owned bool) {
	if !region.Enabled {
		return nil, false, false, false
	}
	rows, authored = make([]string, regionCapacity(kind, presets)), true
	for i := range rows[:min(len(rows), len(region.Slots))] {
		slot := region.Slots[i]
		owned = owned || slot.OwnerFixed
		if strings.TrimSpace(slot.Text) == "" || !authoredSlot(slot) && regionSlotFit(kind, RegionPresetID(presets, kind), ratio, i, slot.Text) != "" {
			continue
		}
		rows[i], drawable = slot.Text, true
		authored = authored && authoredSlot(slot)
	}
	return rows, drawable, authored, owned
}

// regionTiming is the interval a region's element keeps and whether the plan
// ever gave it one: the projected element's own, retained while the region
// draws nothing, then that of the first template entry the plan drew the region
// with, and otherwise the first 2.5 s or the last 3 s of the output (CLIP-66).
func regionTiming(portable PortablePlan, kind string) (start, end int, prior *PortableText) {
	valid := func(e composition.Element) bool {
		return regionKind(e.Role) == kind && e.Basis == regionBasis(kind) && e.StartMS != nil && e.EndMS != nil
	}
	for _, t := range slices.Concat(portable.Elements, portable.RetiredElements) {
		if t.Resolved.InstanceID == RegionElementID(kind) && valid(t.Resolved.Element) {
			return *t.Resolved.Element.StartMS, *t.Resolved.Element.EndMS, &t
		}
	}
	for _, t := range portable.Elements {
		if valid(t.Resolved.Element) {
			return *t.Resolved.Element.StartMS, *t.Resolved.Element.EndMS, &t
		}
	}
	if kind == "outro" {
		return -int(design.Timing.OutroDefaultS * 1000), 0, nil
	}
	return 0, int(design.Timing.IntroDefaultS * 1000), nil
}

// ProjectPlanRegions makes a plan draw exactly the project's regions
// (CLIP-147, CLIP-188): an enabled region with a drawable slot is one element
// whose rows are its active slots, and a region that is off or holds nothing
// drawable draws nothing (CDS-73). No template entry of either region survives
// beside it (CLIP-68). The footage, the captions and every other element stay
// as they are; changed is false for a plan that already drew the regions, which
// is then returned as it was.
func ProjectPlanRegions(plan EditPlan, r ProjectRegions, presets composition.DesignSelection) (_ EditPlan, changed bool, _ error) {
	if plan.Portable == nil {
		return plan, false, nil
	}
	portable := *plan.Portable
	elements, retired := portable.Elements, portable.RetiredElements
	for _, kind := range []string{"intro", "outro"} {
		start, end, prior := regionTiming(portable, kind)
		rows, drawable, authored, owned := regionDrawnRows(*r.Region(kind), kind, presets, plan.Ratio)
		id := RegionElementID(kind)
		e := composition.Element{ID: id, Kind: "ai", Role: regionRole(kind), Style: "auto", Position: "auto", Align: "center", Basis: regionBasis(kind), StartMS: &start, EndMS: &end}
		if authored {
			e.Kind = "fixed"
		}
		text := PortableText{OwnerEdited: owned, Resolved: composition.ResolvedElement{InstanceID: id, Element: e, AuthoredTiming: true}}
		for _, row := range rows {
			text.Resolved.Rows = append(text.Resolved.Rows, composition.ResolvedRow{Text: row})
		}
		if drawable {
			// An interval the output no longer holds stays the owner's to
			// correct rather than moved to fit it (CLIP-67).
			a, b, problem := composition.ResolveInterval(e, plan.DurationMS, 0, 0, 0)
			if problem != nil {
				// Named by the element the owner corrects it on.
				if prior != nil {
					problem.ElementID = prior.Resolved.Element.ID
				}
				return plan, false, problem
			}
			text.Resolved.StartMS, text.Resolved.EndMS = a, b
		} else if prior != nil && prior.Resolved.InstanceID == id {
			// Retired exactly as it was last drawn, so undoing its removal in ②
			// names the element the archive holds (CLIP-55).
			text = *prior
		} else if prior != nil {
			text.Resolved.StartMS, text.Resolved.EndMS = prior.Resolved.StartMS, prior.Resolved.EndMS
		}
		var next []PortableText
		placed := false
		for _, t := range elements {
			if regionKind(t.Resolved.Element.Role) != kind {
				next = append(next, t)
				continue
			}
			if !placed && drawable {
				next = append(next, text)
			}
			placed = true
		}
		if !placed && drawable {
			next = append(next, text)
		}
		elements = next
		retired = slices.DeleteFunc(slices.Clone(retired), func(t PortableText) bool { return regionKind(t.Resolved.Element.Role) == kind })
		// A region drawing nothing keeps its element retired, so the interval
		// the plan gave it returns with it (CLIP-66).
		if !drawable && prior != nil {
			at := slices.IndexFunc(retired, func(t PortableText) bool { return t.Resolved.InstanceID > id })
			if at < 0 {
				at = len(retired)
			}
			retired = slices.Insert(retired, at, text)
		}
	}
	same := func(a, b []PortableText) bool {
		return slices.EqualFunc(a, b, func(x, y PortableText) bool { return reflect.DeepEqual(x, y) })
	}
	if same(elements, portable.Elements) && same(retired, portable.RetiredElements) {
		return plan, false, nil
	}
	portable.Elements, portable.RetiredElements = elements, retired
	plan.Portable = &portable
	return plan, true, nil
}

// regionSlotNotices is one notice per slot of an enabled region holding words
// the video does not show: content past the chosen preset's slots (CLIP-147,
// CLIP-189) and a generated text its slot cannot fit (CDS-77). Each names its
// slot and is derived rather than stored, so the edit or the preset that
// removes its cause clears it.
func regionSlotNotices(region ProjectRegion, kind string, presets composition.DesignSelection, ratio string) []PlanNotice {
	if !region.Enabled {
		return nil
	}
	var out []PlanNotice
	capacity := regionCapacity(kind, presets)
	for i, slot := range region.Slots {
		reason := ""
		switch {
		case strings.TrimSpace(slot.Text) == "":
			continue
		case i >= capacity:
			reason = NoticeRegionLineSurplus
		case !authoredSlot(slot) && regionSlotFit(kind, RegionPresetID(presets, kind), ratio, i, slot.Text) != "":
			reason = kind + "_slot_omitted"
		default:
			continue
		}
		out = append(out, PlanNotice{CopyFallback: CopyFallback{ElementID: slot.ID, Reason: reason}, Action: "removal"})
	}
	return out
}

// RegionSlotNotices is every region slot's notice, intro first.
func RegionSlotNotices(r ProjectRegions, presets composition.DesignSelection, ratio string) []PlanNotice {
	return append(regionSlotNotices(r.Intro, "intro", presets, ratio), regionSlotNotices(r.Outro, "outro", presets, ratio)...)
}

// ProjectNotices is every notice a project shows: its plan's own and its
// regions' (CLIP-108, CLIP-147). A region its plan still draws through template
// entries is noticed from those entries, as it always was, until a write
// projects the project's slots into the plan.
func ProjectNotices(p Project) []PlanNotice {
	presets := p.DesignSelection().RegionPresets()
	var out []PlanNotice
	var plan *PortablePlan
	if p.EditPlan != "" {
		if decoded, err := DecodeEditPlan(p.EditPlan); err == nil {
			out, plan = ActivePlanNotices(decoded, presets), decoded.Portable
		}
	}
	r := EffectiveProjectRegions(p)
	for _, kind := range []string{"intro", "outro"} {
		if plan != nil {
			if _, present, projected := regionPlanLines(plan.Elements, kind); present && !projected {
				continue
			}
		}
		out = append(out, regionSlotNotices(*r.Region(kind), kind, presets, p.Ratio)...)
	}
	return out
}

// CorrectRegions is a correction's effect on the project's regions and the plan
// that draws them (CLIP-188): the region lines it changed become owner-fixed
// slot text, removing or restoring a region's entry switches the region, and
// the corrected plan then draws exactly the slots. old is the saved plan the
// correction was applied to. The regions come back only when the correction
// changed them, still carrying the revision they were read at, so saving them
// can refuse a concurrent region edit instead of overwriting it; a correction
// that changed none leaves the plan's region entries as the owner left them,
// an interval they were given included.
func CorrectRegions(p Project, old, corrected EditPlan) (EditPlan, *ProjectRegions, error) {
	if old.Portable == nil || corrected.Portable == nil {
		return corrected, nil, nil
	}
	current := EffectiveProjectRegions(p)
	regions := current.Clone()
	AbsorbWrittenRegions(&regions, old)
	if err := ReconcileRegionCorrection(&regions, *old.Portable, corrected.Portable.Elements); err != nil {
		return corrected, nil, err
	}
	presets := p.DesignSelection().RegionPresets()
	if err := ValidateOwnerRegions(regions, presets, p.Ratio); err != nil {
		return corrected, nil, err
	}
	if reflect.DeepEqual(regions, current) {
		return corrected, nil, nil
	}
	synced, _, err := ProjectPlanRegions(corrected, regions, presets)
	if err != nil {
		return corrected, nil, err
	}
	return synced, &regions, nil
}
