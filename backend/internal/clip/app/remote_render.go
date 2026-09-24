package app

import (
	"context"
	"errors"
	"reflect"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
)

func freezeRenderSourceAudio(batch clip.SourceBatch, plan clip.EditPlan) *clip.SourceAudioSettings {
	// The owner's switch belongs to the current lease, while the plan keeps
	// its retained identity across a compatible re-upload.
	batch.Sources = slices.Clone(batch.Sources)
	for i, source := range batch.Sources {
		for _, cut := range plan.Cuts {
			if source.Fingerprint == cut.Fingerprint {
				batch.Sources[i].ID = cut.SourceID
				break
			}
		}
	}
	return clip.FreezeSourceAudio(batch, plan.Cuts)
}

func freezeRenderTask(plan clip.EditPlan, retained []clip.AnalysisSource, batch clip.SourceBatch, cfg clip.MediaConfig) (clip.MediaTask, error) {
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		return clip.MediaTask{}, err
	}
	task := clip.MediaTask{Version: clip.MediaContractVersion, Plan: raw, HideDisclosure: plan.HideDisclosure, Render: clip.FreezeMediaRenderInputs(plan)}
	for _, lease := range renderBatchSources(plan, batch).Sources {
		matched := false
		for _, old := range retained {
			if old.Fingerprint == lease.Fingerprint {
				task.Sources = append(task.Sources, clip.MediaTaskSource{ID: old.ID, SourceMetadata: lease.SourceMetadata, Info: old.Info})
				matched = true
				break
			}
		}
		if !matched {
			return task, clip.ErrSourceState
		}
	}
	return task, clip.ValidateMediaTask(clip.MediaRender, task, cfg)
}

func (s *GenerationService) resolveLegacyRenderTask(ctx context.Context, p clip.Project, b clip.SourceBatch, frozen renderPayload) (clip.MediaTask, error) {
	plan, err := clip.DecodeEditPlan(frozen.PlanJSON)
	if err != nil {
		return clip.MediaTask{}, err
	}
	if plan.Portable == nil {
		t := clip.VideoTemplate{}
		if p.Composition != nil && p.Composition.Snapshot.LegacyRecipe != nil {
			t.Recipe = *p.Composition.Snapshot.LegacyRecipe
		} else if p.VideoTemplateID != "" {
			t, err = s.projects.store.GetTemplate(ctx, p.UserID, p.VideoTemplateID)
			if err != nil && !errors.Is(err, clip.ErrNotFound) {
				return clip.MediaTask{}, err
			}
		}
		plan = plan.WithFacts(p.Disclosure, p.Answers, t.Preset, p.CTA, t.Accent, frozen.HideDisclosure)
	}
	plan = plan.WithDesign(p.DesignSelection())
	plan.HideDisclosure = frozen.HideDisclosure
	plan.SourceAudio = freezeRenderSourceAudio(frozen.Batch, plan)
	refs := make([]clip.RenderSource, len(frozen.Sources))
	for i, source := range frozen.Sources {
		refs[i] = source.RenderSource
	}
	if validator, ok := s.renderer.(clip.RenderPlanValidator); ok {
		plan, err = validator.ValidateRenderPlan(ctx, plan, refs)
	} else if plan.Portable != nil {
		layout, ok := s.renderer.(clip.CompositionLayouter)
		if !ok {
			return clip.MediaTask{}, clip.ErrCompositionUnavailable
		}
		plan, _, err = layout.LayoutComposition(ctx, plan, refs)
	} else {
		layout, ok := s.renderer.(clip.PlanLayouter)
		if !ok {
			return clip.MediaTask{}, clip.ErrCompositionUnavailable
		}
		plan, _, err = layout.Layout(ctx, plan, refs)
	}
	if err != nil {
		return clip.MediaTask{}, err
	}
	return freezeRenderTask(plan, frozen.Sources, b, s.cfg.Media)
}

type mediaResultObjects interface {
	HeadMediaArtifact(context.Context, string) (clip.SourceObjectInfo, error)
}

func (s *GenerationService) runRemoteRender(ctx context.Context, user, parent string, p clip.Project, b clip.SourceBatch, frozen renderPayload, set func(string, int, int)) error {
	set("prepare", 0, len(b.Sources))
	task := frozen.Execution
	if task == nil {
		var err error
		task, err = s.remoteMedia.FrozenTask(ctx, user, parent, clip.MediaRender)
		if err != nil {
			return err
		}
	}
	if task == nil {
		resolved, err := s.resolveLegacyRenderTask(ctx, p, b, frozen)
		if err != nil {
			return err
		}
		task = &resolved
	}
	stage, artifacts, err := s.remoteMedia.Request(ctx, MediaDispatchRequest{UserID: user, JobID: parent, ProjectID: p.ID, Revision: frozen.Revision, Operation: clip.MediaRender, Task: *task})
	if err != nil {
		return err
	}
	result, err := mediacodec.DecodeResult(stage.AcceptedResult)
	if err != nil {
		return err
	}
	if err = clip.ValidateMediaResult(clip.MediaRender, *task, result, s.cfg.Media); err != nil {
		return err
	}
	if len(artifacts) != 1 || artifacts[0].State != "accepted" || artifacts[0].AttemptID != stage.CurrentAttemptID || !reflect.DeepEqual(artifacts[0].MediaOutput, result.Outputs[0]) {
		return clip.ErrInvalidMedia
	}
	a := artifacts[0]
	objects, ok := s.objects.(mediaResultObjects)
	if !ok {
		return clip.ErrMediaUnavailable
	}
	info, err := objects.HeadMediaArtifact(ctx, a.ObjectKey)
	if err != nil {
		return err
	}
	if info.Bytes != a.Bytes || info.ContentType != a.ContentType {
		return clip.ErrInvalidMedia
	}
	set("save", 0, 1)
	err = s.finisher.Complete(ctx, clip.AttemptResult{JobID: parent, UserID: user, ProjectID: p.ID, ExpectedRevision: frozen.Revision, Result: clip.Result{Kind: clip.RenderServer, Key: a.ObjectKey, ContentType: a.ContentType, Bytes: a.Bytes, DurationMS: a.DurationMS, CreatedAt: a.CreatedAt}})
	if err == nil {
		set("cleanup", 0, 1)
	}
	return err
}
