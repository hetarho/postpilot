package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) beginBrowserRender(ctx context.Context, p clip.Project, plan clip.EditPlan, sources []clip.RenderSource) (string, error) {
	store, ok := s.store.(clip.BrowserRenderStore)
	if !ok {
		return "", clip.ErrRenderUnavailable
	}
	r := clip.BrowserRender{ID: newID(), UserID: p.UserID, ProjectID: p.ID, Revision: p.EditPlanRevision, Ratio: plan.Ratio, DurationMS: plan.DurationMS, CreatedAt: s.now()}
	for _, cut := range plan.Cuts {
		for _, source := range sources {
			if source.ID == cut.SourceID && source.Info.HasAudio && plan.RetainsOriginalAudio(cut) {
				r.Audio = true
			}
		}
	}
	if err := store.BeginBrowserRender(ctx, r); err != nil {
		return "", err
	}
	return r.ID, nil
}

// ReportRenderVerdict consumes BoundedText measurements, never an object key or
// media bytes. The upload/promotion path consumes this retained verdict later.
func (s *GenerationService) ReportRenderVerdict(ctx context.Context, user, id string, measurements clip.RenderMeasurements, passed bool) (clip.RenderVerdict, error) {
	store, ok := s.store.(clip.BrowserRenderStore)
	if !ok {
		return clip.RenderVerdict{}, clip.ErrRenderUnavailable
	}
	r, err := store.GetBrowserRender(ctx, user, id)
	if err != nil {
		return clip.RenderVerdict{}, err
	}
	v, err := clip.CheckRenderMeasurements(s.cfg.Render, r, measurements)
	if err != nil {
		return clip.RenderVerdict{}, err
	}
	v.ReportedPassed = passed
	if !passed && v.Passed {
		v.Passed = false
		v.Notices = []clip.PlanNotice{{CopyFallback: clip.CopyFallback{Reason: "render_output_verdict"}, Action: "shortfall"}}
	}
	if err := store.SaveBrowserRenderVerdict(ctx, user, id, v, s.now()); err != nil {
		return clip.RenderVerdict{}, err
	}
	return v, nil
}
