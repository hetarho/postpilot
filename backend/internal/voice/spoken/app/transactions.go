// Package app coordinates spoken metadata with durable jobs through owning ports.
package app

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/voice/spoken"
	"time"
)

type JobMetadata interface {
	GetByID(context.Context, string) (job.Job, error)
	ActiveUnattached(context.Context, string, string) (*job.Job, error)
	AuthorizeDispatch(context.Context, string, string) error
	RequestOwnedCancellation(context.Context, string, string, time.Time) error
	Finish(context.Context, string, string, *job.Failure, time.Time) error
}
type TxPorts struct {
	Operations spoken.OperationStorage
	Jobs       JobMetadata
}
type Transactions interface {
	Write(context.Context, func(TxPorts) error) error
}
type Coordinator struct{ Transactions Transactions }

func matchingJob(ctx context.Context, p TxPorts, o spoken.Operation) (job.Job, error) {
	j, err := p.Jobs.GetByID(ctx, o.JobID)
	if err != nil {
		return j, err
	}
	if j.UserID != o.OwnerID || j.Kind != o.Kind || string(j.Payload) != o.ID {
		return j, spoken.ErrNotFound
	}
	return j, nil
}
func (c Coordinator) Claim(ctx context.Context, owner, operationID, jobID string) error {
	return c.Transactions.Write(ctx, func(p TxPorts) error {
		o, err := p.Operations.GetOperation(ctx, owner, operationID)
		if err != nil {
			return err
		}
		if o.JobID != "" && o.JobID != jobID {
			return spoken.ErrConflict
		}
		o.JobID = jobID
		j, err := matchingJob(ctx, p, o)
		if err != nil {
			return err
		}
		if j.Status != job.StatusRunning || j.CancelRequestedAt != nil {
			return spoken.ErrOperationStopped
		}
		if o.Kind != spoken.JobKindProbe {
			if err := p.Jobs.AuthorizeDispatch(ctx, owner, jobID); err != nil {
				return err
			}
		}
		return p.Operations.ClaimOperation(ctx, owner, operationID, jobID)
	})
}
func (c Coordinator) ClaimProbe(ctx context.Context, owner, id string, index int) error {
	return c.Transactions.Write(ctx, func(p TxPorts) error {
		o, err := p.Operations.GetOperation(ctx, owner, id)
		if err != nil {
			return err
		}
		if _, err := matchingJob(ctx, p, o); err != nil {
			return err
		}
		if err := p.Jobs.AuthorizeDispatch(ctx, owner, o.JobID); err != nil {
			return err
		}
		return p.Operations.ClaimProbeCall(ctx, owner, id, index)
	})
}
func (c Coordinator) Cancel(ctx context.Context, owner, id string) error {
	return c.Transactions.Write(ctx, func(p TxPorts) error {
		o, err := p.Operations.GetOperation(ctx, owner, id)
		if err != nil {
			return err
		}
		if o.Terminal() {
			return nil
		}
		if o.JobID == "" {
			return p.Operations.CancelOperation(ctx, owner, id)
		}
		j, err := matchingJob(ctx, p, o)
		if err != nil {
			return err
		}
		if job.Terminal(j.Status) {
			return p.Operations.CancelOperation(ctx, owner, id)
		}
		if err := p.Jobs.RequestOwnedCancellation(ctx, owner, j.ID, time.Now()); err != nil {
			return err
		}
		return p.Operations.CancelOperation(ctx, owner, id)
	})
}

// PublicationStorage wraps only the worker's library. Audition uploads happen
// outside this method; its transaction orders publication against cancellation.
type PublicationStorage struct {
	spoken.OperationStorage
	Transactions Transactions
	OperationID  string
}

