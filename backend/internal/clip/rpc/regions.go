package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func regionsProto(p clip.Project) *v1.ClipProjectRegions {
	r := clip.EffectiveProjectRegions(p)
	region := func(r clip.ProjectRegion) *v1.ClipProjectRegion {
		out := &v1.ClipProjectRegion{Enabled: r.Enabled}
		for _, s := range r.Slots {
			out.Slots = append(out.Slots, &v1.ClipRegionSlot{Id: s.ID, Instruction: s.Instruction, Text: s.Text, InstructionEdited: s.InstructionEdited, OwnerFixed: s.OwnerFixed, Bound: len(s.Binding) > 0})
		}
		return out
	}
	return &v1.ClipProjectRegions{Revision: int32(r.Revision), Intro: region(r.Intro), Outro: region(r.Outro)}
}
func regionEdit(m *v1.ClipRegionEdit) *clip.RegionPatch {
	if m == nil {
		return nil
	}
	p := &clip.RegionPatch{Enabled: m.Enabled}
	for _, s := range m.Slots {
		p.Slots = append(p.Slots, clip.RegionSlotPatch{ID: s.Id, Instruction: s.Instruction, Text: s.Text})
	}
	return p
}
