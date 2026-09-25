package ai

import (
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

type planTier string

const (
	repairPlan planTier = "repair"
	removePlan planTier = "removal"
	failPlan   planTier = "failure"
)

// Every writer check declares its tier here. Observation and render checks are
// outside this ladder. Unknown readable violations remove their smallest target;
// only decoding or an unrenderable remainder can fail a generation.
var planCheckTiers = map[string]planTier{
	"plan_cut_scene": repairPlan, "plan_cut_rate": repairPlan, "plan_cut_usability": repairPlan,
	"plan_accent": repairPlan, "plan_focal": repairPlan, "plan_volume": repairPlan,
	"plan_cut_fade": repairPlan, "plan_cut_transition": repairPlan, "plan_target_duration": repairPlan,
	"plan_caption_time": repairPlan, "composition_cut_evidence": repairPlan, "plan_ratio": repairPlan,
	"composition_section_order": removePlan, "composition_item_order": removePlan,
	"plan_source_overlap": removePlan, "plan_cut_identity": removePlan, "plan_source": removePlan,
	"composition_cut_identity": removePlan, "composition_observation_gap": removePlan,
	"composition_generated_identity": removePlan, "composition_generated_rows": removePlan,
	"composition_generated_bounds": removePlan, "composition_plan_bounds": removePlan, "plan_cut_count": removePlan,
	"plan_cut_range": removePlan, "plan_source_metadata": removePlan, "plan_hook": removePlan,
	"intro_slot_shortened": repairPlan, "outro_slot_shortened": repairPlan,
	"intro_slot_omitted": removePlan, "outro_slot_omitted": removePlan,
	"plan_chip_count": removePlan, "plan_chip_label": removePlan, "plan_copy_chars": removePlan,
	"plan_copy_classes": removePlan, "plan_copy_count": removePlan, "plan_copy_exposure": removePlan,
	"plan_copy_format": removePlan, "plan_copy_keyword": removePlan, "plan_copy_lines": removePlan,
	"plan_copy_second_cut": removePlan, "plan_copy_sequence": removePlan,
	"plan_source_audio": repairPlan, "plan_duration_range": repairPlan, "plan_duration_limit": repairPlan,
	"plan_timeline": failPlan, "caption_measurement": failPlan, "plan_length_floor": failPlan,
	"plan_required": failPlan, "plan_cut_fields": failPlan, "plan_caption_fields": failPlan,
	"output_encoding_or_size": failPlan, "output_field_type": failPlan, "output_json": failPlan, "output_shape": failPlan,
}

func tierForPlan(check string) planTier {
	if tier, ok := planCheckTiers[check]; ok {
		return tier
	}
	return removePlan
}

func recordGeneratedNotice(plan *clip.EditPlan, check, cut, element, action string) {
	tier := tierForPlan(check)
	if check == "plan_cut_usability" && action == "removal" {
		tier = removePlan
	}
	clip.AddPlanNotice(plan, check, cut, element, string(tier))
}

// The selector repairs only within observed material, retains order and never
// changes copy or owner facts. Timeline arithmetic continues in composeTimeline.
func narrowGeneratedCuts(cfg Config, in clip.PlanningInput, plan *clip.EditPlan) {
	if plan.Ratio != in.Ratio {
		plan.Ratio = in.Ratio
		recordGeneratedNotice(plan, "plan_ratio", "", "", "repair")
	}
	selected := make([]clip.Cut, 0, len(plan.Cuts))
	written := []clip.Written{}
	seen := map[string]bool{}
	for i, original := range plan.Cuts {
		c := original
		notice := func(check, action string) { recordGeneratedNotice(plan, check, c.ID, "", action) }
		drop := ""
		switch {
		case len(selected) >= cfg.Render.MaxCuts:
			drop = "plan_cut_count"
		case strings.TrimSpace(c.ID) == "" || seen[c.ID]:
			drop = "plan_cut_identity"
		}
		if drop != "" {
			notice(drop, "removal")
			continue
		}
		seen[c.ID] = true
		at := slices.IndexFunc(in.Analyses, func(a clip.SourceAnalysis) bool {
			return a.Source.ID == c.SourceID && a.Source.Fingerprint == c.Fingerprint
		})
		if at < 0 {
			notice("plan_source", "removal")
			continue
		}
		a := in.Analyses[at]
		if c.StartMS < 0 || c.EndMS <= c.StartMS || c.EndMS > a.Source.Info.DurationMS {
			notice("plan_cut_range", "removal")
			continue
		}
		scene, contained := clip.ContainedScene(in.Analyses, c)
		if !contained {
			overlaps := clip.CutScenes(in.Analyses, c)
			if len(overlaps) == 0 {
				notice("composition_observation_gap", "removal")
				continue
			}
			scene = overlaps[0]
			c.StartMS, c.EndMS = max(c.StartMS, scene.StartMS), min(c.EndMS, scene.EndMS)
			notice("plan_cut_scene", "repair")
		}
		if !slices.Contains(clip.AllowedPlaybackRates(a.Source.Info), c.Rate()) {
			c.PlaybackRatePermille = clip.RateUnitPermille
			notice("plan_cut_rate", "repair")
		}
		if !clip.SelectableScene(scene, c.Rate()) {
			if !clip.SelectableScene(scene, clip.RateUnitPermille) {
				notice("plan_cut_usability", "removal")
				continue
			}
			c.PlaybackRatePermille = clip.RateUnitPermille
			notice("plan_cut_usability", "repair")
		}
		if !clip.Normalized(c.Focal.X) || !clip.Normalized(c.Focal.Y) {
			c.Focal.X, c.Focal.Y = clampUnit(c.Focal.X), clampUnit(c.Focal.Y)
			notice("plan_focal", "repair")
		}
		if !clip.Normalized(c.OriginalVolume()) {
			v := clampUnit(c.OriginalVolume())
			c.Volume = &v
			notice("plan_volume", "repair")
		}
		if slices.ContainsFunc(selected, func(prior clip.Cut) bool {
			return prior.SourceID == c.SourceID && prior.StartMS < c.EndMS && c.StartMS < prior.EndMS
		}) {
			notice("plan_source_overlap", "removal")
			continue
		}
		c.Copies = slices.Clone(c.Copies)
		for j := range c.Copies {
			p := &c.Copies[j]
			start, end := p.StartMS, p.EndMS
			p.StartMS = max(0, min(p.StartMS, c.OutputDurationMS()-1))
			p.EndMS = max(p.StartMS+1, min(p.EndMS, c.OutputDurationMS()))
			if p.StartMS != start || p.EndMS != end {
				notice("plan_caption_time", "repair")
			}
			if p.Accent != "" && p.Accent != in.Template.Accent {
				p.Accent = in.Template.Accent
				notice("plan_accent", "repair")
			}
		}
		selected = append(selected, c)
		if i < len(plan.Written) {
			written = append(written, plan.Written[i])
		}
	}
	plan.Cuts = selected
	if plan.Written != nil {
		plan.Written = written
	}
}

func clampUnit(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return max(0, min(1, v))
}

func repairTransitions(plan *clip.EditPlan) {
	for i := range plan.Cuts {
		c := &plan.Cuts[i]
		if i == 0 && c.TransitionMS != 0 || !clip.ValidTransition(c.TransitionMS) {
			c.TransitionMS = 0
			recordGeneratedNotice(plan, "plan_cut_transition", c.ID, "", "repair")
		}
		overlap := c.TransitionMS
		if i+1 < len(plan.Cuts) {
			overlap += plan.Cuts[i+1].TransitionMS
		}
		if c.OutputDurationMS() <= overlap {
			c.TransitionMS = 0
			if i+1 < len(plan.Cuts) {
				plan.Cuts[i+1].TransitionMS = 0
			}
			recordGeneratedNotice(plan, "plan_cut_fade", c.ID, "", "repair")
		}
	}
}

// Trim an otherwise feasible overrun at the tail; surviving words remain exact.
// No cut is trimmed under CDS's cut floor while an earlier cut can still give
// the time: a sliver of footage stays in the plan, shown in ② and counted, yet
// plays as nothing. Only an overrun those floors cannot absorb falls back to the
// transitions' own floor, because validatePlan refuses a plan over its target.
func trimGeneratedOverrun(plan *clip.EditPlan, target int) {
	total := trimTail(plan, target, int(math.Round(design.Timing.CutMinS*1000)))
	if total > target {
		total = trimTail(plan, target, 1)
	}
	plan.DurationMS = total
}

// trimTail walks the cuts from the last, taking from each what it holds above
// minimum and above the transitions it overlaps, and returns the new total.
func trimTail(plan *clip.EditPlan, target, minimum int) int {
	total := -plan.TransitionTotal()
	for _, c := range plan.Cuts {
		total += c.OutputDurationMS()
	}
	for i := len(plan.Cuts) - 1; i >= 0 && total > target; i-- {
		c := &plan.Cuts[i]
		floor := c.TransitionMS + 1
		if i+1 < len(plan.Cuts) {
			floor += plan.Cuts[i+1].TransitionMS
		}
		floor = max(floor, minimum)
		amount := min(total-target, max(0, c.OutputDurationMS()-floor))
		if amount == 0 {
			continue
		}
		c.EndMS = c.StartMS + sourceSpan(c.OutputDurationMS()-amount, c.Rate())
		recordGeneratedNotice(plan, "plan_target_duration", c.ID, "", "repair")
		for j := range c.Copies {
			p := &c.Copies[j]
			if p.StartMS == 0 && p.EndMS == 0 {
				continue
			}
			p.StartMS = max(0, min(p.StartMS, c.OutputDurationMS()-1))
			p.EndMS = max(p.StartMS+1, min(p.EndMS, c.OutputDurationMS()))
		}
		total = -plan.TransitionTotal()
		for _, cut := range plan.Cuts {
			total += cut.OutputDurationMS()
		}
	}
	return total
}

// Timing repair can leave an otherwise valid generated caption too short to
// read. Validate each addition and omit only that text, keeping exact words on
// the surviving captions. The owner-edit validator remains strict.
func removeInvalidGeneratedCopies(cfg Config, in clip.PlanningInput, plan *clip.EditPlan) {
	sources := make([]clip.RenderSource, 0, len(in.Analyses))
	for _, a := range in.Analyses {
		sources = append(sources, a.Source.RenderSource)
	}
	bounds := cfg.Render
	bounds.MinDurationMS = 1
	for i := range plan.Cuts {
		cut := &plan.Cuts[i]
		kept := make([]clip.Copy, 0, len(cut.Copies))
		for _, copy := range cut.Copies {
			candidate := *cut
			candidate.Copies = append(slices.Clone(kept), copy)
			candidate.TransitionMS = 0
			probe := clip.EditPlan{Ratio: plan.Ratio, DurationMS: candidate.OutputDurationMS(), Cuts: []clip.Cut{candidate}}
			err := clip.ValidateEditPlan(bounds, probe, sources)
			var cause interface{ OutputValidationCode() string }
			code := ""
			if errors.As(err, &cause) {
				code = cause.OutputValidationCode()
			}
			if errors.Is(err, clip.ErrCopyTooLong) {
				code = "plan_copy_chars"
			}
			if strings.HasPrefix(code, "plan_copy_") || code == "plan_caption_time" {
				clip.AddPlanNotice(plan, code, cut.ID, "caption", "removal")
				continue
			}
			kept = append(kept, copy)
		}
		if len(kept) != len(cut.Copies) {
			cut.Copies = kept
		}
	}
}

// repairGeneratedText bounds generated text outside a region block by its
// declared character maximum (CLIP-118): the text itself, then the first grounded
// alternative within it, then removal.
func repairGeneratedText(text string, alternatives []clip.CopyAlternative, limit int) (string, string) {
	return repairWith(text, alternatives, func(value string) bool {
		return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\r\n") && design.Chars(value) <= limit
	})
}

func repairWith(text string, alternatives []clip.CopyAlternative, fits func(string) bool) (string, string) {
	if fits(text) {
		return text, ""
	}
	for _, candidate := range alternatives {
		if fits(candidate.Text) {
			return candidate.Text, "repair"
		}
	}
	return "", "removal"
}

// Alternatives reaching this rung already passed the same scoped grounding as
// the original. A row is judged by the slot's own fit (CDS-86): one that shrinks
// or wraps is kept as written, and only one still too wide at the floor — or
// holding a glyph the slot's face lacks — takes the shorter alternative. A
// row's declared maximum stays the author's stricter bound (CLIP-116).
func repairGeneratedSlot(text string, alternatives []clip.CopyAlternative, spec design.SlotSpec, width float64, declared int) (string, string) {
	return repairWith(text, alternatives, func(value string) bool {
		return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\r\n") && !design.FitRegionSlot(spec, value, width).Over && (declared <= 0 || design.Chars(value) <= declared)
	})
}
