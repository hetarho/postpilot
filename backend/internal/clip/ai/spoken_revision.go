package ai

import (
	"context"
	_ "embed"
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
	"slices"
)

//go:embed schemas/spoken-revision.schema.json
var spokenRevisionSchema []byte

func (s *Service) reviseSpoken(ctx context.Context, model llm.ModelRef, in clip.RevisionInput) (clip.EditPlan, llm.Usage, error) {
	next := in.Current
	usage := llm.Usage{}
	rewrite := in.Target != clip.RevisionNarration
	var flow clip.EditPlan
	if in.Target == clip.RevisionAuto {
		// The narrated path routes the same way: a kept flow returns the spoken script itself.
		routed, changed, spent, e := s.route(ctx, model, in, "spoken-revision-route", spokenRouteOutputSchema, spokenRouteShape, func() (string, string) {
			return buildSpokenRoutePrompt(in, s.cfg.Render.FadeMS, compositionLimits(s.cfg, in.PlanningInput))
		}, func(raw string) (clip.EditPlan, error) { return s.parseSpokenRevision(in, in.Current, raw) })
		usage = addUsage(usage, spent)
		if e != nil || !changed {
			return routed, usage, e
		}
		flow = routed
	} else if rewrite {
		written, spent, e := s.write(ctx, model, in.PlanningInput, "flow", "flow-revision", s.cfg.FlowCompletionTokens, RevisionFlowSchema(), func() (string, string) {
			return buildFlowRevisionPrompt(in, s.cfg.Render.FadeMS, compositionLimits(s.cfg, in.PlanningInput))
		}, func(raw string) (clip.EditPlan, error) { return parseFlowPlan(s.cfg, in.PlanningInput, raw, false) })
		usage = addUsage(usage, spent)
		if e != nil {
			return clip.EditPlan{}, usage, e
		}
		flow = written
	}
	if rewrite {
		var e error
		if next, e = keepOwnerCaptions(in.Current, flow); e != nil {
			return clip.EditPlan{}, usage, e
		}
	}
	result, spent, e := s.write(ctx, model, in.PlanningInput, "script", "spoken-script-revision", s.cfg.NarrationCompletionTokens, structuralSchema(spokenRevisionSchema), func() (string, string) {
		return buildSpokenRevisionPrompt(in, next, compositionLimits(s.cfg, in.PlanningInput))
	}, func(raw string) (clip.EditPlan, error) { return s.parseSpokenRevision(in, next, raw) })
	usage = addUsage(usage, spent)
	return result, usage, e
}

// keepOwnerCaptions replaces the flow while the owner's displayed wording and output
// intervals remain.
func keepOwnerCaptions(current, flow clip.EditPlan) (clip.EditPlan, error) {
	raw, e := clip.EncodeEditPlan(current)
	if e != nil {
		return clip.EditPlan{}, e
	}
	next, e := clip.DecodeEditPlan(raw)
	if e != nil {
		return clip.EditPlan{}, e
	}
	next.Cuts, next.DurationMS = flow.Cuts, flow.DurationMS
	for i := range next.Portable.Elements {
		t := &next.Portable.Elements[i]
		if t.Resolved.Element.Role != "caption" {
			continue
		}
		a, b := t.Resolved.StartMS, t.Resolved.EndMS
		t.Resolved.Element.Basis = "output-start"
		t.Resolved.Element.StartMS = &a
		t.Resolved.Element.EndMS = &b
	}
	return next, nil
}

// parseSpokenRevision reads spoken_lines into the narration of next, reusing each
// unchanged segment's identity so its speech stays current (DUB-20).
func (s *Service) parseSpokenRevision(in clip.RevisionInput, next clip.EditPlan, raw string) (clip.EditPlan, error) {
	var wire struct {
		Lines []string `json:"spoken_lines"`
	}
	if e := decode(raw, s.cfg.MaxResponseBytes, readShape(spokenRevisionSchema), &wire); e != nil {
		return clip.EditPlan{}, e
	}
	candidate := next
	n := *in.Current.Narration
	n.Segments = nil
	n.RetiredSegments = slices.Clone(in.Current.Narration.RetiredSegments)
	used := map[string]bool{}
	cursor := 0
	for i, text := range wire.Lines {
		var seg clip.SpokenSegment
		for _, old := range in.Current.Narration.Segments {
			if old.Text == text && !used[old.ID] {
				seg = old
				break
			}
		}
		if seg.ID == "" && i < len(in.Current.Narration.Segments) && !used[in.Current.Narration.Segments[i].ID] {
			seg = in.Current.Narration.Segments[i]
		}
		if seg.ID == "" {
			seg = clip.SpokenSegment{Creation: true, Text: text, StartMS: cursor, EndMS: cursor + 1}
		} else {
			used[seg.ID] = true
			seg.Text = text
		}
		cursor = seg.EndMS
		n.Segments = append(n.Segments, seg)
	}
	if e := clip.CorrectNarration(in.Current, clip.CorrectionPlan{Narration: &n}, &candidate); e != nil {
		return clip.EditPlan{}, e
	}
	if e := validatePlan(s.cfg, in.PlanningInput, candidate); e != nil {
		return clip.EditPlan{}, e
	}
	return candidate, nil
}

func spokenLinesPayload(in clip.RevisionInput) []map[string]any {
	lines := []map[string]any{}
	for _, seg := range in.Current.Narration.Segments {
		lines = append(lines, map[string]any{"id": seg.ID, "text": seg.Text, "start_ms": seg.StartMS, "end_ms": seg.EndMS})
	}
	return lines
}

// spokenRevisionContract is the spoken script revision's own rules and response
// contract, without the shared writing contract, guide rule or 영상 지침.
func spokenRevisionContract(in clip.RevisionInput) string {
	contract := spokenScriptRule
	contract += "\nFor this revision return ONLY spoken_lines under the supplied closed contract. Rewrite the dedicated spoken script in response to revision_request, preserving lines the request does not ask to change. Visible captions and all owner-authored words are independent and immutable. Never synthesize or change audio speed.\n" + responseContract
	if in.Policy.StructuredOutput {
		contract += "Use the supplied response schema."
	} else {
		contract += compactContract(spokenRevisionSchema)
	}
	return contract
}

// buildSpokenRevisionPrompt is shared by the actual admitted seam and its synthetic inventory.
func buildSpokenRevisionPrompt(in clip.RevisionInput, next clip.EditPlan, limits composition.Limits) (string, string) {
	_, user := BuildStorylinePrompt(clip.StorylineInput{PlanningInput: in.PlanningInput}, limits)
	var data map[string]any
	_ = json.Unmarshal([]byte(user), &data)
	delete(data, "intro_outro")
	if shown := regionTextPayload(in.PlanningInput, limits); len(shown) > 0 {
		data["intro_outro"] = shown
	}
	user = promptJSON(data)
	system := videoWritingContract(in.PlanningInput) + spokenRevisionContract(in)
	system += scopeRule(in) + advertiserGuideRule(in.PlanningInput) + videoGuidelineBlock(videoGuidelinesFor(in.Guidelines, "spoken-script-revision"))
	user += promptJSON(withScope(map[string]any{"current_spoken_script": spokenLinesPayload(in), "revision_request": in.Request, "output_duration_ms": next.DurationMS}, in))
	return system, user
}
