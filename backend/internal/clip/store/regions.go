package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

type regionSlotJSON struct {
	ID, Instruction, Text, ElementID string
	InstructionEdited, OwnerFixed    bool
	Binding                          []composition.Part
	Row                              int
}
type regionJSON struct {
	Enabled bool
	Slots   []regionSlotJSON
}
type regionsJSON struct {
	Version, Revision int
	Intro, Outro      regionJSON
}

func encodeRegions(r *clip.ProjectRegions) (string, error) {
	if r == nil {
		return "", nil
	}
	region := func(r clip.ProjectRegion) regionJSON {
		out := regionJSON{Enabled: r.Enabled}
		for _, s := range r.Slots {
			out.Slots = append(out.Slots, regionSlotJSON{s.ID, s.Instruction, s.Text, s.ElementID, s.InstructionEdited, s.OwnerFixed, s.Binding, s.Row})
		}
		return out
	}
	raw, err := json.Marshal(regionsJSON{1, r.Revision, region(r.Intro), region(r.Outro)})
	return string(raw), err
}
func decodeRegions(raw string) (*clip.ProjectRegions, error) {
	if raw == "" {
		return nil, nil
	}
	var wire regionsJSON
	if err := strictJSON(raw, &wire); err != nil || wire.Version != 1 || wire.Revision < 0 {
		return nil, fmt.Errorf("invalid stored clip regions")
	}
	region := func(r regionJSON) clip.ProjectRegion {
		out := clip.ProjectRegion{Enabled: r.Enabled}
		for _, s := range r.Slots {
			out.Slots = append(out.Slots, clip.RegionSlot{ID: s.ID, Instruction: s.Instruction, Text: s.Text, ElementID: s.ElementID, InstructionEdited: s.InstructionEdited, OwnerFixed: s.OwnerFixed, Binding: s.Binding, Row: s.Row})
		}
		return out
	}
	return &clip.ProjectRegions{Revision: wire.Revision, Intro: region(wire.Intro), Outro: region(wire.Outro)}, nil
}
func saveRegions(ctx context.Context, q *sqlc.Queries, p clip.Project) error {
	if p.Regions == nil {
		return nil
	}
	raw, err := encodeRegions(p.Regions)
	if err != nil {
		return err
	}
	return affected(q.SaveClipRegions(ctx, sqlc.SaveClipRegionsParams{ID: p.ID, UserID: p.UserID, RegionsJson: nullable(raw), UpdatedAt: stamp(p.UpdatedAt)}))
}
