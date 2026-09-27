package ai

import (
	"github.com/postpilot/backend/internal/clip"
)

type storylineJSON struct {
	Storyline []flowStorylineJSON `json:"storyline"`
}

var storylineShape = readShape(storylineSchema)

// parseStoryline reads the storyline call's answer (CLIP-177): the storyline is the whole
// output, so one that keeps nothing after its bounds is bad output and goes to the correction
// like any other — there is nothing else the job could keep.
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
	return clip.Storyline{Paragraphs: kept}, nil
}
