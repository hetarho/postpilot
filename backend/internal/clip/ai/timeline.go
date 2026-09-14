package ai

import (
	"math"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// composeTimeline compiles model timing hints into an executable timeline. The
// model selects footage and copy; arithmetic must not require a second paid call.
// This boundary is used only for fresh AI output, never persisted/manual plans.
func composeTimeline(cfg Config, in clip.PlanningInput, plan *clip.EditPlan) (err error) {
	phase := "selection"
	values := map[string]int{"target_ms": in.TargetDurationMS, "cut_count": len(plan.Cuts), "min_ms": cfg.Render.MinDurationMS, "max_ms": cfg.Render.MaxDurationMS}
	defer func() {
		if err != nil {
			check := "plan_timeline"
			if code, ok := err.(interface{ OutputValidationCode() string }); ok {
				check = code.OutputValidationCode()
			}
			err = clip.WithAttemptDiagnostic(err, clip.AttemptDiagnostic{Check: check, Phase: phase, Values: values, Ranges: clip.AttemptRangeDiagnostics(*plan, in.Analyses)})
		}
	}()
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
		values["cut"] = i + 1
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
		// Exactly one fixed rate per cut, and only one this SOURCE can actually
		// be played at: a slow rate needs verified original cadence, and an
		// unsupported one is named rather than replaced by 1x (CDS-68, CLIP-99).
		if !slices.Contains(clip.AllowedPlaybackRates(a.Source.Info), c.Rate()) {
			values["rate_permille"] = c.Rate()
			return outputError("plan_cut_rate")
		}
		// One cut comes from ONE observed scene. Two scenes that merely touch in
		// time are still two scenes, and a selection spanning them answers for
		// neither (CLIP-7, CLIP-98).
		scene, contained := clip.ContainedScene(in.Analyses, *c)
		if !contained {
			return outputError("plan_cut_scene")
		}
		if !clip.SelectableScene(scene, c.Rate()) {
			return outputError("plan_cut_usability")
		}
		// Every length below is measured on the transformed OUTPUT timeline,
		// which is what the viewer sees and what every caption resolves against
		// (CDS-62). The source span keeps its own original timestamps.
		length := c.OutputDurationMS()
		if length <= 0 {
			return outputError("plan_cut_rate")
		}
		if length <= 2*cfg.Render.FadeMS {
			return outputError("plan_cut_fade")
		}
		// The model writes one caption per cut; CDS-43's second one is the
		// compiler's, and is placed after this timeline is settled.
		for j := range c.Copies {
			copy := &c.Copies[j]
			if copy.StartMS < 0 || copy.EndMS <= copy.StartMS || copy.StartMS >= length {
				return outputError("plan_caption_time")
			}
			// A caption cannot be exposed beyond the selected footage. Do not
			// infer absolute/source timestamps, move its start, or accept an
			// empty exposure.
			copy.EndMS = min(copy.EndMS, length)
		}
	}
	delete(values, "cut")
	delete(values, "rate_permille")
	// The same footage may be split into several cuts, but never selected twice:
	// a fresh plan inherits no grandfathered overlap (CLIP-98).
	if clip.ValidateSourceRanges(*plan, nil) != nil {
		// The same rule, stated in the WRITER's own vocabulary: an overlap in a
		// generated plan is bad model output and earns a bounded correction,
		// while that rule refuses an owner's edit as an invalid request.
		return outputError("plan_source_overlap")
	}
	// CDS-37's length targets and CDS-36's transitions are the caller's, not the
	// model's: the scene the observer reported and the template's preset decide
	// both, so the same input joins the same cuts the same way (CDS-7).
	preset := in.Template.Preset
	authoredRhythm := false
	if nativeComposition(in) {
		preset = ""
		doc, problem := composition.Parse(in.Composition.Snapshot.Body, cfg.Template.Composition)
		if problem != nil {
			return problem
		}
		authoredRhythm = len(doc.Guidance) > 0
		for _, section := range doc.Sections {
			authoredRhythm = authoredRhythm || len(section.Guidance) > 0
		}
	}
	scenes := make([]string, len(plan.Cuts))
	for i, c := range plan.Cuts {
		scenes[i], _ = clip.CutScene(c, analyses[c.SourceID])
	}
	if !authoredRhythm {
		holdCutLengths(plan, analyses, scenes, preset)
	}
	for i, ms := range design.Transitions(scenes) {
		plan.Cuts[i].TransitionMS = ms
	}
	for _, c := range plan.Cuts {
		total += c.OutputDurationMS()
	}
	total -= plan.TransitionTotal()
	values["before_ms"], values["transition_ms"] = total, plan.TransitionTotal()
	if total >= cfg.Render.MinDurationMS && total <= cfg.Render.MaxDurationMS && math.Abs(float64(total)-float64(in.TargetDurationMS)) <= float64(cfg.TargetToleranceMS) {
		plan.DurationMS = total
		return nil
	}
	remaining := in.TargetDurationMS - total
	grow := remaining > 0
	phase = "timeline_grow"
	if !grow {
		phase = "timeline_shrink"
	}
	if !grow {
		remaining = -remaining
	}
	// Two passes. The first keeps every cut inside its CDS-37 target; when no
	// cut has room left there, the target yields — it is rhythm, not
	// correctness — and the second pass uses the bounds that actually hold:
	// the observed footage when growing, the transitions and the copy's start
	// when shrinking (the existing plan_cut_fade bound). A reachable timeline is
	// never refused for a target (CDS-37 r3).
	for _, hard := range []bool{false, true} {
		if authoredRhythm && !hard {
			continue
		}
		room := make([]int, len(plan.Cuts))
		for i, c := range plan.Cuts {
			minimum, _ := design.CutBounds(scenes[i], preset)
			if grow {
				// The room is what the extra SOURCE footage is worth on the
				// output timeline, which is what the target is measured in.
				ceiling := cutCeiling(plan, analyses, scenes, preset, i, hard)
				room[i] = outputSpan(ceiling-c.StartMS, c.Rate()) - c.OutputDurationMS()
			} else {
				floor := max(2*cfg.Render.FadeMS+1, c.FirstCopy().StartMS+1)
				if !hard {
					floor = max(floor, minimum)
				}
				room[i] = c.OutputDurationMS() - floor
			}
		}
		// Distribute the adjustment evenly, redistributing when a cut reaches
		// its bound. Every pass finishes or exhausts a cut; work is bounded by cuts².
		for remaining > 0 {
			active := 0
			for _, available := range room {
				if available > 0 {
					active++
				}
			}
			if active == 0 {
				break
			}
			share := (remaining + active - 1) / active
			for i := range plan.Cuts {
				c := &plan.Cuts[i]
				amount := max(0, min(room[i], share, remaining))
				// The share is an OUTPUT delta; convert it back to the source
				// footage that produces it before moving the selected range.
				output := c.OutputDurationMS()
				if grow {
					output += amount
				} else {
					output -= amount
				}
				c.EndMS = c.StartMS + sourceSpan(output, c.Rate())
				room[i] -= amount
				remaining -= amount
			}
		}
		if remaining == 0 {
			break
		}
	}
	// Authored rhythm skips holdCutLengths, so end-only growth used to reject
	// plans with enough observed footage BEFORE the selected start. Exhaust both
	// directions without crossing a gap or a neighboring selected range.
	if grow && remaining > 0 {
		for i := range plan.Cuts {
			c := &plan.Cuts[i]
			floor := backwardSceneFloor(plan, analyses, i)
			amount := max(0, min(remaining, outputSpan(c.StartMS-floor, c.Rate())))
			c.StartMS = max(floor, c.StartMS-sourceSpan(amount, c.Rate()))
			for j := range c.Copies {
				c.Copies[j].StartMS += amount
				c.Copies[j].EndMS += amount
			}
			remaining -= amount
			values["backward_ms"] += amount
		}
	}
	values["remaining_ms"] = remaining
	values["after_ms"] = 0
	for _, c := range plan.Cuts {
		values["after_ms"] += c.OutputDurationMS()
	}
	values["after_ms"] -= plan.TransitionTotal()
	for i := range plan.Cuts {
		c := &plan.Cuts[i]
		for j := range c.Copies {
			c.Copies[j].EndMS = min(c.Copies[j].EndMS, c.OutputDurationMS())
		}
	}
	if remaining == 0 {
		plan.DurationMS = in.TargetDurationMS
		return nil
	}
	if !grow {
		// Even at the fade floor the cuts hold more than the target: the model
		// picked footage that cannot be trimmed to it.
		return outputError("plan_timeline")
	}
	// Every cut sits at the end of its observed footage. The target is the
	// owner's, but the footage is what it is: a clip the length floor accepts
	// ships at the length the footage holds rather than being refused for the
	// seconds it cannot have.
	achieved := in.TargetDurationMS - remaining
	if achieved < cfg.Render.MinDurationMS {
		return outputError("plan_timeline")
	}
	plan.DurationMS = achieved
	return nil
}

