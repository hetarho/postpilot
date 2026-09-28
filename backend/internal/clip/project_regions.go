package clip

import (
	"slices"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// ProjectRegions is the one project-owned slot draft. Revision guards edits even
// before a plan exists. Presets remain in Project's existing design selection.
type ProjectRegions struct {
	Revision     int
	Intro, Outro ProjectRegion
}
type ProjectRegion struct {
	Enabled bool
	Slots   []RegionSlot
}
type RegionSlot struct {
	ID, Instruction, Text         string
	InstructionEdited, OwnerFixed bool
	Binding                       []composition.Part
	ElementID                     string
	Row                           int
}
type RegionPatch struct {
	Enabled *bool
	Slots   []RegionSlotPatch
}
type RegionSlotPatch struct {
	ID                string
	Instruction, Text *string
}

func RegionSlotID(kind string, index int) string {
	return "project-" + kind + "-" + strconv.Itoa(index+1)
}

func (r ProjectRegions) Clone() ProjectRegions {
	clone := func(region ProjectRegion) ProjectRegion {
		region.Slots = slices.Clone(region.Slots)
		for i := range region.Slots {
			region.Slots[i].Binding = slices.Clone(region.Slots[i].Binding)
		}
		return region
	}
	r.Intro, r.Outro = clone(r.Intro), clone(r.Outro)
	return r
}

func (r *ProjectRegions) Region(kind string) *ProjectRegion {
	if kind == "outro" {
		return &r.Outro
	}
	return &r.Intro
}

// EnsureRegionSlots retains surplus drafts and creates only newly exposed slots.
func EnsureRegionSlots(r *ProjectRegions, p Project) {
	presets := p.DesignSelection().RegionPresets()
	for _, kind := range []string{"intro", "outro"} {
		preset, ok := design.Region(kind, RegionPresetID(presets, kind))
		if !ok {
			continue
		}
		region := r.Region(kind)
		for len(region.Slots) < len(preset.Slots()) {
			region.Slots = append(region.Slots, RegionSlot{ID: RegionSlotID(kind, len(region.Slots))})
		}
	}
}

func regionParts(parts []composition.Part, values map[string]string) string {
	var out strings.Builder
	for _, part := range parts {
		out.WriteString(part.Literal)
		if part.Field != "" {
			out.WriteString(values[part.Field])
		}
	}
	return out.String()
}

// SeedProjectRegions copies retained template entries without requiring answers
// to be complete yet. Owner-edited instructions and exact text win on reseeding.
func SeedProjectRegions(p Project, previous *ProjectRegions) (ProjectRegions, error) {
	r := ProjectRegions{}
	if previous != nil {
		r = previous.Clone()
	}
	seed := ProjectRegions{}
	if p.Composition != nil {
		d, err := composition.ReadStored(p.Composition.Snapshot.Body, DefaultCompositionLimits())
		if err != nil {
			return r, err
		}
		for _, e := range d.Elements {
			kind := regionKind(e.Role)
			if kind == "" {
				continue
			}
			region := seed.Region(kind)
			region.Enabled = true
			rows := e.Rows
			if len(rows) == 0 {
				rows = []composition.Row{{Kind: e.Kind, Parts: e.Parts}}
			}
			for row, item := range rows {
				slot := RegionSlot{ID: RegionSlotID(kind, len(region.Slots)), ElementID: e.ID, Row: row}
				if composition.RowKind(e, item) == "ai" {
					slot.Instruction = regionParts(item.Parts, p.Composition.Inputs.Values)
				} else {
					slot.Text = regionParts(item.Parts, p.Composition.Inputs.Values)
					slot.Binding = slices.Clone(item.Parts)
					if len(slot.Binding) == 0 {
						slot.Binding = []composition.Part{{}}
					}
				}
				region.Slots = append(region.Slots, slot)
			}
		}
	}
	for _, kind := range []string{"intro", "outro"} {
		old, fresh := r.Region(kind), seed.Region(kind)
		old.Enabled = fresh.Enabled
		for i, slot := range fresh.Slots {
			if i >= len(old.Slots) {
				old.Slots = append(old.Slots, slot)
				continue
			}
			if old.Slots[i].OwnerFixed || old.Slots[i].InstructionEdited {
				continue
			}
			old.Slots[i] = slot
		}
	}
	EnsureRegionSlots(&r, p)
	return r, nil
}

// EffectiveProjectRegions derives missing metadata from retained content only.
// A saved plan is the authority for presence; a preset alone enables nothing.
func EffectiveProjectRegions(p Project) ProjectRegions {
	if p.Regions != nil {
		return p.Regions.Clone()
	}
	r, _ := SeedProjectRegions(p, nil)
	if p.EditPlan != "" {
		seed := r
		r = ProjectRegions{}
		if plan, err := DecodeEditPlan(p.EditPlan); err == nil && plan.Portable != nil {
			retained := p
			retained.Composition = &ProjectComposition{Snapshot: plan.Portable.Snapshot, Inputs: plan.Portable.Inputs}
			seed, _ = SeedProjectRegions(retained, nil)
			for _, text := range plan.Portable.Elements {
				kind := regionKind(text.Resolved.Element.Role)
				if kind == "" {
					continue
				}
				region := r.Region(kind)
				region.Enabled = true
				for row, words := range regionRowTexts(text.Resolved) {
					slot := RegionSlot{ID: RegionSlotID(kind, len(region.Slots)), Text: words, OwnerFixed: text.OwnerEdited, ElementID: text.Resolved.Element.ID, Row: row}
					for _, prior := range seed.Region(kind).Slots {
						if prior.ElementID == slot.ElementID && prior.Row == row {
							slot.Instruction, slot.Binding = prior.Instruction, prior.Binding
							break
						}
					}
					if slot.OwnerFixed {
						slot.Binding = nil
					} else if len(slot.Binding) == 0 && (text.Authored || text.Resolved.Element.Kind != "ai") {
						slot.OwnerFixed = true
					}
					region.Slots = append(region.Slots, slot)
				}
			}
		}
	}
	EnsureRegionSlots(&r, p)
	return r
}

func ApplyRegionPatch(region *ProjectRegion, patch *RegionPatch, limits Limits) error {
	if patch == nil {
		return nil
	}
	if patch.Enabled != nil {
		region.Enabled = *patch.Enabled
	}
	seen := map[string]bool{}
	for _, edit := range patch.Slots {
		i := slices.IndexFunc(region.Slots, func(s RegionSlot) bool { return s.ID == edit.ID })
		if i < 0 || seen[edit.ID] {
			return ErrInvalid
		}
		seen[edit.ID] = true
		slot := &region.Slots[i]
		if edit.Instruction != nil {
			if !BoundedText(*edit.Instruction, 0, limits.InstructionChars) {
				return ErrInvalid
			}
			slot.Instruction, slot.InstructionEdited = *edit.Instruction, true
		}
		if edit.Text != nil {
			if !BoundedText(*edit.Text, 0, limits.Composition.CopyChars) {
				return ErrInvalid
			}
			slot.Text, slot.OwnerFixed, slot.Binding = *edit.Text, true, nil
		}
	}
	return nil
}

func RefreshRegionBindings(r *ProjectRegions, inputs CompositionInputs) {
	for _, region := range []*ProjectRegion{&r.Intro, &r.Outro} {
		for i := range region.Slots {
			slot := &region.Slots[i]
			if len(slot.Binding) > 0 && !slot.OwnerFixed {
				slot.Text = regionParts(slot.Binding, inputs.Values)
			}
		}
	}
}
