package clip

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// RegionDraft is the words a writing call left for one generated slot
// (CLIP-187), fitted to the slot once and addressed by the slot's id. Notice is
// the change the server made to fit them — the slot's shortened or omitted
// notice — and empty when they stand as written.
type RegionDraft struct {
	SlotID, Text string
	Notice       string `json:",omitempty"`
}

// RegionWritingSlot is one active slot of an enabled region as a writing call
// sees it (CLIP-186): its words when the owner fixed them or an answer or a
// template entry supplies them, and otherwise the instruction it is drafted
// from, with the bounds the chosen preset gives it (CDS-86).
type RegionWritingSlot struct {
	Kind  string
	Index int
	Slot  RegionSlot
	// Write is a slot the writer drafts: neither fixed by the owner nor bound.
	Write bool
	Spec  design.SlotSpec
	Width float64
	// The template row's own maximum it was seeded from (CLIP-116), 0 when none.
	Declared int
}

// RegionWritingSlots is every active slot of the enabled regions, intro first
// and in slot order: the slots the chosen presets draw, and no surplus draft
// beyond them (CLIP-147). A region that is off is not written (CLIP-187).
func RegionWritingSlots(r ProjectRegions, presets composition.DesignSelection, ratio, snapshot string, limits composition.Limits) []RegionWritingSlot {
	declared := map[string][]int{}
	if doc, problem := composition.ReadStored(snapshot, limits); problem == nil {
		for _, e := range doc.Elements {
			if regionKind(e.Role) == "" {
				continue
			}
			rows := []int{e.Chars}
			if len(e.Rows) > 0 {
				rows = rows[:0]
				for _, row := range e.Rows {
					rows = append(rows, row.Chars)
				}
			}
			declared[e.ID] = rows
		}
	}
	var out []RegionWritingSlot
	for _, kind := range []string{"intro", "outro"} {
		region := r.Region(kind)
		if !region.Enabled {
			continue
		}
		preset := RegionPresetID(presets, kind)
		for i, slot := range region.Slots[:min(regionCapacity(kind, presets), len(region.Slots))] {
			spec, width, ok := design.RegionSlotAt(kind, preset, ratio, i)
			if !ok {
				continue
			}
			w := RegionWritingSlot{Kind: kind, Index: i, Slot: slot, Write: !authoredSlot(slot), Spec: spec, Width: width}
			if rows := declared[slot.ElementID]; slot.ElementID != "" && slot.Row < len(rows) {
				w.Declared = rows[slot.Row]
			}
			out = append(out, w)
		}
	}
	return out
}

// Budget is how many syllables fit the slot at its floor, narrowed by the
// template row's own maximum (CLIP-116, CDS-86).
func (w RegionWritingSlot) Budget() int {
	budget := design.RegionSlotBudget(w.Spec, w.Width)
	if w.Declared > 0 && w.Declared < budget {
		return w.Declared
	}
	return budget
}

// Fits reports whether text is words this slot draws as written: one line of
// it at most per line the slot takes, inside its width at its floor, in a face
// that has every character, and within the row's own maximum.
func (w RegionWritingSlot) Fits(text string) bool {
	return strings.TrimSpace(text) != "" && !strings.ContainsAny(text, "\r\n") && !design.FitRegionSlot(w.Spec, text, w.Width).Over && (w.Declared <= 0 || design.Chars(text) <= w.Declared)
}

// NoticeShortened and NoticeOmitted are the reasons a region slot's draft names
// when its written words could not stand (CDS-77): a grounded shorter text took
// their place, or none fitted and the slot draws nothing.
func NoticeShortened(kind string) string { return kind + "_slot_shortened" }
func NoticeOmitted(kind string) string   { return kind + "_slot_omitted" }

// ApplyRegionDrafts writes a writing call's drafts into the generated slots of
// the enabled regions (CLIP-187). A slot the owner fixed or an answer binds, a
// region that is off, a slot past the chosen preset and an id no slot has take
// nothing, so a stale or stray draft can never replace the owner's words.
func ApplyRegionDrafts(r *ProjectRegions, drafts []RegionDraft, presets composition.DesignSelection) {
	for _, d := range drafts {
		for _, kind := range []string{"intro", "outro"} {
			region := r.Region(kind)
			for i := range region.Slots {
				slot := &region.Slots[i]
				if slot.ID != d.SlotID {
					continue
				}
				if region.Enabled && i < regionCapacity(kind, presets) && !authoredSlot(*slot) {
					slot.Text, slot.Notice = d.Text, d.Notice
				}
			}
		}
	}
}

// ValidateAuthoredRegions refuses, before any paid work, active slot text the
// owner fixed or an answer binds that its slot cannot draw, naming that slot
// (CDS-64, CDS-77): such words are never shortened or dropped to fit, so a
// generation would spend credits on a clip that cannot render them.
func ValidateAuthoredRegions(r ProjectRegions, presets composition.DesignSelection, ratio string) error {
	return validateRegions(r, presets, ratio, authoredSlot)
}

// WritingDigest binds a quote and a kept candidate plan to the region inputs a
// writing call reads (CLIP-69, CLIP-90): which regions are on, their presets and
// every active slot's instruction, words and ownership. It is empty while both
// regions are off, so a project with none keeps the digest it had.
func (r ProjectRegions) WritingDigest(presets composition.DesignSelection) string {
	type slot struct {
		ID, Instruction, Text string
		OwnerFixed, Bound     bool
	}
	type region struct {
		Preset string
		Slots  []slot
	}
	regions := map[string]region{}
	for _, kind := range []string{"intro", "outro"} {
		value := r.Region(kind)
		if !value.Enabled {
			continue
		}
		out := region{Preset: RegionPresetID(presets, kind), Slots: []slot{}}
		for _, s := range value.Slots[:min(regionCapacity(kind, presets), len(value.Slots))] {
			out.Slots = append(out.Slots, slot{s.ID, s.Instruction, s.Text, s.OwnerFixed, len(s.Binding) > 0})
		}
		regions[kind] = out
	}
	if len(regions) == 0 {
		return ""
	}
	raw, _ := json.Marshal(regions)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// WrittenRegions is the regions a writing job leaves (CLIP-187): the project's
// slots as they stand, the words its stored plan still holds in template
// entries carried into the generated slots first (CLIP-190), then the drafts
// the job's calls wrote. The job and the save that commits it both compute it,
// so the plan the job lays out is the plan the save stores.
func WrittenRegions(p Project, drafts []RegionDraft) ProjectRegions {
	r := EffectiveProjectRegions(p)
	if stored, err := DecodeEditPlan(p.EditPlan); p.EditPlan != "" && err == nil {
		AbsorbWrittenRegions(&r, stored)
	}
	ApplyRegionDrafts(&r, drafts, p.DesignSelection().RegionPresets())
	return r
}
