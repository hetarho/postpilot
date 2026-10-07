package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"time"
)

func analysisVerificationTask(s clip.AnalysisPreparation) clip.AnalysisVerificationTask {
	t := clip.AnalysisVerificationTask{Version: 1, ProfileVersion: s.ProfileVersion, ManifestDigest: s.ManifestDigest}
	for _, c := range s.Copies {
		c.ObjectKey, c.State, c.PutExpiresAt = "", "", time.Time{}
		c.Info = clip.MediaInfo{}
		t.Copies = append(t.Copies, c)
	}
	return t
}
func (a *AnalysisPreparations) Claim(ctx context.Context, profile clip.MediaWorkerProfile) (out *clip.MediaWork, err error) {
	if profile.Operation != clip.MediaVerifyAnalysis || !profile.Compatible() || profile.Validate() != nil {
		return nil, clip.ErrMediaIncompatible
	}
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		now := a.now().UTC()
		lease, e := p.Analysis.ClaimAnalysisVerification(ctx, profile, a.limits, now)
		if e != nil || lease == nil {
			return e
		}
		s, e := p.Analysis.AuthorizeAnalysisLease(ctx, lease.Credentials, now, false)
		if e != nil {
			return e
		}
		if e = p.Analysis.AuthorizeAnalysisPreparation(ctx, s, now); e != nil {
			return e
		}
		if e = a.parent(ctx, p, s, true); e != nil {
			return e
		}
		task := analysisVerificationTask(s)
		if e = task.Validate(); e != nil {
			return e
		}
		stage := lease.Stage
		out = &clip.MediaWork{Credentials: lease.Credentials, Operation: clip.MediaVerifyAnalysis, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.AnalysisVerificationRenderer, AssetVersion: clip.AnalysisVerificationAssets, InputDigest: stage.InputDigest, Payload: stage.Payload, LeaseRemaining: lease.ExpiresAt.Sub(a.now()), HeartbeatAfter: lease.HeartbeatAfter, StageRemaining: stage.DeadlineAt.Sub(a.now())}
		return nil
	})
	return
}
func (a *AnalysisPreparations) lease(ctx context.Context, p Ports, auth clip.MediaLeaseCredentials, accepted bool) (clip.AnalysisPreparation, error) {
	s, e := p.Analysis.AuthorizeAnalysisLease(ctx, auth, a.now().UTC(), accepted)
	if e != nil {
		return s, e
	}
	if e = p.Analysis.AuthorizeAnalysisPreparation(ctx, s, a.now().UTC()); e != nil {
		return s, e
	}
	e = a.parent(ctx, p, s, true)
	return s, e
}
func (a *AnalysisPreparations) Renew(ctx context.Context, auth clip.MediaLeaseCredentials, progress int) (remaining time.Duration, err error) {
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		if _, e := a.lease(ctx, p, auth, false); e != nil {
			return e
		}
		end, e := p.Analysis.RenewAnalysisLease(ctx, auth, progress, a.limits, a.now().UTC())
		remaining = max(end.Sub(a.now()), 0)
		return e
	})
	return
}
func (a *AnalysisPreparations) Read(ctx context.Context, auth clip.MediaLeaseCredentials, slot string) (out clip.MediaArtifactAccess, err error) {
	var copy clip.AnalysisCopy
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		s, e := a.lease(ctx, p, auth, false)
		if e != nil {
			return e
		}
		for _, c := range s.Copies {
			if c.Slot == slot && c.State == "reserved" {
				copy = c
				return nil
			}
		}
		return clip.ErrNotFound
	})
	if err != nil {
		return
	}
	out, err = a.objects.PresignMediaRead(ctx, copy.ObjectKey, min(a.limits.Stages.LeaseTTL, clip.MediaArtifactAccessTTL))
	if err != nil {
		return clip.MediaArtifactAccess{}, errors.New("analysis copy access signing failed")
	}
	out.Slot, out.ContentType, out.Bytes = copy.Slot, "video/mp4", copy.Bytes
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error { _, e := a.lease(ctx, p, auth, false); return e })
	return
}
func (a *AnalysisPreparations) CompleteVerification(ctx context.Context, auth clip.MediaLeaseCredentials, receipt string) error {
	if len(receipt) > clip.MediaPayloadMaxBytes {
		return clip.ErrInvalid
	}
	var result clip.AnalysisVerificationResult
	if clip.StrictJSON(receipt, &result) != nil {
		return clip.ErrInvalid
	}
	return WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		s, e := a.lease(ctx, p, auth, true)
		if e != nil {
			return e
		}
		task := analysisVerificationTask(s)
		if e = task.Validate(); e != nil {
			return e
		}
		if e = clip.ValidateAnalysisVerification(task, result, a.cfg); e != nil {
			return e
		}
		if e = p.Analysis.AcceptAnalysisReceipt(ctx, auth, receipt, result, a.now().UTC()); e != nil {
			return e
		}
		if s.ParentJobID != "" {
			_, e = p.Waits.Wake(ctx, s.ParentJobID, AnalysisPreparationWaitKey(s.ID), a.now().UTC())
		}
		return e
	})
}
func (a *AnalysisPreparations) Fail(ctx context.Context, auth clip.MediaLeaseCredentials, failure clip.MediaFailure) error {
	if !failure.Valid() {
		return clip.ErrInvalid
	}
	return WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		s, e := p.Analysis.AuthorizeAnalysisLease(ctx, auth, a.now().UTC(), false)
		if e != nil && !(errors.Is(e, clip.ErrMediaCancelled) && failure == clip.MediaFailureCancelled) {
			return e
		}
		if failure == clip.MediaFailureCancelled {
			if s.ParentJobID != "" {
				j, e := p.Jobs.GetByID(ctx, s.ParentJobID)
				if e != nil {
					return e
				}
				if j.CancelRequestedAt == nil {
					return clip.ErrMediaLeaseLost
				}
			}
			return p.Analysis.StopAnalysisAttempt(ctx, auth.AttemptID, "cancelled", a.now().UTC())
		}
		if e = p.Analysis.StopAnalysisAttempt(ctx, auth.AttemptID, "failed", a.now().UTC()); e != nil {
			return e
		}
		if failure == clip.MediaFailureWorkerLost && s.Attempts < a.limits.Stages.MaxAttempts {
			return nil
		}
		return p.Analysis.SetAnalysisPreparationState(ctx, s.ID, "failed", string(failure))
	})
}
func (a *AnalysisPreparations) Status(ctx context.Context, worker string) (clip.MediaRuntimeStatus, error) {
	return a.store.AnalysisVerificationStatus(ctx, worker, a.limits.Stages.MaxAttempts, a.now().UTC())
}
func (a *AnalysisPreparations) ReserveOutputs(context.Context, clip.MediaLeaseCredentials, []clip.MediaOutput) ([]clip.MediaArtifactAccess, error) {
	return nil, clip.ErrMediaUnsupported
}

