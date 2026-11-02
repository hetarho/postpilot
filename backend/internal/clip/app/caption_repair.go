package app

import (
	"context"
	"errors"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// The refusal reasons the owner's notice copy names; any other refusal of an
// omitted caption is recorded as a plain omission.
var namedCaptionRefusals = []string{"copy_limit", "readability", "rapid_readability", "automatic_placement", "invalid_interval", "unsupported_glyph", "safe_area"}

// repairWriterCaptions keeps a writer-authored caption from stopping a render
// (CLIP-224). It runs the element checks the render start runs; a caption the
// writer authored and the owner never touched is first freed to automatic
// placement and, if still refused, omitted with one notice. Only a plan the
// checks then accept is saved, as the next revision, and the saved project
// returned; anything else returns the plan unchanged, so the start refuses it
// exactly as it would have.
func (s *GenerationService) repairWriterCaptions(ctx context.Context, user string, p clip.Project, plan clip.EditPlan) (clip.Project, clip.EditPlan, error) {
	sources, err := clip.RetainedSources(p)
	if err != nil {
		return p, plan, nil
	}
	refs := make([]clip.RenderSource, 0, len(sources))
	for _, source := range sources {
		refs = append(refs, source.RenderSource)
	}
	unchanged, changed := plan, false
	if plan.Portable != nil {
		portable := *plan.Portable
		portable.Elements = slices.Clone(portable.Elements)
		plan.Portable = &portable
	}
	freed := map[string]bool{}
	for attempts := 2*len(portableElements(plan)) + 1; ; attempts-- {
		raw, err := clip.EncodeEditPlan(plan)
		if err != nil {
			return p, unchanged, nil
		}
		candidate := p
		candidate.EditPlan = raw
		refusal := s.checkRenderElements(ctx, candidate, plan, refs)
		if refusal == nil {
			break
		}
		var problem *composition.Problem
		i := -1
		if errors.As(refusal, &problem) {
			i = writerCaption(plan, problem.ElementID)
		}
		if i < 0 || attempts <= 0 {
			return p, unchanged, nil
		}
		text := &plan.Portable.Elements[i]
		id := text.Resolved.Element.ID
		if !freed[id] && (text.Placement != nil || len(text.Phrases) > 0) {
			freed[id] = true
			text.Placement, text.Phrases, changed = nil, nil, true
			continue
		}
		cut := text.Resolved.CutID
		plan.Portable.Elements = slices.Delete(plan.Portable.Elements, i, i+1)
		reason := "dropped"
		if slices.Contains(namedCaptionRefusals, problem.Reason) {
			reason = problem.Reason
		}
		clip.AddPlanNotice(&plan, reason, cut, id, "omitted")
		changed = true
	}
	if !changed {
		return p, unchanged, nil
	}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		return p, unchanged, nil
	}
	saved, err := s.store.SaveCorrection(ctx, user, p.ID, p.EditPlanRevision, raw, nil)
	if err != nil {
		return p, unchanged, err
	}
	plan, err = clip.DecodeEditPlan(saved.EditPlan)
	return saved, plan, err
}

// checkRenderElements is the part of the render start that can refuse a
// single element: correction, evidence, rates and the layout over the
// project's own design.
func (s *GenerationService) checkRenderElements(ctx context.Context, p clip.Project, plan clip.EditPlan, refs []clip.RenderSource) error {
	plan, err := clip.ApplyCorrection(s.cfg.Render, p, clip.CorrectionFromPlan(plan))
	if err != nil {
		return err
	}
	if err := clip.ValidateCompositionEvidence(plan); err != nil {
		return err
	}
	plan = plan.WithDesign(p.DesignSelection())
	plan.HideDisclosure = p.HideDisclosure
	if err := clip.RefuseUnrenderableRates(plan, refs); err != nil {
		return err
	}
	if validator, ok := s.renderer.(clip.RenderPlanValidator); ok {
		_, err = validator.ValidateRenderPlan(ctx, plan, refs)
		return err
	}
	if layout, ok := s.renderer.(clip.CompositionLayouter); ok {
		_, _, err = layout.LayoutComposition(ctx, plan, refs)
	}
	return err
}

func portableElements(plan clip.EditPlan) []clip.PortableText {
	if plan.Portable == nil {
		return nil
	}
	return plan.Portable.Elements
}

// writerCaption is the index of the writer-authored caption a refusal names, or
// -1 when it names anything the owner made or touched.
func writerCaption(plan clip.EditPlan, element string) int {
	for i, text := range portableElements(plan) {
		r := text.Resolved
		if element == "" || (r.Element.ID != element && r.InstanceID != element) {
			continue
		}
		if r.Element.Role != "caption" || r.Element.Kind != "ai" || text.OwnerEdited || text.Owner != (clip.OwnerCaption{}) {
			return -1
		}
		return i
	}
	return -1
}
