package ai

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
)

// An auto revision names no document (CLIP-131). Its first writing call answers
// one envelope: the narration revision itself when the footage flow stays, or a
// revised flow that the existing narration call then writes over. A request
// that needs no footage change therefore spends one call of the two approved
// (CLIP-132, QUOTA-46).
func routeSchema(flow, narration []byte) []byte {
	var flowDoc, narrationDoc map[string]any
	if err := json.Unmarshal(flow, &flowDoc); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(narration, &narrationDoc); err != nil {
		panic(err)
	}
	out, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"flow_changed"},
		"properties": map[string]any{"flow_changed": map[string]any{"type": "boolean"}, "flow": flowDoc, "narration": narrationDoc},
	})
	if err != nil {
		panic(err)
	}
	return out
}

var revisionRouteSchema = routeSchema(revisionFlowSchema, narrationSchema)
var revisionRouteShape = readShape(routeSchema(revisionFlowParseSchema, narrationParseSchema))
var revisionRouteOutputSchema = structuralSchema(revisionRouteSchema)
var spokenRouteSchema = routeSchema(revisionFlowSchema, spokenRevisionSchema)
var spokenRouteShape = readShape(routeSchema(revisionFlowParseSchema, spokenRevisionSchema))
var spokenRouteOutputSchema = structuralSchema(spokenRouteSchema)

// RevisionRouteSchema is the structural shape of an auto revision's first answer.
func RevisionRouteSchema() []byte { return append([]byte(nil), revisionRouteOutputSchema...) }

// routeJSON keeps each branch raw so the branch is read by its own existing parser.
type routeJSON struct {
	FlowChanged *bool           `json:"flow_changed"`
	Flow        json.RawMessage `json:"flow"`
	Narration   json.RawMessage `json:"narration"`
}

// routeBlock follows both contracts, so it is the last word on the envelope.
const routeBlock = `
This revision ALSO decides which document it rewrites. Change the footage flow only when revision_request needs footage, order or timing changes: other cuts, other ranges, another order, another rate or another length. Otherwise the footage flow in current_plan stays exactly as it is and only what is said over it is rewritten.
Return ONE JSON object {"flow_changed": boolean, "flow": object, "narration": object} instead of the bare documents above:
- flow_changed false: "narration" is the whole narration document answering the NARRATION contract over the cuts in current_plan, and "flow" is omitted.
- flow_changed true: "flow" is the whole footage flow document answering the FLOW contract, and "narration" is omitted; the narration is written over the new flow in a separate call.
`

const routeNarrationHeading = "\nNARRATION contract, answered under \"narration\" when the flow stays:\n"

// buildRoutePrompt is the flow revision request with the narration contract and the
// narration's own material beside it. The narration's cuts are current_plan's.
func buildRoutePrompt(in clip.RevisionInput, fadeMS int, limits composition.Limits, narrationSystem string, narrationMaterial map[string]any, mode string) (string, string) {
	flowSystem, flowUser := flowPromptParts(in.PlanningInput, fadeMS, limits, false)
	var data map[string]any
	_ = json.Unmarshal([]byte(flowUser), &data)
	for key, value := range narrationMaterial {
		if _, taken := data[key]; !taken && key != "cuts" && key != "analyses" {
			data[key] = value
		}
	}
	system := flowSystem + revisionBlock + routeNarrationHeading + narrationSystem + routeBlock + scopeRule(in) +
		videoGuidelineBlock(videoGuidelinesFor(in.Guidelines, mode))
	return system, promptJSON(data) + promptJSON(withScope(map[string]any{"current_plan": revisionPayload(in), "revision_request": in.Request}, in))
}

func buildRevisionRoutePrompt(in clip.RevisionInput, fadeMS int, limits composition.Limits) (string, string) {
	narration := clip.NarrationInput{PlanningInput: in.PlanningInput, Flow: in.Current}
	system, user := narrationPromptParts(narration, limits)
	system = strings.TrimSuffix(strings.TrimPrefix(system, videoWritingContract(in.PlanningInput)), advertiserGuideRule(in.PlanningInput))
	var material map[string]any
	_ = json.Unmarshal([]byte(user), &material)
	return buildRoutePrompt(in, fadeMS, limits, system, material, "revision-route")
}

