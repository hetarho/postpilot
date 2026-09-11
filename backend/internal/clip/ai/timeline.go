package ai

import (
	"math"

	"github.com/postpilot/backend/internal/clip"
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
	total := -cfg.Render.FadeMS * (len(plan.Cuts) - 1)
	for i := range plan.Cuts {
		c := &plan.Cuts[i]
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
		total += length
	}
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
		if grow {
			// Extend forward only within the SAME observed scene, never through
			// an unobserved gap or into the next already-selected source range.
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
			room[i] = end - c.EndMS
		} else {
			// Keep every selected cut, the fade and a positive caption exposure.
			room[i] = c.EndMS - c.StartMS - max(2*cfg.Render.FadeMS+1, c.Copy.StartMS+1)
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
