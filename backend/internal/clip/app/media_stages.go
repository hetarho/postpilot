package app

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
	"time"
)

// MediaStageStore is the atomic lease behavior needed by the internal worker
// use cases. A transaction-bound implementation can also join parent admission.
type MediaStageStore interface {
	CreateMediaStage(context.Context, clip.MediaStageInput, time.Time) (clip.MediaStage, error)
	ClaimMediaStage(context.Context, clip.MediaWorkerProfile, time.Time) (*clip.MediaLease, error)
	RenewMediaLease(context.Context, clip.MediaLeaseCredentials, int, time.Time) (time.Time, error)
	AcceptMediaResult(context.Context, clip.MediaLeaseCredentials, string, time.Time) (clip.MediaStage, error)
	ReserveMediaArtifact(context.Context, clip.MediaLeaseCredentials, clip.MediaArtifactReservation, time.Time) error
}

// MediaStages supplies API-authoritative time and freezes deployment limits.
// Parent authorization/cancellation is composed by the admitting saga; no
// production dispatcher consumes this foundation until the continuation lands.
type MediaStages struct {
	store  MediaStageStore
	limits clip.MediaStageLimits
	now    func() time.Time
}

func NewMediaStages(store MediaStageStore, limits clip.MediaStageLimits, now func() time.Time) (*MediaStages, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &MediaStages{store: store, limits: limits, now: now}, nil
}

func (m *MediaStages) Create(ctx context.Context, in clip.MediaStageInput) (clip.MediaStage, error) {
	in.Limits = m.limits
	return m.store.CreateMediaStage(ctx, in, m.now().UTC())
}
func (m *MediaStages) Claim(ctx context.Context, p clip.MediaWorkerProfile) (*clip.MediaLease, error) {
	return m.store.ClaimMediaStage(ctx, p, m.now().UTC())
}
func (m *MediaStages) Heartbeat(ctx context.Context, a clip.MediaLeaseCredentials, progress int) (time.Time, error) {
	return m.store.RenewMediaLease(ctx, a, progress, m.now().UTC())
}
func (m *MediaStages) Complete(ctx context.Context, a clip.MediaLeaseCredentials, result string) (clip.MediaStage, error) {
	return m.store.AcceptMediaResult(ctx, a, result, m.now().UTC())
}
func (m *MediaStages) ReserveOutput(ctx context.Context, a clip.MediaLeaseCredentials, out clip.MediaArtifactReservation) error {
	return m.store.ReserveMediaArtifact(ctx, a, out, m.now().UTC())
}