// MediaWorkerRouter retains the native lease protocol and routes only the new
// namespaced session leases to the independently bounded verification service.
type MediaWorkerRouter struct {
	native   *MediaWorker
	analysis *AnalysisPreparations
}

func NewMediaWorkerRouter(native *MediaWorker, analysis *AnalysisPreparations) *MediaWorkerRouter {
	if native == nil || analysis == nil {
		panic("clip: media worker routing requires both owned services")
	}
	return &MediaWorkerRouter{native, analysis}
}
func (r *MediaWorkerRouter) Claim(ctx context.Context, p clip.MediaWorkerProfile) (*clip.MediaWork, error) {
	if p.Operation == clip.MediaVerifyAnalysis {
		return r.analysis.Claim(ctx, p)
	}
	return r.native.Claim(ctx, p)
}
func (r *MediaWorkerRouter) Renew(ctx context.Context, a clip.MediaLeaseCredentials, p int) (time.Duration, error) {
	if _, ok := clip.AnalysisPreparationStage(a.StageID); ok {
		return r.analysis.Renew(ctx, a, p)
	}
	return r.native.Renew(ctx, a, p)
}
func (r *MediaWorkerRouter) Complete(ctx context.Context, a clip.MediaLeaseCredentials, result string) error {
	if _, ok := clip.AnalysisPreparationStage(a.StageID); ok {
		return r.analysis.CompleteVerification(ctx, a, result)
	}
	return r.native.Complete(ctx, a, result)
}
func (r *MediaWorkerRouter) Fail(ctx context.Context, a clip.MediaLeaseCredentials, f clip.MediaFailure) error {
	if _, ok := clip.AnalysisPreparationStage(a.StageID); ok {
		return r.analysis.Fail(ctx, a, f)
	}
	return r.native.Fail(ctx, a, f)
}
func (r *MediaWorkerRouter) Read(ctx context.Context, a clip.MediaLeaseCredentials, s string) (clip.MediaArtifactAccess, error) {
	if _, ok := clip.AnalysisPreparationStage(a.StageID); ok {
		return r.analysis.Read(ctx, a, s)
	}
	return r.native.Read(ctx, a, s)
}
func (r *MediaWorkerRouter) Reserve(ctx context.Context, a clip.MediaLeaseCredentials, o []clip.MediaOutput) ([]clip.MediaArtifactAccess, error) {
	if _, ok := clip.AnalysisPreparationStage(a.StageID); ok {
		return nil, clip.ErrMediaUnsupported
	}
	return r.native.Reserve(ctx, a, o)
}
func (r *MediaWorkerRouter) Status(ctx context.Context, w string) (clip.MediaRuntimeStatus, error) {
	return r.native.Status(ctx, w)
}
func (r *MediaWorkerRouter) StatusForRole(ctx context.Context, worker, role string) (clip.MediaRuntimeStatus, error) {
	switch role {
	case clip.NativeWorkerRole:
		return r.native.Status(ctx, worker)
	case clip.AnalysisVerificationRole:
		return r.analysis.Status(ctx, worker)
	default:
		return clip.MediaRuntimeStatus{}, clip.ErrMediaUnsupported
	}
}

type AnalysisQueue interface {
	Cancel(context.Context, string, job.Subject, string) (*job.JobSummary, error)
	FailWait(context.Context, string, string, job.Failure) (bool, error)
	AcknowledgeWaitCancellation(context.Context, string, string) (bool, error)
}
type AnalysisJobs struct{ queue AnalysisQueue }

func NewAnalysisJobs(queue AnalysisQueue) AnalysisJobs {
	if queue == nil {
		panic("clip: analysis recovery needs parent cancellation and wait behavior")
	}
	return AnalysisJobs{queue}
}
func (a AnalysisJobs) CancelJob(ctx context.Context, user, project, id string) error {
	_, e := a.queue.Cancel(ctx, user, job.Subject{Dimension: clip.JobSubject, ID: project}, id)
	return e
}
func (a AnalysisJobs) FailWait(ctx context.Context, id, key string, f job.Failure) (bool, error) {
	return a.queue.FailWait(ctx, id, key, f)
}
func (a AnalysisJobs) AcknowledgeWaitCancellation(ctx context.Context, id, key string) (bool, error) {
	return a.queue.AcknowledgeWaitCancellation(ctx, id, key)
}

// These JSON envelopes belong exclusively to verification v1, never to the
// existing strict native v3 MediaTask/MediaInfo records.
func encodeAnalysisVerification(t clip.AnalysisVerificationTask) (string, error) {
	b, e := json.Marshal(t)
	return string(b), e
}
