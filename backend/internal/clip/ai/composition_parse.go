package ai

import (
	"regexp"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

type compositionCutJSON struct {
	ID           string     `json:"id"`
	SourceID     string     `json:"source_id"`
	SectionID    string     `json:"template_section_id"`
	GroupID      string     `json:"group_id"`
	ItemID       string     `json:"item_id"`
	StartMS      int        `json:"start_ms"`
	EndMS        int        `json:"end_ms"`
	Rate         int        `json:"rate_permille"`
	Focal        *pointJSON `json:"focal"`
	Volume       float64    `json:"volume"`
	Observations []string   `json:"observation_refs"`
}
type factJSON struct {
	FieldID string `json:"field_id"`
	GroupID string `json:"group_id"`
	ItemID  string `json:"item_id"`
}
type generatedJSON struct {
	ElementID    string     `json:"element_id"`
	CutID        string     `json:"cut_id"`
	Text         string     `json:"text"`
	ShortText    string     `json:"short_text"`
	Keyword      string     `json:"keyword"`
	Rows         []string   `json:"rows"`
	ShortRows    []string   `json:"short_rows"`
	Observations []string   `json:"observation_refs"`
	Facts        []factJSON `json:"fact_refs"`
}
type compositionPlanJSON struct {
	Ratio      string               `json:"ratio"`
	DurationMS int                  `json:"duration_ms"`
	Cuts       []compositionCutJSON `json:"cuts"`
	Generated  []generatedJSON      `json:"generated"`
}

var compositionPlanShape = readShape(compositionPlanSchema)
var writerIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// A reference must identify an actual overlapping observation. For cut selection
// all covering observations are mandatory; copy may cite a subset of that cut.
func selectedReferences(ids []string, evidence []clip.ObservedEvidence, complete bool) ([]clip.SourceEvidence, bool) {
	if len(ids) > 120 || complete && len(ids) == 0 {
		return nil, false
	}
	seen := map[string]bool{}
	var out []clip.SourceEvidence
	for _, id := range ids {
		if seen[id] {
			return nil, false
		}
		seen[id] = true
		found := false
		for _, observation := range evidence {
			if observation.ID == id {
				out = append(out, observation.Source)
				found = true
			}
		}
		if !found {
			return nil, false
		}
	}
	if complete {
		for _, observation := range evidence {
			if !seen[observation.ID] {
				return nil, false
			}
		}
	}
	return out, true
}

func parseCompositionPlan(cfg Config, input clip.PlanningInput, raw string) (out clip.EditPlan, err error) {
	var wire compositionPlanJSON
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
	if err := decode(raw, cfg.MaxResponseBytes, compositionPlanShape, &wire); err != nil {
		return clip.EditPlan{}, err
	}
	phase = "selection"
	for _, c := range wire.Cuts {
		if len(candidate.Cuts) >= cfg.Render.MaxCuts {
			break
		}
		candidate.Cuts = append(candidate.Cuts, clip.Cut{SourceID: c.SourceID, StartMS: c.StartMS, EndMS: c.EndMS, PlaybackRatePermille: c.Rate})
	}
	doc, problem := composition.Parse(input.Composition.Snapshot.Body, compositionLimits(cfg, input))
	if problem != nil {
		return clip.EditPlan{}, problem
	}

	sections, order := map[string]composition.Section{}, map[string]int{}
	for i, section := range doc.Sections {
		sections[section.ID], order[section.ID] = section, i
	}
	if len(doc.Sections) == 0 {
		sections[""] = composition.Section{Scope: "scene", Repeat: "scenes"}
	}
	analyses := map[string]clip.SourceAnalysis{}
	for _, a := range input.Analyses {
		analyses[a.Source.ID] = a
	}
	plan := clip.EditPlan{Ratio: wire.Ratio}
	proposals := map[string]compositionCutJSON{}
	limit := min(cfg.Render.MaxCuts, compositionLimits(cfg, input).Cuts)
	for index, proposed := range wire.Cuts {
		failedCut = index + 1
		drop := func(check string) { recordGeneratedNotice(&plan, check, proposed.ID, "", "removal") }
		if index >= limit {
			drop("composition_plan_bounds")
			continue
		}
		_, exists := sections[proposed.SectionID]
		if !exists {
			drop("composition_section_order")
			continue
		}
		focal, valid := proposed.Focal.domain()
		analysis, exists := analyses[proposed.SourceID]
		if !valid {
			return clip.EditPlan{}, outputError("plan_cut_fields")
		}
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
	bindings := map[string]clip.ItemBinding{}
	evidence := map[string][]clip.ObservedEvidence{}
	// Each retry of this local pass removes at least one cut; no model retry.
	for pass, budget := 0, len(plan.Cuts)+1; pass < budget; pass++ {
		if err := composeTimeline(cfg, input, &plan); err != nil {
			return clip.EditPlan{}, err
		}
		candidate = plan
		phase = "composition"
		portable.Cuts = nil
		bindings = map[string]clip.ItemBinding{}
		evidence = map[string][]clip.ObservedEvidence{}
		lastItem := map[string]int{}
		lastSection := -1
		kept := []clip.Cut{}
		for i, cut := range plan.Cuts {
			failedCut = i + 1
			proposed := proposals[cut.ID]
			observed, covered := clip.CutEvidence(input.Analyses, cut)
			if !covered {
				recordGeneratedNotice(&plan, "composition_observation_gap", cut.ID, "", "removal")
				continue
			}
			// The model's stale or fabricated references never become evidence.
			if _, valid := selectedReferences(proposed.Observations, observed, true); !valid {
				recordGeneratedNotice(&plan, "composition_cut_evidence", cut.ID, "", "repair")
			}
			section := sections[proposed.SectionID]
			if order[section.ID] < lastSection {
				recordGeneratedNotice(&plan, "composition_section_order", cut.ID, "", "removal")
				continue
			}
			group := section.Repeat
			if group == "scenes" {
				group = ""
			}
			binding := clip.ItemBinding{}
			if section.Scope != "context" {
				binding = clip.BindCutItem(portable.Inputs, observed, cut, group)
			}
			backwards := false
			if group != "" && binding.ItemID != "" {
				for index, item := range portable.Inputs.Items[group] {
					if item.ID != binding.ItemID {
						continue
					}
					if index+1 < lastItem[section.ID] {
						backwards = true
					} else {
						lastItem[section.ID] = index + 1
					}
				}
			}
			if backwards {
				recordGeneratedNotice(&plan, "composition_item_order", cut.ID, "", "removal")
				continue
			}
			lastSection = order[section.ID]
			kept = append(kept, cut)
			bindings[cut.ID], evidence[cut.ID] = binding, observed
			portable.Cuts = append(portable.Cuts, composition.Cut{ID: cut.ID, SectionID: section.ID, SourceID: cut.SourceID, GroupID: binding.GroupID, ItemID: binding.ItemID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS, PlaybackRatePermille: cut.Rate()})
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
	for i := range portable.Fallbacks {
		f := &portable.Fallbacks[i]
		if reason := bindings[f.CutID].Reason; reason != "" {
			f.Reason = reason
		}
	}
	if err := attachCompositionCopy(cfg, doc, wire.Generated, timeline, portable, bindings, evidence, &plan, input.Instruction != ""); err != nil {
		return clip.EditPlan{}, err
	}
	plan.Portable = portable
	clip.RecomputePlanNotices(&plan, input.TargetDurationMS, cfg.TargetToleranceMS)
	if _, err := clip.EncodeEditPlan(plan); err != nil {
		return clip.EditPlan{}, err
	}
	return plan, nil
}
