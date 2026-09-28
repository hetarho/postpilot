package ai

import (
	"github.com/postpilot/backend/internal/clip"
)

type storylineJSON struct {
	Storyline   []flowStorylineJSON `json:"storyline"`
	RegionSlots []regionSlotJSON    `json:"region_slots"`
}

var storylineShape = readShape(storylineSchema)

// parseStoryline reads the storyline call's answer (CLIP-177): the body is what the call is
// for, so one that keeps nothing after its bounds is bad output and goes to the correction
// like any other — there is nothing else the job could keep. The slot words it drafted ride
// with it (CLIP-187).
func parseStoryline(cfg Config, input clip.StorylineInput, raw string) (clip.Storyline, error) {
	var wire storylineJSON
	if err := decode(raw, cfg.MaxResponseBytes, storylineShape, &wire); err != nil {
		return clip.Storyline{}, err
	}
	paragraphs := make([]clip.StorylineParagraph, 0, len(wire.Storyline))
	for _, p := range wire.Storyline {
		paragraphs = append(paragraphs, clip.StorylineParagraph{Text: p.Text, ObservationIDs: p.ObservationIDs})
	}
	kept := clip.BoundStoryline(paragraphs, clip.ObservedScenes(input.Analyses))
	if len(kept) == 0 {
		return clip.Storyline{}, outputError("storyline_empty")
	}
	// The generated slots' words, fitted once and carried to the save (CLIP-187).
	drafts := regionDrafts(input.PlanningInput, wire.RegionSlots, compositionLimits(cfg, input.PlanningInput))
	return clip.Storyline{Paragraphs: kept, RegionDrafts: drafts}, nil
}
