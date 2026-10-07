package ai

import (
	"context"
	_ "embed"
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
	if in.Target != clip.RevisionNarration {
		flow, spent, e := s.write(ctx, model, in.PlanningInput, "flow", "flow-revision", s.cfg.FlowCompletionTokens, RevisionFlowSchema(), func() (string, string) {
			return buildFlowRevisionPrompt(in, s.cfg.Render.FadeMS, compositionLimits(s.cfg, in.PlanningInput))
		}, func(raw string) (clip.EditPlan, error) { return parseFlowPlan(s.cfg, in.PlanningInput, raw, false) })
		usage = addUsage(usage, spent)
		if e != nil {
			return clip.EditPlan{}, usage, e
		}
		// Flow is replaced while the owner's displayed wording and output intervals remain.
		raw, e := clip.EncodeEditPlan(in.Current)
		if e != nil {
			return clip.EditPlan{}, usage, e
		}
		next, e = clip.DecodeEditPlan(raw)
		if e != nil {
			return clip.EditPlan{}, usage, e
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
	}
	result, spent, e := s.write(ctx, model, in.PlanningInput, "script", "spoken-script-revision", s.cfg.NarrationCompletionTokens, spokenRevisionSchema, func() (string, string) {
		return buildSpokenRevisionPrompt(in, next, compositionLimits(s.cfg, in.PlanningInput))
	}, func(raw string) (clip.EditPlan, error) {
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
	})
	usage = addUsage(usage, spent)
	return result, usage, e
}

// buildSpokenRevisionPrompt is shared by the actual admitted seam and its synthetic inventory.
func buildSpokenRevisionPrompt(in clip.RevisionInput, next clip.EditPlan, limits composition.Limits) (string, string) {
	lines := []map[string]any{}
	for _, seg := range in.Current.Narration.Segments {
		lines = append(lines, map[string]any{"id": seg.ID, "text": seg.Text, "start_ms": seg.StartMS, "end_ms": seg.EndMS})
	}
	_, user := BuildStorylinePrompt(clip.StorylineInput{PlanningInput: in.PlanningInput}, limits)
	system := spokenScriptRule
	system += "\nFor this revision return ONLY spoken_lines under the supplied closed contract. Rewrite the dedicated spoken script in response to revision_request. Visible captions and all owner-authored words are independent and immutable. Never synthesize or change audio speed.\n" + compactContract(spokenRevisionSchema)
	user += promptJSON(map[string]any{"current_spoken_script": lines, "revision_request": in.Request, "output_duration_ms": next.DurationMS})
	return system, user
}
