package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) EditingState(p clip.Project) (*clip.CorrectionState, error) {
	return clip.EditingStateOf(p, s.cfg.Render, s.CompositionCapability() >= clip.CompositionPlanVersion)
}

func (s *GenerationService) SaveCorrection(ctx context.Context, user, id string, revision int, input clip.CorrectionPlan) (clip.Project, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.Project{}, err
	}
	if p.Finalized != nil {
		return clip.Project{}, clip.ErrFinalized
	}
	if err := s.checkComposition(p.Composition); err != nil {
		return clip.Project{}, err
	}
	if p.EditPlanRevision != revision || revision <= 0 {
		return clip.Project{}, clip.ErrPlanConflict
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return clip.Project{}, err
	}
	if active != nil {
		return clip.Project{}, clip.ErrBusy
	}
	next, err := clip.ApplyCorrection(s.cfg.Render, p, input)
	if err != nil {
		return clip.Project{}, err
	}
	next = next.WithDesign(p.DesignSelection())
	if next.Portable != nil {
		layout, ok := s.renderer.(clip.CompositionLayouter)
		if !ok {
			return clip.Project{}, clip.ErrCompositionUnavailable
		}
		sources, err := clip.RetainedSources(p)
		if err != nil {
			return clip.Project{}, err
		}
		refs := make([]clip.RenderSource, 0, len(sources))
		for _, source := range sources {
			refs = append(refs, source.RenderSource)
		}
		next, _, err = layout.LayoutComposition(ctx, next, refs)
		if err != nil {
			return clip.Project{}, err
		}
		raw, err := clip.EncodeEditPlan(next)
		if err != nil {
			return clip.Project{}, err
		}
		return s.store.SaveCorrection(ctx, user, id, revision, raw)
	}
	sizer, ok := s.renderer.(clip.CaptionSizer)
	if !ok {
		return clip.Project{}, clip.ErrInvalid
	}
	// The same bundled font/measurement used by rendering. No source or provider.
	for _, c := range next.Cuts {
		for _, copy := range c.Copies {
			if _, _, err = sizer.CaptionSize(ctx, next.Ratio, copy); err != nil {
				break
			}
		}
		if err != nil {
			return clip.Project{}, err
		}
	}
	if p.Composition != nil && p.Composition.Snapshot.Legacy && p.Composition.Snapshot.LegacyRecipe != nil {
		next.Portable, err = clip.FreezeLegacyPlan(p, next, *p.Composition.Snapshot.LegacyRecipe, s.projects.limits.Composition)
		if err != nil {
			return clip.Project{}, err
		}
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil {
		return clip.Project{}, err
	}
	return s.store.SaveCorrection(ctx, user, id, revision, raw)
}

type renderPayload struct {
	HideDisclosure bool
	Version        int
	ProjectID      string
	Revision       int
	PlanJSON       string
	Sources        []clip.AnalysisSource
	Batch          clip.SourceBatch
}

