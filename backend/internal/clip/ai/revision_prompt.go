package ai

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// A revision is the SAME contract with one block appended (CLIP-92): the plan
// as the owner's own edits left it, what they asked for, and the one sentence
// that says a response replaces the targeted document whole. Restating the cut,
// rate or caption rules in a second wording is exactly what one vocabulary
// forbids — a rule stated twice is a rule that can disagree with itself.
const revisionBlock = `
This is a REVISION of the plan in current_plan, which is the owner's own plan as their edits left it. revision_request is what they asked for, in their words.
Answer it inside the contract above: return the whole document this call writes, rewritten, keeping everything the request does not ask to change. The template's fixed regions — the intro and outro skeleton, the badge and every declared maximum — are not yours to change, and neither are the recorded observations. A request you cannot answer within this contract is answered as closely as the contract allows; never invent a structure, a field or a value to satisfy it.
`

// The one sentence a narration-only revision adds: the flow under it is the
// owner's, not this call's (CLIP-131).
const narrationRevisionBlock = `The footage flow in current_plan is FINAL for this revision: cuts, their ranges, their rates and their order stay exactly as they are, whatever the request says about them. Rewrite only what is said over them.
`

// revisionPayload is the plan the request is about, in the same vocabulary the
// two writing calls already read: the flow as cuts on the output timeline, and
// the narration as its captions.
func revisionPayload(in clip.RevisionInput) map[string]any {
	captions := []map[string]any{}
	if in.Current.Portable != nil {
		for _, text := range in.Current.Portable.Elements {
			if text.Scope != clip.NarrationScope {
				continue
			}
			r := text.Resolved
			captions = append(captions, map[string]any{"text": r.Text, "start_ms": r.StartMS, "end_ms": r.EndMS})
		}
	}
	return map[string]any{
		"cuts":               resolvedFlowPayload(clip.NarrationInput{PlanningInput: in.PlanningInput, Flow: in.Current}),
		"output_duration_ms": in.Current.DurationMS,
		"captions":           captions,
	}
}

func buildFlowRevisionPrompt(in clip.RevisionInput, fadeMS int, limits composition.Limits) (string, string) {
	system, user := BuildFlowPrompt(in.PlanningInput, fadeMS, limits)
	return system + revisionBlock, user + promptJSON(map[string]any{
		"current_plan": revisionPayload(in), "revision_request": in.Request,
	})
}

func buildNarrationRevisionPrompt(in clip.RevisionInput, flow clip.EditPlan, limits composition.Limits) (string, string) {
	system, user := BuildNarrationPrompt(clip.NarrationInput{PlanningInput: in.PlanningInput, Flow: flow}, limits)
	block := revisionBlock
	if in.Target == clip.RevisionNarration {
		block += narrationRevisionBlock
	}
	return system + block, user + promptJSON(map[string]any{
		"current_plan": revisionPayload(in), "revision_request": in.Request,
	})
}
