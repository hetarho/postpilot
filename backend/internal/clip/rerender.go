package clip

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"reflect"
	"time"
)

type CaptionSizer interface {
	CaptionSize(context.Context, string, Caption) (float64, float64, error)
}

func (s *GenerationService) EditingState(p Project) (*CorrectionState, error) {
	return EditingState(p, s.cfg.Render)
}
func (s *GenerationService) SaveCorrection(ctx context.Context, user, id string, revision int, input CorrectionPlan) (Project, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return Project{}, err
	}
	if p.EditPlanRevision != revision || revision <= 0 {
		return Project{}, ErrPlanConflict
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return Project{}, err
	}
	if active != nil {
		return Project{}, ErrBusy
	}
	next, styles, err := ApplyCorrection(s.cfg.Render, p, input)
	if err != nil {
		return Project{}, err
	}
	sizer, ok := s.renderer.(CaptionSizer)
	if !ok {
		return Project{}, ErrInvalid
	}
	// The same bundled font/measurement used by rendering. No source or provider.
	for _, c := range next.Cuts {
		for _, copy := range c.Copies {
			if _, _, err = sizer.CaptionSize(ctx, next.Ratio, copy); err != nil {
				break
			}
		}
		if err != nil {
			return Project{}, err
		}
	}
	raw, err := EncodeEditPlan(next, styles)
	if err != nil {
		return Project{}, err
	}
	return s.store.SaveCorrection(ctx, user, id, revision, raw)
}

type renderPayload struct {
	Version   int
	ProjectID string
	Revision  int
	PlanJSON  string
	Sources   []AnalysisSource
	Batch     SourceBatch
}

func (s *GenerationService) StartRender(ctx context.Context, user, id, batch string, revision int) (string, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return "", err
	}
	if revision <= 0 || p.EditPlanRevision != revision {
		return "", ErrPlanConflict
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return "", err
	}
	if active != nil {
		return "", ErrBusy
	}
	plan, _, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return "", err
	}
	plan, _, err = ApplyCorrection(s.cfg.Render, p, CorrectionFromPlan(plan))
	if err != nil {
		return "", err
	}
	sources, err := RetainedSources(p)
	if err != nil {
		return "", err
	}
	b, err := s.store.GetSourceBatch(ctx, user, batch)
	if err != nil {
		return "", err
	}
	if b.ProjectID != id || b.State != "ready" || !time.Now().Before(b.ExpiresAt) {
		return "", ErrSourceState
	}
	for _, v := range b.Sources {
		if v.State != "ready" || v.ActualBytes != v.Bytes {
			return "", ErrSourceState
		}
	}
	if err = MatchRenderBatch(plan, b); err != nil {
		return "", err
	}
	raw, err := json.Marshal(renderPayload{1, id, revision, p.EditPlan, sources, b})
	if err != nil {
		return "", err
	}
	return s.enqueue(ctx, GenerationStart{UserID: user, ProjectID: id, Payload: raw, RenderOnly: true}, batch, revision)
}

func (s *GenerationService) RunRender(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	stage := "prepare"
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.CleanupTimeout)
		defer cancel()
		if b, e := s.store.BatchForJob(cleanup, user, job); e == nil {
			if e = s.sources.Finish(cleanup, user, b.ID); e != nil {
				slog.Warn("clip render cleanup deferred", "job", job)
			}
		}
		if err != nil {
			err = &StageFailure{stage, err}
		}
	}()
	b, err := s.store.BatchForJob(ctx, user, job)
	if err != nil {
		return err
	}
	var frozen renderPayload
	if strictJSON(string(payload), &frozen) != nil || frozen.Version != 1 || frozen.ProjectID != project || frozen.Batch.ID != b.ID || frozen.Batch.UserID != user || !reflect.DeepEqual(frozen.Batch.Sources, b.Sources) {
		return ErrInvalid
	}
	if b.State != "consuming" {
		return ErrSourceState
	}
	p, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	if p.EditPlanRevision != frozen.Revision || p.EditPlan != frozen.PlanJSON {
		return ErrPlanConflict
	}
	retained, e := RetainedSources(p)
	if e != nil || !reflect.DeepEqual(retained, frozen.Sources) {
		return ErrInvalid
	}
	plan, _, err := DecodeEditPlan(frozen.PlanJSON)
	if err != nil {
		return err
	}
	// The badge and the chips are read fresh from the project and its template,
	// so a re-render always carries the owner's current campaign type.
	t, err := s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID)
	if err != nil {
		return err
	}
	plan = plan.WithFacts(p.Disclosure, p.Answers, t.Preset, p.CTA, t.Accent).WithStyles(t.CopyStyles)
	if err = MatchRenderBatch(plan, b); err != nil {
		return err
	}
	set := func(name string, n, total int) { stage = name; progress(name, n, total) }
	var result Result
	err = s.media.WithWorkspace(ctx, job, func(ws MediaWorkspace) error {
		set("prepare", 0, len(b.Sources))
		actual, infos, _, err := s.probeBatch(ctx, ws, b, func(n int) { set("prepare", n, len(b.Sources)) })
		if err != nil {
			return err
		}
		// Rebind fresh leases to retained identities only after actual media agrees.
		refs := []RenderSource{}
		leases := map[string]int{}
		for i, a := range actual {
			matched := false
			for _, old := range frozen.Sources {
				if old.Fingerprint == a.Fingerprint {
					if old.Info.DurationMS != a.Info.DurationMS || old.Info.Width != a.Info.Width || old.Info.Height != a.Info.Height || old.Info.HasAudio != a.Info.HasAudio {
						return ErrInvalidMedia
					}
					refs = append(refs, old.RenderSource)
					leases[old.ID] = i
					matched = true
					break
				}
			}
			if !matched {
				return ErrSourceState
			}
		}
		if err = ValidateEditPlan(s.cfg.Render, plan, refs); err != nil {
			return err
		}
		set("render", 0, 1)
		video, err := s.renderer.Render(ctx, ws, plan, refs, func(ctx context.Context, id string, fn func(MediaSource) error) error {
			i, ok := leases[id]
			if !ok {
				return ErrNotFound
			}
			return s.withSource(ctx, ws, b.Sources[i], infos[i], func(source MediaSource) error { source.SourceID = id; return fn(source) })
		})
		if err != nil {
			return err
		}
		set("save", 0, 1)
		key := ResultPrefix + url.PathEscape(user) + "/" + url.PathEscape(project) + "/" + newID() + ".mp4"
		if err = s.uploadPath(ctx, key, video.Path, video.Bytes); err != nil {
			return err
		}
		result = Result{Key: key, ContentType: "video/mp4", Bytes: video.Bytes, DurationMS: video.Info.DurationMS, CreatedAt: time.Now()}
		return nil
	})
	if err != nil {
		return err
	}
	if err = s.store.SaveRender(ctx, user, project, frozen.Revision, result); err != nil {
		return err
	}
	set("cleanup", 0, 1)
	return nil
}
