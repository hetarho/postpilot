package ai

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

type flowCutJSON struct {
	ID           string     `json:"id"`
	SourceID     string     `json:"source_id"`
	StartMS      int        `json:"start_ms"`
	EndMS        int        `json:"end_ms"`
	Rate         int        `json:"rate_permille"`
	Focal        *pointJSON `json:"focal"`
	Volume       float64    `json:"volume"`
	Observations []string   `json:"observation_refs"`
}
type flowJSON struct {
	Ratio      string        `json:"ratio"`
	DurationMS int           `json:"duration_ms"`
	Cuts       []flowCutJSON `json:"cuts"`
}

var flowShape = readShape(flowSchema)

// parseFlowPlan admits the footage flow. Every check the single writer's cuts
// answered still applies — source, range, coverage, one contained scene,
// usability, duplicate ids and overlap — and nothing about sections or items
// does, because a body this writer plans for declares neither. The 40 % share
// and the speech rule bind the WRITING (CLIP-128, CLIP-129): a response that
// breaks one is planned and rendered as written, never silently re-rated,
// because returning one cut to 1x moves every millisecond after it.
func parseFlowPlan(cfg Config, input clip.PlanningInput, raw string) (out clip.EditPlan, err error) {
	var wire flowJSON
	candidate := clip.EditPlan{}
	failedCut := 0
	phase := "decode"
	defer func() {
		if err != nil {
			if _, exists := clip.DiagnosticFromError(err); !exists {
				err = planFailure(err, input, candidate, phase, failedCut)
			}
		}
	}()
	if err := decode(raw, cfg.MaxResponseBytes, flowShape, &wire); err != nil {
		return clip.EditPlan{}, err
	}
	doc, problem := composition.Parse(input.Composition.Snapshot.Body, compositionLimits(cfg, input))
	if problem != nil {
		return clip.EditPlan{}, problem
	}
	analyses := map[string]clip.SourceAnalysis{}
	for _, a := range input.Analyses {
		analyses[a.Source.ID] = a
	}
	phase = "selection"
	plan := clip.EditPlan{Ratio: wire.Ratio}
	proposals := map[string]flowCutJSON{}
	limit := min(cfg.Render.MaxCuts, compositionLimits(cfg, input).Cuts)
	for index, proposed := range wire.Cuts {
		failedCut = index + 1
		drop := func(check string) { recordGeneratedNotice(&plan, check, proposed.ID, "", "removal") }
		if index >= limit {
			drop("composition_plan_bounds")
			continue
		}
		focal, valid := proposed.Focal.domain()
		if !valid {
			return clip.EditPlan{}, outputError("plan_cut_fields")
		}
		analysis, exists := analyses[proposed.SourceID]
		if !exists {
			drop("plan_source")
			continue
		}
		if !writerIdentity.MatchString(proposed.ID) {
			drop("composition_cut_identity")
			continue
		}
		if _, duplicate := proposals[proposed.ID]; duplicate {
			drop("plan_cut_identity")
			continue
		}
		cut := clip.Cut{ID: proposed.ID, SourceID: proposed.SourceID, Fingerprint: analysis.Source.Fingerprint, StartMS: proposed.StartMS, EndMS: proposed.EndMS, PlaybackRatePermille: proposed.Rate, Focal: focal, Volume: &proposed.Volume}
		proposals[cut.ID] = proposed
		plan.Cuts = append(plan.Cuts, cut)
	}
	failedCut = 0
	portable := &clip.PortablePlan{Snapshot: input.Composition.Snapshot, Inputs: input.Composition.Inputs, Observations: input.Analyses, TargetDurationMS: input.TargetDurationMS}
	// Each retry of this local pass removes at least one cut; no model retry.
	for pass, budget := 0, len(plan.Cuts)+1; pass < budget; pass++ {
		if err := composeTimeline(cfg, input, &plan); err != nil {
			return clip.EditPlan{}, err
		}
		candidate = plan
		phase = "composition"
		portable.Cuts = nil
		kept := []clip.Cut{}
		for i, cut := range plan.Cuts {
			failedCut = i + 1
			observed, covered := clip.CutEvidence(input.Analyses, cut)
			if !covered {
				recordGeneratedNotice(&plan, "composition_observation_gap", cut.ID, "", "removal")
				continue
			}
			// The model's stale or fabricated references never become evidence.
			if _, valid := selectedReferences(proposals[cut.ID].Observations, observed, true); !valid {
				recordGeneratedNotice(&plan, "composition_cut_evidence", cut.ID, "", "repair")
			}
			kept = append(kept, cut)
			// No section, group or item: the flow is ordered by the writer, and
			// an item is a hint about footage rather than a place in a template.
			portable.Cuts = append(portable.Cuts, composition.Cut{ID: cut.ID, SourceID: cut.SourceID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS, PlaybackRatePermille: cut.Rate()})
		}
		if len(kept) == len(plan.Cuts) {
			break
		}
		plan.Cuts = kept
	}
	failedCut = 0
	timeline, fallbacks, err := clip.ResolveSelectedComposition(doc, portable.Inputs, portable.Cuts, compositionLimits(cfg, input), cfg.MaxResponseBytes)
	if err != nil {
		return clip.EditPlan{}, err
	}
	portable.Fallbacks = fallbacks
	// The regions the template already states in full. An element still waiting
	// for written text belongs to the narration call and is no notice until it
	// answers (CLIP-138).
	for _, resolved := range timeline.Elements {
		// A caption entry of the outline is placed by the narration call, whatever
		// its text already says, so the flow leaves it alone (CLIP-112).
		if generatesText(resolved.Element) || resolved.Element.Role == "caption" && resolved.CutID == "" {
			continue
		}
		portable.Elements = append(portable.Elements, clip.PortableText{Resolved: resolved, Scope: "context", Accent: doc.Accent, Pace: doc.Pace})
	}
	plan.Portable = portable
	clip.RecomputePlanNotices(&plan, input.TargetDurationMS, cfg.TargetToleranceMS)
	if _, err := clip.EncodeEditPlan(plan); err != nil {
		return clip.EditPlan{}, err
	}
	return plan, nil
}
