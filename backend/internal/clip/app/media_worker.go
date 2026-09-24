package app

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type MediaWorkerStore interface {
	MediaStageStore
	FailMediaStage(context.Context, clip.MediaLeaseCredentials, clip.MediaFailure, time.Time) error
	MediaRuntimeStatus(context.Context, string, time.Time) (clip.MediaRuntimeStatus, error)
	MediaIncompatible(context.Context, clip.MediaWorkerProfile, time.Time) (bool, error)
}

// MediaWorker is the execution-only internal API. Parent authorization is joined
// through the owning saga when production dispatch is enabled.
type MediaWorker struct {
	store MediaWorkerStore
	now   func() time.Time
}

func NewMediaWorker(store MediaWorkerStore, now func() time.Time) *MediaWorker {
	if now == nil {
		now = time.Now
	}
	return &MediaWorker{store: store, now: now}
}

func (m *MediaWorker) Claim(ctx context.Context, profile clip.MediaWorkerProfile) (*clip.MediaWork, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if !profile.Compatible() {
		return nil, clip.ErrMediaIncompatible
	}
	now := m.now().UTC()
	lease, err := m.store.ClaimMediaStage(ctx, profile, now)
	if err != nil {
		return nil, err
	}
	if lease == nil {
		incompatible, err := m.store.MediaIncompatible(ctx, profile, now)
		if err != nil {
			return nil, err
		}
		if incompatible {
			return nil, clip.ErrMediaIncompatible
		}
		return nil, nil
	}
	s := lease.Stage
	return &clip.MediaWork{Credentials: lease.Credentials, Operation: s.Operation, ContractVersion: s.ContractVersion, RendererVersion: s.RendererVersion, AssetVersion: s.AssetVersion, InputDigest: s.InputDigest, Payload: s.Payload, LeaseRemaining: lease.ExpiresAt.Sub(m.now()), HeartbeatAfter: lease.HeartbeatAfter, StageRemaining: s.DeadlineAt.Sub(m.now())}, nil
}

func (m *MediaWorker) Renew(ctx context.Context, lease clip.MediaLeaseCredentials, progress int) (time.Duration, error) {
	end, err := m.store.RenewMediaLease(ctx, lease, progress, m.now().UTC())
	if err != nil {
		return 0, err
	}
	return max(end.Sub(m.now()), 0), nil
}
func (m *MediaWorker) Complete(ctx context.Context, lease clip.MediaLeaseCredentials, result string) error {
	_, err := m.store.AcceptMediaResult(ctx, lease, result, m.now().UTC())
	return err
}
func (m *MediaWorker) Fail(ctx context.Context, lease clip.MediaLeaseCredentials, failure clip.MediaFailure) error {
	return m.store.FailMediaStage(ctx, lease, failure, m.now().UTC())
}
func (m *MediaWorker) Status(ctx context.Context, worker string) (clip.MediaRuntimeStatus, error) {
	return m.store.MediaRuntimeStatus(ctx, worker, m.now().UTC())
}
