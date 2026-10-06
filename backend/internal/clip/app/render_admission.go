package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/job"
)

// Product-agnostic job primitives; only this saga names native render work.
type RenderJobTx interface {
	LockAdmission(context.Context) error
	ActiveCount(context.Context, job.Filter) (int, error)
	Insert(context.Context, job.Job) error
	SetWaitExpiry(context.Context, string, string, time.Time) error
	Activate(context.Context, string, string) (bool, error)
}
type RenderSourceTx interface {
	LinkRenderSourceJob(context.Context, string, string, string, int, time.Time) error
}

type RenderAdmission struct {
	writer   *sql.DB
	bind     Binder
	capacity clip.RenderCapacity
	stages   clip.MediaStageLimits
	now      func() time.Time
}

func NewRenderAdmission(writer *sql.DB, bind Binder, capacity clip.RenderCapacity, stages clip.MediaStageLimits, now func() time.Time) (*RenderAdmission, error) {
	if writer == nil || bind == nil {
		return nil, clip.ErrCompositionUnavailable
	}
	if err := capacity.Validate(); err != nil {
		return nil, err
	}
	if err := stages.Validate(); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &RenderAdmission{writer, bind, capacity, stages, now}, nil
}

// Already admitted work keeps its rights even after a plan/benefit change.
func (a *RenderAdmission) Existing(ctx context.Context, user, project, batch string, revision int, plan string) (id string, err error) {
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		active, err := p.Jobs.ActiveFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: project}, job.Filter{UserID: user})
		if err != nil || active == nil {
			return err
		}
		var frozen renderPayload
		if active.Kind == clip.JobKindRender && json.Unmarshal(active.Payload, &frozen) == nil && frozen.Revision == revision && frozen.PlanJSON == plan && frozen.Batch.ID == batch {
			id = active.ID
			return nil
		}
		return clip.ErrBusy
	})
	return
}

// Admit freezes the deadline with the job, source ownership and commercial
// reservation in one transaction. Durable jobs are the capacity authority;
// terminal writes release capacity without a second mutable counter.
func (a *RenderAdmission) Admit(ctx context.Context, in clip.GenerationStart, batch string, revision int, task clip.MediaTask, operator bool) (id string, err error) {
	payload, err := mediacodec.EncodeTask(task)
	if err != nil {
		return "", err
	}
	reused := false
	now := a.now().UTC()
	defer func() {
		outcome := "accepted"
		if reused {
			outcome = "reused"
		}
		if errors.Is(err, clip.ErrRenderOverloaded) {
			outcome = "overloaded"
		} else if errors.Is(err, clip.ErrRenderAccountBusy) {
			outcome = "account_busy"
		} else if err != nil {
			outcome = "refused"
		}
		slog.Info("clip native admission", "outcome", outcome, "elapsed_ms", max(a.now().Sub(now).Milliseconds(), 0))
	}()
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		if p.Starts == nil || p.RenderSources == nil || p.Stages == nil || p.Exports == nil {
			return clip.ErrCompositionUnavailable
		}
		// Take the writer position before any reads, including with a second API
		// process: no read-to-write snapshot upgrade can race admission.
		if err := p.Starts.LockAdmission(ctx); err != nil {
			return err
		}
		active, err := p.Jobs.ActiveFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: in.ProjectID}, job.Filter{UserID: in.UserID})
		if err != nil {
			return err
		}
		if active != nil {
			if active.Kind == clip.JobKindRender && bytes.Equal(active.Payload, in.Payload) {
				id = active.ID
				reused = true
				return nil
			}
			return clip.ErrBusy
		}
		owned, err := p.Starts.ActiveCount(ctx, job.Filter{Kind: clip.JobKindRender, UserID: in.UserID})
		if err != nil {
			return err
		}
		if owned >= a.capacity.PerAccount {
			return clip.ErrRenderAccountBusy
		}
		total, err := p.Starts.ActiveCount(ctx, job.Filter{Kind: clip.JobKindRender})
		if err != nil {
			return err
		}
		if total >= a.capacity.Active+a.capacity.Waiting {
			return clip.ErrRenderOverloaded
		}
		id = newID()
		if !operator {
			if err := p.Exports.ReserveExport(ctx, in.UserID, in.ProjectID, revision, id, now); err != nil {
				return err
			}
		}
		if err := p.Starts.Insert(ctx, job.Job{ID: id, UserID: in.UserID, Kind: clip.JobKindRender, Subjects: []job.Subject{{Dimension: clip.JobSubject, ID: in.ProjectID}}, Payload: in.Payload, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		if err := p.RenderSources.LinkRenderSourceJob(ctx, in.UserID, batch, id, revision, now); err != nil {
			return err
		}
		if !operator {
			if err := p.Exports.BindExport(ctx, id, id); err != nil {
				return err
			}
		}
		if _, err := p.Stages.CreateMediaStage(ctx, clip.MediaStageInput{ID: newID(), ParentJobID: id, UserID: in.UserID, ProjectID: in.ProjectID, ExpectedRevision: revision, Operation: clip.MediaRender, ContractVersion: clip.MediaContractVersion, InputDigest: clip.MediaPayloadDigest(payload), Payload: payload, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Limits: a.stages}, now); err != nil {
			return err
		}
		if err := p.Starts.SetWaitExpiry(ctx, id, "render_wait", now.Add(a.stages.WaitTimeout)); err != nil {
			return err
		}
		ok, err := p.Starts.Activate(ctx, in.UserID, id)
		if err != nil {
			return err
		}
		if !ok {
			return job.ErrNotFound
		}
		return nil
	})
	if errors.Is(err, job.ErrActiveConflict) {
		err = clip.ErrBusy
	}
	if errors.Is(err, job.ErrInvalidTarget) {
		err = clip.ErrNotFound
	}
	if err != nil {
		id = ""
	}
	return
}
