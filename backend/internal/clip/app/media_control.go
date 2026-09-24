package app

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
)

type MediaControlTx interface {
	ClaimMediaStage(context.Context, clip.MediaWorkerProfile, time.Time) (*clip.MediaLease, error)
	RenewMediaLease(context.Context, clip.MediaLeaseCredentials, int, time.Time) (time.Time, error)
	FailMediaStage(context.Context, clip.MediaLeaseCredentials, clip.MediaFailure, time.Time) error
	AcknowledgeMediaStop(context.Context, clip.MediaLeaseCredentials, time.Time) error
}

// Control commits the worker's lease change with the owning job/project guard.
// The store alone cannot authorize another context's parent job.
type MediaControl struct {
	MediaWorkerStore
	writer *sql.DB
	bind   Binder
}

func NewMediaControl(writer *sql.DB, bind Binder, store MediaWorkerStore) *MediaControl {
	return &MediaControl{MediaWorkerStore: store, writer: writer, bind: bind}
}
func (m *MediaControl) ClaimMediaStage(ctx context.Context, profile clip.MediaWorkerProfile, now time.Time) (lease *clip.MediaLease, err error) {
	err = WriteTx(ctx, m.writer, m.bind, func(p Ports) error {
		var err error
		lease, err = p.Control.ClaimMediaStage(ctx, profile, now)
		if err != nil || lease == nil {
			return err
		}
		task, err := mediacodec.DecodeTask(lease.Stage.Payload)
		if err != nil {
			return err
		}
		return authorizeMediaParent(ctx, p, lease.Stage, task, now)
	})
	if err != nil {
		return nil, err
	}
	return lease, nil
}
func (m *MediaControl) RenewMediaLease(ctx context.Context, auth clip.MediaLeaseCredentials, progress int, now time.Time) (end time.Time, err error) {
	err = WriteTx(ctx, m.writer, m.bind, func(p Ports) error {
		stage, err := p.Media.AuthorizeMediaLease(ctx, auth, now, false)
		if err != nil {
			return err
		}
		task, err := mediacodec.DecodeTask(stage.Payload)
		if err != nil {
			return err
		}
		if err = authorizeMediaParent(ctx, p, stage, task, now); err != nil {
			return err
		}
		end, err = p.Control.RenewMediaLease(ctx, auth, progress, now)
		return err
	})
	return
}
func (m *MediaControl) FailMediaStage(ctx context.Context, auth clip.MediaLeaseCredentials, failure clip.MediaFailure, now time.Time) error {
	return WriteTx(ctx, m.writer, m.bind, func(p Ports) error {
		state, err := p.Recovery.MediaRecoveryState(ctx, auth.StageID)
		if err != nil {
			return err
		}
		j, err := p.Jobs.GetByID(ctx, state.Stage.ParentJobID)
		if err != nil {
			return err
		}
		if failure == clip.MediaFailureCancelled {
			if j.CancelRequestedAt == nil {
				return clip.ErrMediaLeaseLost
			}
			return p.Control.AcknowledgeMediaStop(ctx, auth, now)
		}
		return p.Control.FailMediaStage(ctx, auth, failure, now)
	})
}
