package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/job"
)

// A browser render draws from what the server drew, and the scrim, the accent
// colour and the contrast notices the footage decides are part of that drawing
// (CLIP-159, CLIP-192). Footage is decoded on a media worker, never in the API
// (ARCH-45), so the render waits on a job that samples its grounds over the
// render's own frozen task before its assets are served.
type browserSamplePayload struct {
	Version   int
	RenderID  string
	ProjectID string
	Revision  int
	Batch     clip.SourceBatch
	Task      clip.MediaTask
}

// StartBrowserRender admits a browser render and starts the job its grounds are
// sampled by, returning both. Without a media worker (the in-process mode the
// tests run) no footage is read: the render is drawn on no ground at all and
// needs no job.
func (s *GenerationService) StartBrowserRender(ctx context.Context, user, id, batch string, revision int) (render, sampling string, err error) {
	started, err := s.startRender(ctx, user, id, batch, revision, clip.RenderBrowser)
	return started.renderID, started.jobID, err
}

func (s *GenerationService) startBrowserSampling(ctx context.Context, p clip.Project, plan clip.EditPlan, sources []clip.AnalysisSource, b clip.SourceBatch, render string, speech ...clip.SpeechAsset) (string, error) {
	store, ok := s.store.(clip.BrowserSamplingStore)
	if !ok {
		return "", clip.ErrRenderUnavailable
	}
	if s.remoteMedia == nil {
		// The render stands for its own sampling here, which read nothing.
		if err := store.BindBrowserRenderSampling(ctx, p.UserID, render, render); err != nil {
			return "", err
		}
		return "", store.SaveBrowserRenderGrounds(ctx, p.UserID, render, render, p.EditPlanRevision, nil, s.now())
	}
	task, err := freezeRenderTask(plan, sources, b, s.cfg.Media, speech...)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(browserSamplePayload{Version: 1, RenderID: render, ProjectID: p.ID, Revision: p.EditPlanRevision, Batch: b, Task: task})
	if err != nil {
		return "", err
	}
	sampling, err := s.enqueue(ctx, clip.GenerationStart{UserID: p.UserID, ProjectID: p.ID, Payload: raw, RenderOnly: true, Kind: clip.JobKindSampleBrowserRender}, b.ID, p.EditPlanRevision)
	if err != nil {
		return "", err
	}
	// The job binds itself too before it asks for its stage, so a start that
	// loses this write still leaves a render its own job can finish.
	if err := store.BindBrowserRenderSampling(ctx, p.UserID, render, sampling); err != nil {
		return "", err
	}
	return sampling, nil
}

// RunBrowserSampling is a sampling job's handler: one `sample` media stage over
// the frozen task, then the grounds it measured kept on the render.
func (s *GenerationService) RunBrowserSampling(ctx context.Context, user, id, project string, payload []byte, progress func(string, int, int)) error {
	store, ok := s.store.(clip.BrowserSamplingStore)
	if !ok || s.remoteMedia == nil {
		return clip.ErrMediaUnavailable
	}
	b, err := s.store.BatchForJob(ctx, user, id)
	if err != nil {
		return err
	}
	var frozen browserSamplePayload
	if clip.StrictJSON(string(payload), &frozen) != nil || frozen.Version != 1 || frozen.RenderID == "" || frozen.ProjectID != project || frozen.Revision <= 0 ||
		frozen.Batch.ID != b.ID || frozen.Batch.UserID != user || !clip.SameSourceManifest(frozen.Batch.Sources, b.Sources) {
		return clip.ErrInvalid
	}
	if b.State != "consuming" {
		return clip.ErrSourceState
	}
	if err := store.BindBrowserRenderSampling(ctx, user, frozen.RenderID, id); err != nil {
		return err
	}
	stage, _, err := s.remoteMedia.Request(ctx, MediaDispatchRequest{UserID: user, JobID: id, ProjectID: project, Revision: frozen.Revision, Operation: clip.MediaSample, Task: frozen.Task})
	if err != nil {
		return err
	}
	result, err := mediacodec.DecodeResult(stage.AcceptedResult)
	if err != nil {
		return err
	}
	if err := clip.ValidateMediaResult(clip.MediaSample, frozen.Task, result, s.cfg.Media); err != nil {
		return err
	}
	if progress != nil {
		progress("save", 0, 1)
	}
	return store.SaveBrowserRenderGrounds(ctx, user, frozen.RenderID, id, frozen.Revision, result.Grounds, s.now())
}

// jobCanceller is the queue's own stop, which the clip jobs port does not carry.
type jobCanceller interface {
	Cancel(ctx context.Context, user string, subject job.Subject, id string) (*job.JobSummary, error)
}

// CancelJob stops one of a project's jobs, when the queue offers a stop.
func (a Jobs) CancelJob(ctx context.Context, user, project, id string) error {
	q, ok := a.queue.(jobCanceller)
	if !ok {
		return job.ErrCancellationUnavailable
	}
	_, err := q.Cancel(ctx, user, job.Subject{Dimension: clip.JobSubject, ID: project}, id)
	return err
}

// stopBrowserSampling stops a cancelled render's sampling job, so no worker
// claims or keeps its stage (ARCH-51). A job that already ended is left as it is.
func (s *GenerationService) stopBrowserSampling(ctx context.Context, user string, r clip.BrowserRender) {
	if r.SampleJobID == "" || r.SampleJobID == r.ID {
		return
	}
	stopper, ok := s.jobs.(interface {
		CancelJob(context.Context, string, string, string) error
	})
	if !ok {
		return
	}
	if err := stopper.CancelJob(ctx, user, r.ProjectID, r.SampleJobID); err != nil && !errors.Is(err, job.ErrNotFound) {
		slog.Warn("clip browser render sampling was not stopped", "job", r.SampleJobID, "err", err)
	}
}
