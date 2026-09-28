package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

type regionSlotJSON struct {
	ID, Instruction, Text, ElementID string
	InstructionEdited, OwnerFixed    bool
	Binding                          []composition.Part
	Row                              int
	Notice                           string `json:",omitempty"`
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
			out.Slots = append(out.Slots, regionSlotJSON{s.ID, s.Instruction, s.Text, s.ElementID, s.InstructionEdited, s.OwnerFixed, s.Binding, s.Row, s.Notice})
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
			out.Slots = append(out.Slots, clip.RegionSlot{ID: s.ID, Instruction: s.Instruction, Text: s.Text, ElementID: s.ElementID, InstructionEdited: s.InstructionEdited, OwnerFixed: s.OwnerFixed, Binding: s.Binding, Row: s.Row, Notice: s.Notice})
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

// syncPlanRegions keeps a project's plan drawing exactly its regions after a
// write that changed them or their presets (CLIP-188), inside that write's
// transaction and from the plan as it stands there, so a concurrent correction
// is never overwritten. The plan revision advances only when what the plan
// holds changed, which makes a matching render stale (CLIP-139), and an
// owner-fixed text its slot cannot draw refuses the whole write (CDS-64). A
// write that left both the regions and the presets as they were — ①'s autosave
// of its other fields — touches no plan.
func syncPlanRegions(ctx context.Context, q *sqlc.Queries, before clip.Project, now time.Time) error {
	user, id := before.UserID, before.ID
	p, err := getProject(ctx, q, user, id)
	if err != nil {
		return err
	}
	regions := clip.EffectiveProjectRegions(p)
	presets := p.DesignSelection().RegionPresets()
	if reflect.DeepEqual(regions, clip.EffectiveProjectRegions(before)) && presets == before.DesignSelection().RegionPresets() {
		return nil
	}
	if err := clip.ValidateOwnerRegions(regions, presets, p.Ratio); err != nil {
		return err
	}
	// A plan this version cannot read has nothing to project into; every
	// reader of it refuses it on its own.
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if p.EditPlan == "" || err != nil {
		return nil
	}
	synced, changed, err := clip.ProjectPlanRegions(plan, regions, presets)
	if err != nil || !changed {
		return err
	}
	raw, err := clip.EncodeEditPlan(synced)
	if err != nil {
		return err
	}
	n, err := q.SaveCorrection(ctx, sqlc.SaveCorrectionParams{EditPlanJson: nullable(raw), UpdatedAt: stamp(now), ID: id, UserID: user, EditPlanRevision: int64(p.EditPlanRevision)})
	if err != nil {
		return err
	}
	if n != 1 {
		return clip.ErrPlanConflict
	}
	return saveRegionState(ctx, q, p, regions, now, true)
}

// saveRegionState saves region state computed from the revision it carries: a
// region edit made since refuses it rather than being overwritten, and changed
// state advances the revision. State equal to what is stored writes nothing and
// keeps the revision, unless pin asks for it to be stored anyway — once a plan
// has been projected from regions that were only ever derived from it, they are
// recorded, since the projected element no longer says which template entry or
// answer each slot came from (CLIP-190).
func saveRegionState(ctx context.Context, q *sqlc.Queries, before clip.Project, next clip.ProjectRegions, now time.Time, pin bool) error {
	current := clip.EffectiveProjectRegions(before)
	if next.Revision != current.Revision {
		return clip.ErrPlanConflict
	}
	next = next.Clone()
	if !reflect.DeepEqual(next, current) {
		next.Revision++
	} else if !pin {
		return nil
	}
	updated := before
	updated.Regions, updated.UpdatedAt = &next, now
	return saveRegions(ctx, q, updated)
}

// projectWrittenPlan is a writer's plan as it is saved (CLIP-187): the plan
// draws the regions WrittenRegions leaves — the owner's words kept, the drafts
// in the generated slots — so a region that is off draws nothing and an enabled
// one draws without a template entry. A revision passes no drafts: it rewrites
// no region word (CLIP-131). It returns the plan to save, the regions at the
// revision they were read at, and whether the projection rewrote the plan.
func projectWrittenPlan(p clip.Project, raw string, drafts []clip.RegionDraft) (string, clip.ProjectRegions, bool, error) {
	regions := clip.WrittenRegions(p, drafts)
	plan, err := clip.DecodeEditPlan(raw)
	if err != nil || plan.Portable == nil {
		return raw, regions, false, nil
	}
	synced, changed, err := clip.ProjectPlanRegions(plan, regions, p.DesignSelection().RegionPresets())
	if err != nil || !changed {
		return raw, regions, false, err
	}
	raw, err = clip.EncodeEditPlan(synced)
	return raw, regions, true, err
}
