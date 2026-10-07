package guideline

import "slices"

// RuleStage and RuleOutput describe a code-owned recommendation's responsibility.
// They never classify, translate or select arbitrary owner-authored text.
type RuleStage string

const (
	StageWrite         RuleStage = "write"
	StageRevise        RuleStage = "revise"
	StageStoryline     RuleStage = "storyline"
	StageClipWrite     RuleStage = "clip-write"
	StageClipRevise    RuleStage = "clip-revise"
	StageClipStoryline RuleStage = "clip-storyline"
)

type RuleOutput string

const (
	OutputPlan       RuleOutput = "plan"
	OutputTitle      RuleOutput = "title"
	OutputTags       RuleOutput = "tags"
	OutputProse      RuleOutput = "prose"
	OutputPlacements RuleOutput = "placements"
	OutputGroups     RuleOutput = "groups"
	OutputCaptions   RuleOutput = "captions"
	OutputNarration  RuleOutput = "narration"
)

type RuleApplicability struct {
	Stage   RuleStage
	Outputs []RuleOutput
}

// StockRule freezes the product identity, localized text, registry position and
// declared applicability together. Nil Stock in a retained payload means unknown
// historical applicability; its plain text must never be matched to today's registry.
type StockRule struct {
	Key, Text     string
	SourceOrder   int
	Applicability []RuleApplicability
}

func copyApplicability(source []RuleApplicability) []RuleApplicability {
	if source == nil {
		return nil
	}
	result := slices.Clone(source)
	for index := range result {
		result[index].Outputs = slices.Clone(result[index].Outputs)
	}
	return result
}
func postFinal(outputs ...RuleOutput) []RuleApplicability {
	return []RuleApplicability{{Stage: StageWrite, Outputs: slices.Clone(outputs)}, {Stage: StageRevise, Outputs: slices.Clone(outputs)}}
}
func postMaterial(outputs ...RuleOutput) []RuleApplicability {
	return append(postFinal(outputs...), RuleApplicability{Stage: StageStoryline, Outputs: []RuleOutput{OutputPlan, OutputPlacements}})
}
func clipMaterial(outputs ...RuleOutput) []RuleApplicability {
	return []RuleApplicability{{Stage: StageClipWrite, Outputs: slices.Clone(outputs)}, {Stage: StageClipRevise, Outputs: slices.Clone(outputs)}, {Stage: StageClipStoryline, Outputs: []RuleOutput{OutputPlan, OutputPlacements}}}
}

// AppliesTo is a metadata-only check. An empty output selection asks for every
// responsibility of that stage; unknown stages never gain an inferred fallback.
func (r StockRule) AppliesTo(stage RuleStage, outputs ...RuleOutput) bool {
	for _, entry := range r.Applicability {
		if entry.Stage != stage {
			continue
		}
		if len(outputs) == 0 {
			return len(entry.Outputs) > 0
		}
		for _, output := range outputs {
			if slices.Contains(entry.Outputs, output) {
				return true
			}
		}
	}
	return false
}
func defaultPromptStock(states []DefaultState, target Language, withMemories bool) []StockRule {
	// Non-nil empty is authoritative: this is newly resolved code-owned metadata.
	result := make([]StockRule, 0, len(states))
	for order, state := range states {
		if !state.Enabled || state.Default.MemoriesOnly && !withMemories {
			continue
		}
		if text, ok := state.Default.Text(target); ok {
			result = append(result, StockRule{Key: state.Default.Key, Text: text, SourceOrder: order, Applicability: copyApplicability(state.Default.Applicability)})
		}
	}
	return result
}
func stockTexts(rules []StockRule) []string {
	var texts []string
	for _, rule := range rules {
		texts = append(texts, rule.Text)
	}
	return texts
}
