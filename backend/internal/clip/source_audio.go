package clip

import (
	"context"
	"strings"
)

// SetOriginalSound records the owner's 원본 소리 유지 choice for one source.
//
// It is the ONLY way the setting changes: no template, model, render or plan
// save reaches it (CLIP-100, CDS-6). The store performs the whole change — the
// lease, the plan's snapshot, the revision and the retention renewal — inside
// one writer transaction, because a lease that disagrees with the saved plan
// would render audio the owner never asked for.
func (s *SourceService) SetOriginalSound(ctx context.Context, user string, change SourceAudioChange) (SourceBatch, Project, error) {
	if strings.TrimSpace(user) == "" || strings.TrimSpace(change.ProjectID) == "" || strings.TrimSpace(change.BatchID) == "" ||
		strings.TrimSpace(change.SourceID) == "" || strings.TrimSpace(change.Fingerprint) == "" || change.ExpectedRevision < 0 {
		return SourceBatch{}, Project{}, ErrInvalid
	}
	batch, project, err := s.store.SetSourceOriginalAudio(ctx, user, change)
	if err != nil {
		return SourceBatch{}, Project{}, err
	}
	now := s.now()
	for i := range batch.Sources {
		batch.Sources[i].Availability = SourceAvailability(batch, batch.Sources[i], now)
	}
	return batch, project, nil
}

// FreezeSourceAudio is the render-time snapshot: the LEASES are the owner-facing
// authority, so the payload an executor receives states what the owner has
// chosen right now rather than what a plan was saved with. A source the batch
// does not carry is off, because nothing has authorized its audio.
func FreezeSourceAudio(batch SourceBatch, cuts []Cut) *SourceAudioSettings {
	out := &SourceAudioSettings{}
	for _, key := range cutSourceKeys(cuts) {
		retain := false
		for _, lease := range batch.Sources {
			if lease.ID == key.id && lease.Fingerprint == key.fingerprint {
				retain = lease.RetainOriginalAudio
			}
		}
		out.Values = append(out.Values, SourceAudioSetting{key.id, key.fingerprint, retain})
	}
	return out
}

// batchSourceAudio states every declared source's original-sound choice for the
// writer to read (CLIP-129). It is the batch's own snapshot, not a plan's: it
// is stated before any cut exists.
func batchSourceAudio(batch SourceBatch) []SourceAudioSetting {
	out := make([]SourceAudioSetting, 0, len(batch.Sources))
	for _, lease := range batch.Sources {
		out = append(out, SourceAudioSetting{lease.ID, lease.Fingerprint, lease.RetainOriginalAudio})
	}
	return out
}

// ApplySourceAudio returns the plan with ONE source's setting changed and the
// snapshot left complete. It changes nothing else: cut ranges, rates, per-cut
// volume, captions, evidence and the frozen composition are untouched, which is
// what makes the change render-only (CLIP-100).
func ApplySourceAudio(plan EditPlan, sourceID, fingerprint string, retain bool) EditPlan {
	settings := ReconcileSourceAudio(plan.SourceAudio, plan.Cuts)
	for i := range settings.Values {
		if settings.Values[i].SourceID == sourceID && settings.Values[i].Fingerprint == fingerprint {
			settings.Values[i].RetainOriginal = retain
		}
	}
	plan.SourceAudio = settings
	return plan
}