// cutCeiling is the furthest a cut's end may move: inside the footage it was
// observed in, never through an unobserved gap or into the next already-selected
// range of the same source, and — unless the target has yielded (hard) — never
// past CDS-37's target ceiling for its scene and preset.
func cutCeiling(plan *clip.EditPlan, analyses map[string]clip.SourceAnalysis, scenes []string, preset string, i int, hard bool) int {
	c := plan.Cuts[i]
	_, end := observedSpan(analyses[c.SourceID], c)
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
	if hard {
		return end
	}
	_, maximum := design.CutBounds(scenes[i], preset)
	return min(end, c.StartMS+maximum)
}

// observedSpan is the footage a cut may grow into: the ONE observed scene that
// contains it. Reconciliation may move either end inside that scene and no
// further — a neighbouring scene shows something else, so footage taken from it
// would not be the scene the cut was selected, bound and written for (CLIP-7).
// A cut no single scene contains gets no room either way.
func observedSpan(a clip.SourceAnalysis, c clip.Cut) (int, int) {
	for _, segment := range a.Segments {
		if segment.StartMS <= c.StartMS && c.EndMS <= segment.EndMS {
			return segment.StartMS, segment.EndMS
		}
	}
	return c.StartMS, c.EndMS
}

// Backward repair preserves the selected opening scene, and therefore the
// scene-derived transition: the floor is the containing scene's own start,
// never back over a neighbouring scene or another selected range.
func backwardSceneFloor(plan *clip.EditPlan, analyses map[string]clip.SourceAnalysis, i int) int {
	start, _ := observedSpan(analyses[plan.Cuts[i].SourceID], plan.Cuts[i])
	return max(cutFloor(plan, analyses, i), start)
}