func (s *GenerationService) StartRender(ctx context.Context, user, id, batch string, revision int, kind clip.RenderKind) (string, error) {
	switch kind {
	case clip.RenderServer, clip.RenderBrowser:
	default:
		return "", clip.ErrInvalid
	}
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return "", err
	}
	if p.Finalized != nil {
		return "", clip.ErrFinalized
	}
	if err := s.checkComposition(p.Composition); err != nil {
		return "", err
	}
	if revision <= 0 || p.EditPlanRevision != revision {
		return "", clip.ErrPlanConflict
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return "", err
	}
	if active != nil {
		return "", clip.ErrBusy
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		return "", err
	}
	plan, err = clip.ApplyCorrection(s.cfg.Render, p, clip.CorrectionFromPlan(plan))
	if err != nil {
		return "", err
	}
	if err := clip.ValidateCompositionEvidence(plan); err != nil {
		return "", err
	}
	sources, err := clip.RetainedSources(p)
	if err != nil {
		return "", err
	}
	b, err := s.sources.AvailableBatch(ctx, user, batch, plan)
	if err != nil {
		return "", err
	}
	if b.ProjectID != id || b.State != "ready" || !time.Now().Before(b.ExpiresAt) {
		return "", clip.ErrSourceState
	}
	// The owner's per-source sound choice is frozen HERE, from the leases that
	// own it, so the job payload carries what the owner has chosen right now
	// rather than whatever a plan was last saved with (CLIP-100).
	plan.SourceAudio = clip.FreezeSourceAudio(b, plan.Cuts)
	for _, v := range b.Sources {
		if v.State != "ready" || v.ActualBytes != v.Bytes {
			return "", clip.ErrSourceState
		}
	}
	if err = clip.MatchRenderBatch(plan, b); err != nil {
		return "", err
	}
	// Resolve the same bundled-font layout and checks before either executor
	// starts. Pixel-dependent contrast and output checks stay with the producer.
	if plan.Portable == nil {
		t := clip.VideoTemplate{}
		if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
			t.Recipe = *p.Composition.Snapshot.LegacyRecipe
		} else if p.VideoTemplateID != "" {
			t, err = s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID)
			if err != nil && !errors.Is(err, clip.ErrNotFound) {
				return "", err
			}
		}
		plan = plan.WithFacts(p.Disclosure, p.Answers, t.Preset, p.CTA, t.Accent, p.HideDisclosure)
	}
	plan = plan.WithDesign(p.DesignSelection())
	refs := make([]clip.RenderSource, 0, len(sources))
	for _, source := range sources {
		refs = append(refs, source.RenderSource)
	}
	if err := clip.RefuseUnrenderableRates(plan, refs); err != nil {
		return "", err
	}
	if validator, ok := s.renderer.(clip.RenderPlanValidator); ok {
		plan, err = validator.ValidateRenderPlan(ctx, plan, refs)
	} else if plan.Portable != nil {
		layout, ok := s.renderer.(clip.CompositionLayouter)
		if !ok {
			return "", clip.ErrCompositionUnavailable
		}
		plan, _, err = layout.LayoutComposition(ctx, plan, refs)
	} else {
		layout, ok := s.renderer.(clip.PlanLayouter)
		if !ok {
			return "", clip.ErrCompositionUnavailable
		}
		plan, _, err = layout.Layout(ctx, plan, refs)
	}
	if err != nil {
		return "", err
	}
	if kind == clip.RenderBrowser {
		return s.beginBrowserRender(ctx, p, plan, refs)
	}
	raw, err := json.Marshal(renderPayload{HideDisclosure: p.HideDisclosure, Version: 1, ProjectID: id, Revision: revision, PlanJSON: p.EditPlan, Sources: sources, Batch: b})
	if err != nil {
		return "", err
	}
	return s.enqueue(ctx, clip.GenerationStart{UserID: user, ProjectID: id, Payload: raw, RenderOnly: true}, batch, revision)
}

