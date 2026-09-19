package app

import (
	"context"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// SetOriginalSound records the owner's 원본 소리 유지 choice for one source.
//
// It is the ONLY way the setting changes: no template, model, render or plan
// save reaches it (CLIP-100, CDS-6). The store performs the whole change — the
// lease, the plan's snapshot, the revision and the retention renewal — inside
// one writer transaction, because a lease that disagrees with the saved plan
// would render audio the owner never asked for.
func (s *SourceService) SetOriginalSound(ctx context.Context, user string, change clip.SourceAudioChange) (clip.SourceBatch, clip.Project, error) {
	if strings.TrimSpace(user) == "" || strings.TrimSpace(change.ProjectID) == "" || strings.TrimSpace(change.BatchID) == "" ||
		strings.TrimSpace(change.SourceID) == "" || strings.TrimSpace(change.Fingerprint) == "" || change.ExpectedRevision < 0 {
		return clip.SourceBatch{}, clip.Project{}, clip.ErrInvalid
	}
	batch, project, err := s.store.SetSourceOriginalAudio(ctx, user, change)
	if err != nil {
		return clip.SourceBatch{}, clip.Project{}, err
	}
	now := s.now()
	for i := range batch.Sources {
		batch.Sources[i].Availability = clip.SourceAvailability(batch, batch.Sources[i], now)
	}
	return batch, project, nil
}

// batchSourceAudio states every declared source's original-sound choice for the
// writer to read (CLIP-129). It is the batch's own snapshot, not a plan's: it
// is stated before any cut exists.
func batchSourceAudio(batch clip.SourceBatch) []clip.SourceAudioSetting {
	out := make([]clip.SourceAudioSetting, 0, len(batch.Sources))
	for _, lease := range batch.Sources {
		out = append(out, clip.SourceAudioSetting{SourceID: lease.ID, Fingerprint: lease.Fingerprint, RetainOriginal: lease.RetainOriginalAudio})
	}
	return out
}