func (s PublicationStorage) Mutate(ctx context.Context, key spoken.RequestIdentity, fn func(spoken.Storage) (spoken.MutationResult, error)) (result spoken.MutationResult, err error) {
	err = s.Transactions.Write(ctx, func(p TxPorts) error {
		o, e := p.Operations.GetOperation(ctx, key.OwnerID, s.OperationID)
		if e != nil {
			return e
		}
		if o.State != spoken.OperationClaimed && o.State != spoken.OperationReceived {
			return spoken.ErrOperationStopped
		}
		if key.Operation != "save_candidates" && key.Operation != "confirm_voice" {
			return spoken.ErrInvalid
		}
		if (key.Operation == "save_candidates" && o.Kind != spoken.JobKindDesign) || (key.Operation == "confirm_voice" && o.Kind != spoken.JobKindConfirm) {
			return spoken.ErrInvalid
		}
		j, e := matchingJob(ctx, p, o)
		if e != nil {
			return e
		}
		if j.CancelRequestedAt != nil || j.Status == job.StatusCancelled {
			return spoken.ErrOperationStopped
		}
		// A known received handle may finish metadata publication after a storage
		// failure. Historical failed accounting remains settled and is never reopened.
		if j.Status != job.StatusRunning && (o.State != spoken.OperationReceived || o.ReceivedHandle == "" || j.Status != job.StatusFailed) {
			return spoken.ErrOperationStopped
		}
		result, e = p.Operations.Mutate(ctx, key, fn)
		if e != nil {
			return e
		}
		if e := p.Operations.PublishOperation(ctx, o.OwnerID, o.ID, result.ID); e != nil {
			return e
		}
		if !job.Terminal(j.Status) {
			return p.Jobs.Finish(ctx, j.ID, job.StatusDone, nil, time.Now())
		}
		return nil
	})
	return
}
func (c Coordinator) PublishProbe(ctx context.Context, owner, id string) error {
	return c.Transactions.Write(ctx, func(p TxPorts) error {
		o, err := p.Operations.GetOperation(ctx, owner, id)
		if err != nil {
			return err
		}
		j, err := matchingJob(ctx, p, o)
		if err != nil {
			return err
		}
		if j.Status != job.StatusRunning || j.CancelRequestedAt != nil || o.AssetIDs[0] == "" || o.AssetIDs[1] == "" {
			return spoken.ErrOperationStopped
		}
		if err := p.Operations.PublishOperation(ctx, owner, id, o.VoiceID); err != nil {
			return err
		}
		return p.Jobs.Finish(ctx, j.ID, job.StatusDone, nil, time.Now())
	})
}
func (c Coordinator) FailInterrupted(ctx context.Context, o spoken.Operation) error {
	return c.Transactions.Write(ctx, func(p TxPorts) error {
		current, err := p.Operations.GetOperation(ctx, o.OwnerID, o.ID)
		if err != nil {
			return err
		}
		if current.Terminal() {
			return nil
		}
		if current.JobID == "" {
			active, err := p.Jobs.ActiveUnattached(ctx, current.OwnerID, current.Kind)
			if err != nil {
				return err
			}
			if active == nil || string(active.Payload) != current.ID {
				return p.Operations.FailOperation(ctx, current.OwnerID, current.ID, "JOB_NOT_CREATED", false)
			}
			if err := p.Operations.BindOperation(ctx, current.OwnerID, current.ID, active.ID); err != nil {
				return err
			}
			current.JobID = active.ID
		}
		j, err := matchingJob(ctx, p, current)
		if err != nil {
			return err
		}
		if j.Status == job.StatusQueued && (current.State == spoken.OperationQueued || current.State == spoken.OperationReserved) {
			return nil
		}
		if j.CancelRequestedAt != nil || j.Status == job.StatusCancelled {
			if err := p.Operations.CancelOperation(ctx, current.OwnerID, current.ID); err != nil {
				return err
			}
			if !job.Terminal(j.Status) {
				return p.Jobs.Finish(ctx, j.ID, job.StatusCancelled, nil, time.Now())
			}
			return nil
		}
		if err := p.Operations.FailOperation(ctx, current.OwnerID, current.ID, job.FailureReasonInterrupted, current.Kind == spoken.JobKindConfirm); err != nil {
			return err
		}
		if !job.Terminal(j.Status) {
			return p.Jobs.Finish(ctx, j.ID, job.StatusFailed, &job.Failure{Reason: job.FailureReasonInterrupted}, time.Now())
		}
		return nil
	})
}
func publicationStale(err error) bool {
	return errors.Is(err, spoken.ErrConflict) || errors.Is(err, spoken.ErrNotFound) || errors.Is(err, spoken.ErrImmutable) || errors.Is(err, spoken.ErrOperationStopped)
}
