package ai

import (
	"math"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// composeTimeline compiles model timing hints into an executable timeline. The
// model selects footage and copy; arithmetic must not require a second paid call.
// This boundary is used only for fresh AI output, never persisted/manual plans.
func composeTimeline(cfg Config, in clip.PlanningInput, plan *clip.EditPlan) error {
	if plan.Ratio != in.Ratio {
		return outputError("plan_ratio")
	}
	// The user's frozen target is authority, not the model's redundant sum.
	if len(plan.Cuts) == 0 || len(plan.Cuts) > cfg.Render.MaxCuts {
		return outputError("plan_cut_count")
	}
	analyses := map[string]clip.SourceAnalysis{}
	for _, a := range in.Analyses {
		analyses[a.Source.ID] = a
	}
	total := 0
	seen := map[string]bool{}
	for i := range plan.Cuts {
		c := &plan.Cuts[i]
		// Identity, focal point and gain are checked HERE, before the design
		// system measures anything: a plan the model got wrong must not cost a
		// single glyph measurement.
		if strings.TrimSpace(c.ID) == "" || seen[c.ID] {
			return outputError("plan_cut_identity")
		}
		seen[c.ID] = true
		if !clip.Normalized(c.Focal.X) || !clip.Normalized(c.Focal.Y) {
			return outputError("plan_focal")
		}
		if !clip.Normalized(c.OriginalVolume()) {
			return outputError("plan_volume")
		}
		a, ok := analyses[c.SourceID]
		if !ok || c.Fingerprint != a.Source.Fingerprint {
			return outputError("plan_source")
		}
		// Check authoritative bounds before subtraction, accumulation or resizing.
		if c.StartMS < 0 || c.EndMS <= c.StartMS || c.EndMS > a.Source.Info.DurationMS {
			return outputError("plan_cut_range")
		}
		length := c.EndMS - c.StartMS
		if length <= 2*cfg.Render.FadeMS {
			return outputError("plan_cut_fade")
		}
		if c.Copy.StartMS < 0 || c.Copy.EndMS <= c.Copy.StartMS || c.Copy.StartMS >= length {
			return outputError("plan_caption_time")
		}
		// A caption cannot be exposed beyond the selected footage. Do not infer
		// absolute/source timestamps, move its start, or accept an empty exposure.
		c.Copy.EndMS = min(c.Copy.EndMS, length)
	}
	// CDS-37's length bounds and CDS-36's transitions are the caller's, not the
	// model's: the scene the observer reported decides both, so the same input
	// joins the same cuts the same way (CDS-7).
	scenes := make([]string, len(plan.Cuts))
	for i, c := range plan.Cuts {
		scenes[i], _ = clip.CutScene(c, analyses[c.SourceID])
	}
	if err := holdCutLengths(plan, analyses, scenes); err != nil {
		return err
	}
	for i, ms := range design.Transitions(scenes) {
		plan.Cuts[i].TransitionMS = ms
	}
	for _, c := range plan.Cuts {
		total += c.EndMS - c.StartMS
	}
	total -= plan.TransitionTotal()
	if total >= cfg.Render.MinDurationMS && total <= cfg.Render.MaxDurationMS && math.Abs(float64(total)-float64(in.TargetDurationMS)) <= float64(cfg.TargetToleranceMS) {
		plan.DurationMS = total
		return nil
	}
	remaining := in.TargetDurationMS - total
	grow := remaining > 0
	if !grow {
		remaining = -remaining
	}
	room := make([]int, len(plan.Cuts))
	for i, c := range plan.Cuts {
		minimum, _ := design.CutBounds(scenes[i])
		if grow {
			room[i] = cutCeiling(plan, analyses, scenes, i) - c.EndMS
		} else {
			// Keep every selected cut, its own CDS-37 floor, the transitions
			// that eat into it and a positive caption exposure.
			room[i] = c.EndMS - c.StartMS - max(2*cfg.Render.FadeMS+1, c.Copy.StartMS+1, minimum)
		}
	}
	// Distribute the adjustment evenly, redistributing when a scene reaches
	// its bound. Every pass finishes or exhausts a cut; work is bounded by cuts².
	for remaining > 0 {
		active := 0
		for _, available := range room {
			if available > 0 {
				active++
			}
		}
		if active == 0 {
			return outputError("plan_timeline")
		}
		share := (remaining + active - 1) / active
		for i := range plan.Cuts {
			amount := min(room[i], share, remaining)
			if grow {
				plan.Cuts[i].EndMS += amount
			} else {
				plan.Cuts[i].EndMS -= amount
			}
			room[i] -= amount
			remaining -= amount
		}
	}
	for i := range plan.Cuts {
		c := &plan.Cuts[i]
		c.Copy.EndMS = min(c.Copy.EndMS, c.EndMS-c.StartMS)
	}
	plan.DurationMS = in.TargetDurationMS
	return nil
}

// cutCeiling is the furthest a cut's end may move: inside the segment it was
// observed in, never through an unobserved gap or into the next already-selected
// range of the same source, and never past CDS-37's ceiling for its scene.
func cutCeiling(plan *clip.EditPlan, analyses map[string]clip.SourceAnalysis, scenes []string, i int) int {
	c := plan.Cuts[i]
	end := c.EndMS
	for _, segment := range analyses[c.SourceID].Segments {
		if segment.StartMS <= c.StartMS && segment.EndMS >= c.EndMS {
			end = segment.EndMS
			break
		}
	}
	for j, other := range plan.Cuts {
		if i == j || other.SourceID != c.SourceID {
			continue
		}
		if other.StartMS >= c.EndMS {
			end = min(end, other.StartMS)
		} else if other.EndMS > c.StartMS {
			// Existing explicit range reuse stays as-is, not expanded.
			end = c.EndMS
		}
	}
	_, maximum := design.CutBounds(scenes[i])
	return min(end, c.StartMS+maximum)
}

// cutFloor is the earliest a cut's start may move: the segment it was observed
// in, and never back over the previous already-selected range of that source.
func cutFloor(plan *clip.EditPlan, analyses map[string]clip.SourceAnalysis, i int) int {
	c := plan.Cuts[i]
	start := c.StartMS
	for _, segment := range analyses[c.SourceID].Segments {
		if segment.StartMS <= c.StartMS && segment.EndMS >= c.EndMS {
			start = segment.StartMS
			break
		}
	}
	for j, other := range plan.Cuts {
		if i == j || other.SourceID != c.SourceID {
			continue
		}
		if other.EndMS <= c.StartMS {
			start = max(start, other.EndMS)
		} else if other.StartMS < c.EndMS {
			start = c.StartMS
		}
	}
	return max(0, start)
}

// holdCutLengths applies CDS-37: a cut over its scene's ceiling is trimmed to
// it, and one under 1.2 s takes the room its own segment still has — forward
// first, then backward, because moving the end keeps the cut's opening frame.
// A cut with no room left in either direction is footage too short to read, and
// the plan is refused rather than shipped under the floor.
func holdCutLengths(plan *clip.EditPlan, analyses map[string]clip.SourceAnalysis, scenes []string) error {
	for i := range plan.Cuts {
		c := &plan.Cuts[i]
		minimum, maximum := design.CutBounds(scenes[i])
		if c.EndMS-c.StartMS > maximum {
			c.EndMS = c.StartMS + maximum
		}
		if c.EndMS-c.StartMS < minimum {
			c.EndMS = min(c.StartMS+minimum, cutCeiling(plan, analyses, scenes, i))
			if c.EndMS-c.StartMS < minimum {
				c.StartMS = max(c.EndMS-minimum, cutFloor(plan, analyses, i))
			}
			if c.EndMS-c.StartMS < minimum {
				return outputError("plan_cut_length")
			}
		}
		// Whichever end moved, the caption still lives inside the cut.
		length := c.EndMS - c.StartMS
		c.Copy.StartMS = min(c.Copy.StartMS, length-1)
		c.Copy.EndMS = min(c.Copy.EndMS, length)
	}
	return nil
}