// cutFloor is the earliest a cut's start may move: the segment it was observed
// in, and never back over the previous already-selected range of that source.
func cutFloor(plan *clip.EditPlan, analyses map[string]clip.SourceAnalysis, i int) int {
	c := plan.Cuts[i]
	start, _ := observedSpan(analyses[c.SourceID], c)
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

// holdCutLengths aims every cut at CDS-37's target: a cut over its target is
// trimmed to it, one under it takes the room its own segment still has —
// forward first, then backward, because moving the end keeps the cut's opening
// frame — and one that still cannot reach the target is left as it is. Nothing
// here refuses: the only hard length rule is the renderer's plan_cut_fade (r3).
func holdCutLengths(plan *clip.EditPlan, analyses map[string]clip.SourceAnalysis, scenes []string, preset string) {
	for i := range plan.Cuts {
		c := &plan.Cuts[i]
		// CDS-37's targets are OUTPUT durations, so the source span that holds
		// them depends on the cut's own rate.
		minimum, maximum := design.CutBounds(scenes[i], preset)
		if c.OutputDurationMS() > maximum {
			c.EndMS = c.StartMS + sourceSpan(maximum, c.Rate())
		}
		if c.OutputDurationMS() < minimum {
			c.EndMS = min(c.StartMS+sourceSpan(minimum, c.Rate()), cutCeiling(plan, analyses, scenes, preset, i, false))
			if c.OutputDurationMS() < minimum {
				c.StartMS = max(c.EndMS-sourceSpan(minimum, c.Rate()), cutFloor(plan, analyses, i))
			}
		}
		// Whichever end moved, the caption still lives inside the cut.
		length := c.OutputDurationMS()
		for j := range c.Copies {
			c.Copies[j].StartMS = min(c.Copies[j].StartMS, length-1)
			c.Copies[j].EndMS = min(c.Copies[j].EndMS, length)
		}
	}
}

// outputSpan is what a stretch of SOURCE footage is worth on the transformed
// output timeline; sourceSpan is the inverse, the footage an output length
// needs. Both go through the one shared helper, so reconciliation cannot drift
// from the timeline the resolver, preview and renderer build (CDS-62).
func outputSpan(sourceMS, rate int) int {
	out, ok := clip.TransformedDuration(sourceMS, rate)
	if !ok {
		return 0
	}
	return out
}
func sourceSpan(outputMS, rate int) int {
	if outputMS <= 0 {
		return 0
	}
	return (outputMS*rate + clip.RateUnitPermille/2) / clip.RateUnitPermille
}