func buildSpokenRoutePrompt(in clip.RevisionInput, fadeMS int, limits composition.Limits) (string, string) {
	return buildRoutePrompt(in, fadeMS, limits, spokenRevisionContract(in), map[string]any{
		"current_spoken_script": spokenLinesPayload(in), "output_duration_ms": in.Current.DurationMS,
	}, "spoken-revision-route")
}

// route makes an auto revision's first call. It returns the finished plan when the
// writing kept the flow, or the revised flow when it did not; flowChanged says which.
// An envelope missing the branch it chose is unreadable and takes CLIP-94's bounded
// correction like any other.
func (s *Service) route(ctx context.Context, model llm.ModelRef, in clip.RevisionInput, mode string, schema []byte, contract *shape,
	prompt func() (string, string), narration func(raw string) (clip.EditPlan, error)) (clip.EditPlan, bool, llm.Usage, error) {
	flowChanged := false
	plan, usage, err := s.write(ctx, model, in.PlanningInput, "flow", mode, s.cfg.FlowCompletionTokens, schema, prompt, func(raw string) (clip.EditPlan, error) {
		var wire routeJSON
		if err := decode(raw, s.cfg.MaxResponseBytes, contract, &wire); err != nil {
			return clip.EditPlan{}, err
		}
		if wire.FlowChanged == nil {
			return clip.EditPlan{}, outputError("output_shape")
		}
		if *wire.FlowChanged {
			if len(wire.Flow) == 0 {
				return clip.EditPlan{}, outputError("output_shape")
			}
			flow, err := parseFlowPlan(s.cfg, in.PlanningInput, string(wire.Flow), false)
			flowChanged = err == nil
			return flow, err
		}
		if len(wire.Narration) == 0 {
			return clip.EditPlan{}, outputError("output_shape")
		}
		flowChanged = false
		return narration(string(wire.Narration))
	})
	if err != nil {
		return clip.EditPlan{}, false, usage, err
	}
	if in.Routed != nil {
		stage := "narrate"
		if flowChanged {
			stage = "flow"
		}
		in.Routed(stage)
	}
	return plan, flowChanged, usage, nil
}

// scopeRule is the one sentence a selected scene adds (CLIP-131); the whole clip adds none.
func scopeRule(in clip.RevisionInput) string {
	if in.ScopeCut <= 0 {
		return ""
	}
	return "\nrevision_scope names the scene block the owner selected: cut number revision_scope.cut of current_plan, in output order, with the captions or spoken segments starting inside it. Change that scene and keep the rest as it stands unless revision_request requires otherwise.\n"
}

// withScope adds the selected scene's number and what is said in it now.
func withScope(payload map[string]any, in clip.RevisionInput) map[string]any {
	if in.ScopeCut <= 0 || in.ScopeCut > len(in.Current.Cuts) {
		return payload
	}
	// The cut's output interval, measured as resolvedFlowPayload measures it.
	start, end := 0, 0
	for i, cut := range in.Current.Cuts {
		start -= cut.TransitionMS
		if i == in.ScopeCut-1 {
			end = start + cut.OutputDurationMS()
			break
		}
		start += cut.OutputDurationMS()
	}
	inside := func(at int) bool {
		return at >= start && (at < end || in.ScopeCut == len(in.Current.Cuts) && at <= end)
	}
	scope := map[string]any{"cut": in.ScopeCut, "start_ms": start, "end_ms": end}
	captions := []map[string]any{}
	if in.Current.Portable != nil {
		for _, text := range in.Current.Portable.Elements {
			if text.Scope == clip.NarrationScope && inside(text.Resolved.StartMS) {
				captions = append(captions, map[string]any{"text": text.Resolved.Text, "start_ms": text.Resolved.StartMS, "end_ms": text.Resolved.EndMS})
			}
		}
	}
	scope["captions"] = captions
	if in.Current.Narration != nil {
		segments := []map[string]any{}
		for _, seg := range in.Current.Narration.Segments {
			if inside(seg.StartMS) {
				segments = append(segments, map[string]any{"id": seg.ID, "text": seg.Text, "start_ms": seg.StartMS, "end_ms": seg.EndMS})
			}
		}
		scope["spoken_segments"] = segments
	}
	payload["revision_scope"] = scope
	return payload
}