func (s *GenerationService) RunRender(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	if s.finisher == nil {
		return clip.ErrCompositionUnavailable
	}
	stage := "prepare"
	checkpoint := clip.AttemptCheckpoint{Version: 1, JobID: job, Stage: stage}
	defer func() {
		if err == nil {
			return
		}
		checkpoint.Stage = stage
		if d, ok := clip.DiagnosticFromError(err); ok {
			checkpoint.Diagnostic = d
		}
		if err != nil {
			logAttemptDiagnostic(job, stage, checkpoint.Diagnostic)
		}
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.CleanupTimeout)
		defer cancel()
		s.checkpoint(recordCtx, user, project, checkpoint)
	}()
	defer func() {
		if err != nil {
			err = &clip.StageFailure{Stage: stage, Cause: err}
		}
	}()
	b, err := s.store.BatchForJob(ctx, user, job)
	if err != nil {
		return err
	}
	var frozen renderPayload
	if clip.StrictJSON(string(payload), &frozen) != nil || frozen.Version != 1 || frozen.ProjectID != project || frozen.Batch.ID != b.ID || frozen.Batch.UserID != user || !clip.SameSourceManifest(frozen.Batch.Sources, b.Sources) {
		return clip.ErrInvalid
	}
	if b.State != "consuming" {
		return clip.ErrSourceState
	}
	p, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	if p.EditPlanRevision != frozen.Revision || p.EditPlan != frozen.PlanJSON || p.HideDisclosure != frozen.HideDisclosure {
		return clip.ErrPlanConflict
	}
	retained, e := clip.RetainedSources(p)
	if e != nil || !reflect.DeepEqual(retained, frozen.Sources) {
		return clip.ErrInvalid
	}
	plan, err := clip.DecodeEditPlan(frozen.PlanJSON)
	if err != nil {
		return err
	}
	plan = plan.WithDesign(p.DesignSelection())
	if err := s.checkComposition(p.Composition); err != nil {
		return err
	}
	if plan.Portable != nil && !plan.Portable.Snapshot.Legacy && s.CompositionCapability() < clip.CompositionPlanVersion {
		return clip.ErrCompositionUnavailable
	}
	// Manual rerender reads the frozen legacy recipe, including after template
	// edits or deletion. Project-local disclosure changes retain their meaning.
	t := clip.VideoTemplate{}
	if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
		t.Recipe = *p.Composition.Snapshot.LegacyRecipe
	} else if plan.Portable == nil && p.VideoTemplateID != "" {
		t, err = s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID)
		if err != nil && !errors.Is(err, clip.ErrNotFound) {
			return err
		}
	}

	if plan.Portable == nil {
		plan = plan.WithFacts(p.Disclosure, p.Answers, t.Preset, p.CTA, t.Accent, frozen.HideDisclosure)
	}
	if err = clip.MatchRenderBatch(plan, b); err != nil {
		return err
	}
	b = renderBatchSources(plan, b)
	observations, _ := clip.RetainedObservations(p)
	for _, a := range observations {
		checkpoint.Observations = append(checkpoint.Observations, clip.SourceAnalysis{Source: a.Source})
	}
	checkpoint.TotalSources = len(b.Sources)
	checkpoint.Diagnostic = clip.AttemptDiagnostic{Ranges: clip.AttemptRangeDiagnostics(plan, observations), Values: map[string]int{"cut_count": len(plan.Cuts), "target_ms": p.TargetDurationMS, "after_ms": plan.DurationMS}}
	set := func(name string, n, total int) {
		stage, checkpoint.Stage = name, name
		if name == "prepare" {
			checkpoint.Diagnostic.Values["source"] = min(n+1, total)
		}
		if progress != nil {
			progress(name, n, total)
		}
		if name != "cleanup" {
			s.checkpoint(ctx, user, project, checkpoint)
		}
	}
	var result clip.Result
	err = s.media.WithWorkspace(ctx, job, func(ws clip.MediaWorkspace) error {
		set("prepare", 0, len(b.Sources))
		actual, infos, _, err := s.probeBatch(ctx, ws, b, func(n int) { set("prepare", n, len(b.Sources)) })
		if err != nil {
			return err
		}
		// Rebind fresh leases to retained identities only after actual media agrees.
		refs := []clip.RenderSource{}
		leases := map[string]int{}
		for i, a := range actual {
			matched := false
			for _, old := range frozen.Sources {
				if old.Fingerprint == a.Fingerprint {
					if old.Info.DurationMS != a.Info.DurationMS || old.Info.Width != a.Info.Width || old.Info.Height != a.Info.Height || old.Info.HasAudio != a.Info.HasAudio {
						return clip.ErrInvalidMedia
					}
					refs = append(refs, old.RenderSource)
					leases[old.ID] = i
					matched = true
					break
				}
			}
			if !matched {
				return clip.ErrSourceState
			}
		}
		if err = clip.ValidateEditPlan(s.cfg.Render, plan, refs); err != nil {
			return err
		}
		set("render", 0, 1)
		load, releaseSource := s.renderLoader(ws, func(id string) (clip.SourceLease, clip.MediaInfo, bool) {
			i, ok := leases[id]
			if !ok {
				return clip.SourceLease{}, clip.MediaInfo{}, false
			}
			return b.Sources[i], infos[i], true
		})
		video, err := s.renderer.Render(ctx, ws, plan, refs, func(ctx context.Context, id string, fn func(clip.MediaSource) error) error {
			return load(ctx, id, func(source clip.MediaSource) error { source.SourceID = id; return fn(source) })
		})
		if err = errors.Join(err, releaseSource()); err != nil {
			return err
		}
		set("save", 0, 1)
		key := clip.ResultPrefix + url.PathEscape(user) + "/" + url.PathEscape(project) + "/" + newID() + ".mp4"
		if err = s.uploadPath(ctx, key, video.Path, video.Bytes); err != nil {
			return err
		}
		result = clip.Result{Kind: clip.RenderServer, Key: key, ContentType: "video/mp4", Bytes: video.Bytes, DurationMS: video.Info.DurationMS, CreatedAt: time.Now()}
		return nil
	})
	if err != nil {
		return err
	}
	if err = s.finisher.Complete(ctx, clip.AttemptResult{JobID: job, UserID: user, ProjectID: project, ExpectedRevision: frozen.Revision, Result: result}); err != nil {
		return err
	}
	set("cleanup", 0, 1)
	return nil
}
