package ai

import (
	"regexp"
	"slices"

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

func parseCompositionPlan(cfg Config, input clip.PlanningInput, raw string) (clip.EditPlan, error) {
	var wire compositionPlanJSON
	if err := decode(raw, cfg.MaxResponseBytes, compositionPlanShape, &wire); err != nil {
		return clip.EditPlan{}, err
	}
	doc, problem := composition.Parse(input.Composition.Snapshot.Body, compositionLimits(cfg, input))
	if problem != nil {
		return clip.EditPlan{}, problem
	}
	if wire.Ratio != input.Ratio || len(wire.Cuts) == 0 || len(wire.Cuts) > min(cfg.Render.MaxCuts, compositionLimits(cfg, input).Cuts) || len(wire.Generated) > compositionLimits(cfg, input).Cues {
		return clip.EditPlan{}, outputError("composition_plan_bounds")
	}
	sections, order := map[string]composition.Section{}, map[string]int{}
	for i, section := range doc.Sections {
		sections[section.ID], order[section.ID] = section, i
	}
	if len(doc.Sections) == 0 {
		sections[""] = composition.Section{Scope: "scene", Repeat: "scenes"}
	}
	analyses := map[string]clip.SourceAnalysis{}
	for _, analysis := range input.Analyses {
		analyses[analysis.Source.ID] = analysis
	}
	plan := clip.EditPlan{Ratio: wire.Ratio, Styles: slices.Clone(doc.Styles)}
	seen, lastSection := map[string]bool{}, -1
	for _, proposed := range wire.Cuts {
		section, exists := sections[proposed.SectionID]
		if !exists || order[section.ID] < lastSection || section.Repeat == "" && seen[section.ID] {
			return clip.EditPlan{}, outputError("composition_section_order")
		}
		seen[section.ID], lastSection = true, order[section.ID]
		focal, valid := proposed.Focal.domain()
		analysis, exists := analyses[proposed.SourceID]
		if !exists || !valid || !writerIdentity.MatchString(proposed.ID) {
			return clip.EditPlan{}, outputError("composition_cut_identity")
		}
		cut := clip.Cut{ID: proposed.ID, SourceID: proposed.SourceID, Fingerprint: analysis.Source.Fingerprint, StartMS: proposed.StartMS, EndMS: proposed.EndMS, Focal: focal, Volume: &proposed.Volume}
		evidence, covered := clip.CutEvidence(input.Analyses, cut)
		if !covered {
			return clip.EditPlan{}, outputError("composition_observation_gap")
		}
		if _, valid := selectedReferences(proposed.Observations, evidence, true); !valid {
			return clip.EditPlan{}, outputError("composition_cut_evidence")
		}
		plan.Cuts = append(plan.Cuts, cut)
	}
	// No category preset is consulted by the native timeline. Recompute evidence
	// and item identity after every deterministic range adjustment.
	if err := composeTimeline(cfg, input, &plan); err != nil {
		return clip.EditPlan{}, err
	}
	portable := &clip.PortablePlan{Snapshot: input.Composition.Snapshot, Inputs: input.Composition.Inputs, Observations: input.Analyses, TargetDurationMS: input.TargetDurationMS}
	bindings := map[string]clip.ItemBinding{}
	evidence := map[string][]clip.ObservedEvidence{}
	lastItem := map[string]int{}
	for i, cut := range plan.Cuts {
		observed, covered := clip.CutEvidence(input.Analyses, cut)
		if !covered {
			return clip.EditPlan{}, outputError("composition_observation_gap")
		}
		evidence[cut.ID] = observed
		section := sections[wire.Cuts[i].SectionID]
		group := section.Repeat
		if group == "scenes" {
			group = ""
		}
		binding := clip.ItemBinding{}
		if section.Scope != "context" {
			binding = clip.BindCutItem(portable.Inputs, observed, cut, group)
		}
		if group != "" && binding.ItemID != "" {
			for index, item := range portable.Inputs.Items[group] {
				if item.ID != binding.ItemID {
					continue
				}
				if index+1 < lastItem[section.ID] {
					return clip.EditPlan{}, outputError("composition_item_order")
				}
				lastItem[section.ID] = index + 1
			}
		}
		bindings[cut.ID] = binding
		portable.Cuts = append(portable.Cuts, composition.Cut{ID: cut.ID, SectionID: section.ID, SourceID: cut.SourceID, GroupID: binding.GroupID, ItemID: binding.ItemID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS})
	}
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
	if err := attachCompositionCopy(cfg, doc, wire.Generated, timeline, portable, bindings, evidence); err != nil {
		return clip.EditPlan{}, err
	}
	plan.Portable = portable
	if _, err := clip.EncodeEditPlan(plan, doc.Styles); err != nil {
		return clip.EditPlan{}, err
	}
	return plan, nil
}
