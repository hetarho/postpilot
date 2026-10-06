package store

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func analysisPreparationRow(ctx context.Context, q *sqlc.Queries, row sqlc.ClipAnalysisPreparation) (clip.AnalysisPreparation, error) {
	var p clip.AnalysisPreparation
	if strictJSON(row.MetadataJson, &p) != nil {
		return p, clip.ErrInvalid
	}
	if p.ID != row.ID || p.UserID != row.UserID || p.ProjectID != row.ProjectID || p.BatchID != row.BatchID || p.QuoteID != row.QuoteID || p.ProfileVersion != row.ProfileVersion || p.ExpectedRevision != int(row.ExpectedRevision) {
		return p, clip.ErrInvalid
	}
	p.State, p.ParentJobID, p.CurrentAttemptID, p.Receipt, p.Failure = row.State, row.ParentJobID.String, row.CurrentAttemptID.String, row.AcceptedResult.String, row.Failure.String
	p.ManifestDigest, p.Attempts, p.Progress = row.ManifestDigest, int(row.AttemptCount), int(row.Progress)
	for _, v := range []struct {
		s string
		t *time.Time
	}{{row.CreatedAt, &p.CreatedAt}, {row.ExpiresAt, &p.ExpiresAt}, {row.QueueDeadlineAt, &p.QueueDeadlineAt}, {row.DeadlineAt, &p.DeadlineAt}} {
		value, err := time.Parse(time.RFC3339Nano, v.s)
		if err != nil {
			return p, err
		}
		*v.t = value
	}
	rows, err := q.AnalysisCopies(ctx, p.ID)
	if err != nil {
		return p, err
	}
	bySlot := map[string]sqlc.ClipAnalysisCopy{}
	for _, r := range rows {
		bySlot[r.Slot] = r
	}
	for i := range p.Copies {
		c := &p.Copies[i]
		r, ok := bySlot[c.Slot]
		if !ok || r.SourceID != c.SourceID || r.SourceFingerprint != c.Fingerprint || int(r.Ordinal) != c.Index || int(r.OffsetMs) != c.OffsetMS || int(r.DurationMs) != c.DurationMS || int(r.Width) != c.Width || int(r.Height) != c.Height || (r.HasAudio != 0) != c.HasAudio {
			return p, clip.ErrInvalid
		}
		c.Bytes, c.Digest, c.ObjectKey, c.State = r.ExactBytes, r.Sha256, r.ObjectKey.String, r.State
		if r.PutExpiresAt.Valid {
			c.PutExpiresAt, err = time.Parse(time.RFC3339Nano, r.PutExpiresAt.String)
			if err != nil {
				return p, err
			}
		}
		if r.VerificationJson.Valid {
			var v clip.AnalysisCopyVerification
			if strictJSON(r.VerificationJson.String, &v) != nil {
				return p, clip.ErrInvalid
			}
			c.Info = v.Info
		}
	}
	if len(rows) != len(p.Copies) {
		return p, clip.ErrInvalid
	}
	return p, nil
}

func (s *Store) GetAnalysisPreparation(ctx context.Context, user, id string) (clip.AnalysisPreparation, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.AnalysisPreparation, error) {
		r, e := q.GetAnalysisPreparation(ctx, sqlc.GetAnalysisPreparationParams{ID: id, UserID: user})
		if e != nil {
			return clip.AnalysisPreparation{}, dbError(e)
		}
		return analysisPreparationRow(ctx, q, r)
	})
}
func (s *Store) AnalysisPreparationForQuote(ctx context.Context, user, quote string) (clip.AnalysisPreparation, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.AnalysisPreparation, error) {
		r, e := q.AnalysisPreparationForQuote(ctx, sqlc.AnalysisPreparationForQuoteParams{QuoteID: quote, UserID: user})
		if e != nil {
			return clip.AnalysisPreparation{}, dbError(e)
		}
		return analysisPreparationRow(ctx, q, r)
	})
}

func (s *Store) AnalysisPreparationInputs(ctx context.Context, user string, in clip.AnalysisPreparationInput, now time.Time) (quote clip.GenerationQuote, batch clip.SourceBatch, recovery *clip.RecoveryState, err error) {
	r, err := s.read.GetClipQuote(ctx, sqlc.GetClipQuoteParams{ID: in.QuoteID, UserID: user})
	if err != nil {
		return quote, batch, nil, dbError(err)
	}
	quote, err = quoteRow(r)
	if err != nil {
		return
	}
	if quote.ProjectID != in.ProjectID || quote.BatchID != in.BatchID || quote.ConsumedJobID != "" {
		err = clip.ErrQuoteChanged
		return
	}
	if err = validateQuoteInputs(ctx, s.read, quote, now); err != nil {
		return
	}
	p, e := getProject(ctx, s.read, user, in.ProjectID)
	if e != nil {
		err = e
		return
	}
	if p.EditPlanRevision != in.ExpectedRevision {
		err = clip.ErrPlanConflict
		return
	}
	batch, err = getSourceBatch(ctx, s.read, user, in.BatchID)
	if err != nil {
		return
	}
	raw, e := s.read.GetClipRecovery(ctx, sqlc.GetClipRecoveryParams{UserID: user, ProjectID: in.ProjectID})
	if errors.Is(dbError(e), clip.ErrNotFound) {
		return
	}
	if e != nil {
		err = e
		return
	}
	recovery = &clip.RecoveryState{}
	if strictJSON(raw.StateJson, recovery) != nil {
		err = clip.ErrInvalid
	}
	return
}

// Only clip-owned rows participate here. Parent job authorization is supplied
// by the application through the job context's transaction-scoped ports.
func (s *Store) AuthorizeAnalysisPreparation(ctx context.Context, p clip.AnalysisPreparation, now time.Time) error {
	project, err := getProject(ctx, s.read, p.UserID, p.ProjectID)
	if err != nil {
		return err
	}
	access, err := s.read.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: p.ProjectID, UserID: p.UserID})
	if err != nil {
		return dbError(err)
	}
	if project.Finalized != nil || project.EditPlanRevision != p.ExpectedRevision || access.Deleting != 0 || access.SourceAccessRevokedAt.Valid || access.SourceBatchID.String != p.BatchID || !p.ExpiresAt.After(now) || !clip.AnalysisPreparationLive(p.State) {
		return clip.ErrAnalysisPreparationState
	}
	batch, err := getSourceBatch(ctx, s.read, p.UserID, p.BatchID)
	if err != nil {
		return err
	}
	if batch.State != "ready" && batch.State != "consuming" || len(batch.Sources) != len(p.Originals) {
		return clip.ErrSourceState
	}
	for i, m := range p.Originals {
		s := batch.Sources[i]
		if s.ID != m.SourceID || s.Fingerprint != m.Fingerprint || s.State != "ready" || s.CleanupPending || s.ActualBytes != s.Bytes || !s.ExpiresAt.After(now) {
			return clip.ErrSourceState
		}
	}
	return nil
}

func (s *Store) BeginAnalysisPreparation(ctx context.Context, p clip.AnalysisPreparation, l clip.AnalysisPreparationLimits, now time.Time) error {
	if l.Validate() != nil {
		return clip.ErrInvalid
	}
	return func() error {
		n, e := s.read.AnalysisPreparationCapacity(ctx, stamp(now))
		if e != nil {
			return e
		}
		if n >= int64(l.Capacity.Active+l.Capacity.Waiting) {
			return clip.ErrAnalysisPreparationOverloaded
		}
		n, e = s.read.AnalysisPreparationAccountCapacity(ctx, sqlc.AnalysisPreparationAccountCapacityParams{UserID: p.UserID, ExpiresAt: stamp(now)})
		if e != nil {
			return e
		}
		if n >= int64(l.Capacity.PerAccount) {
			return clip.ErrAnalysisPreparationOverloaded
		}
		if err := affected(s.write.BindAnalysisQuoteDigest(ctx, sqlc.BindAnalysisQuoteDigestParams{BoundDigest: p.BoundQuoteDigest, QuoteID: p.QuoteID, UserID: p.UserID, OriginalDigest: p.OriginalDigest, Now: stamp(now)})); err != nil {
			return err
		}
		data, e := json.Marshal(p)
		if e != nil {
			return e
		}
		if e = s.write.InsertAnalysisPreparation(ctx, sqlc.InsertAnalysisPreparationParams{ID: p.ID, UserID: p.UserID, ProjectID: p.ProjectID, BatchID: p.BatchID, QuoteID: p.QuoteID, ProfileVersion: p.ProfileVersion, ExpectedRevision: int64(p.ExpectedRevision), MetadataJson: string(data), ManifestDigest: p.ManifestDigest, CreatedAt: stamp(now), ExpiresAt: stamp(p.ExpiresAt), QueueDeadlineAt: stamp(p.QueueDeadlineAt), DeadlineAt: stamp(p.DeadlineAt)}); e != nil {
			return e
		}
		for _, c := range p.Copies {
			if e = s.write.InsertAnalysisCopy(ctx, sqlc.InsertAnalysisCopyParams{PreparationID: p.ID, Slot: c.Slot, SourceID: c.SourceID, SourceFingerprint: c.Fingerprint, Ordinal: int64(c.Index), OffsetMs: int64(c.OffsetMS), DurationMs: int64(c.DurationMS), Width: int64(c.Width), Height: int64(c.Height), HasAudio: flag(c.HasAudio)}); e != nil {
				return e
			}
		}
		return nil
	}()
}

func (s *Store) ReserveAnalysisCopy(ctx context.Context, p clip.AnalysisPreparation, slot string, bytes int64, digest, key string, expires, now time.Time) (clip.AnalysisCopy, error) {
	if bytes <= 0 || bytes > 8<<20 || !clip.ValidSHA256(digest) || !expires.After(now) || expires.After(p.ExpiresAt) {
		return clip.AnalysisCopy{}, clip.ErrInvalid
	}
	n, e := s.write.ReserveAnalysisCopy(ctx, sqlc.ReserveAnalysisCopyParams{ObjectKey: nullable(key), ExactBytes: bytes, Sha256: digest, PutExpiresAt: stamp(expires), PreparationID: p.ID, Slot: slot, Now: stamp(now)})
	if e != nil {
		return clip.AnalysisCopy{}, e
	}
	if n != 1 {
		return clip.AnalysisCopy{}, clip.ErrMediaConflict
	}
	updated, e := s.GetAnalysisPreparation(ctx, p.UserID, p.ID)
	if e != nil {
		return clip.AnalysisCopy{}, e
	}
	for _, c := range updated.Copies {
		if c.Slot == slot {
			return c, nil
		}
	}
	return clip.AnalysisCopy{}, clip.ErrNotFound
}
func (s *Store) BindAnalysisPreparation(ctx context.Context, p clip.AnalysisPreparation, parent string, now time.Time) error {
	return affected(s.write.BindAnalysisParent(ctx, sqlc.BindAnalysisParentParams{ParentJobID: nullable(parent), ID: p.ID, UserID: p.UserID, Now: stamp(now)}))
}
func (s *Store) SubmitAnalysisPreparation(ctx context.Context, p clip.AnalysisPreparation, digest string, wait, deadline, now time.Time) error {
	if len(p.Copies) == 0 {
		return affected(s.write.AcceptEmptyAnalysisPreparation(ctx, sqlc.AcceptEmptyAnalysisPreparationParams{ID: p.ID, UserID: p.UserID, Now: stamp(now)}))
	}
	return affected(s.write.SubmitAnalysisPreparation(ctx, sqlc.SubmitAnalysisPreparationParams{ManifestDigest: digest, DeadlineAt: stamp(deadline), QueueDeadlineAt: stamp(wait), ID: p.ID, UserID: p.UserID, Now: stamp(now)}))
}

func (s *Store) ClaimAnalysisVerification(ctx context.Context, profile clip.MediaWorkerProfile, l clip.AnalysisPreparationLimits, now time.Time) (*clip.MediaLease, error) {
	if profile.Operation != clip.MediaVerifyAnalysis || !profile.Compatible() || profile.Validate() != nil {
		return nil, clip.ErrMediaIncompatible
	}
	attempt, token := rand.Text(), rand.Text()
	row, e := s.write.ClaimAnalysisPreparation(ctx, sqlc.ClaimAnalysisPreparationParams{AttemptID: nullable(attempt), ProfileVersion: clip.BrowserAnalysisProfileVersion, Now: stamp(now), MaxAttempts: int64(l.Stages.MaxAttempts), ActiveLimit: int64(l.Capacity.Active)})
	if errors.Is(dbError(e), clip.ErrNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	p, e := analysisPreparationRow(ctx, s.read, row)
	if e != nil {
		return nil, e
	}
	if e = s.write.ExpireAnalysisVerificationAttempts(ctx, sqlc.ExpireAnalysisVerificationAttemptsParams{PreparationID: p.ID, Now: nullable(stamp(now))}); e != nil {
		return nil, e
	}
	end := minTime(now.Add(l.Stages.LeaseTTL), p.DeadlineAt)
	if e = s.write.InsertAnalysisVerificationAttempt(ctx, sqlc.InsertAnalysisVerificationAttemptParams{ID: attempt, PreparationID: p.ID, Ordinal: int64(p.Attempts), WorkerID: profile.WorkerID, TokenHash: mediaTokenHash(token), LeaseExpiresAt: stamp(end), StartedAt: stamp(now), RuntimeManifest: profile.RuntimeManifest}); e != nil {
		return nil, e
	}
	payload, e := json.Marshal(analysisVerificationTask(p))
	if e != nil {
		return nil, e
	}
	stage := clip.MediaStage{MediaStageInput: clip.MediaStageInput{ID: clip.AnalysisStageID(p.ID), ParentJobID: p.ParentJobID, UserID: p.UserID, ProjectID: p.ProjectID, ExpectedRevision: p.ExpectedRevision, Operation: clip.MediaVerifyAnalysis, ContractVersion: clip.MediaContractVersion, InputDigest: clip.MediaPayloadDigest(string(payload)), Payload: string(payload), RendererVersion: clip.AnalysisVerificationRenderer, AssetVersion: clip.AnalysisVerificationAssets, Limits: l.Stages}, State: clip.MediaRunning, CurrentAttemptID: attempt, AttemptCount: p.Attempts, CreatedAt: p.CreatedAt, DeadlineAt: p.DeadlineAt, QueueDeadlineAt: p.QueueDeadlineAt}
	return &clip.MediaLease{Stage: stage, Credentials: clip.MediaLeaseCredentials{StageID: stage.ID, AttemptID: attempt, WorkerID: profile.WorkerID, Token: token}, Profile: profile, ExpiresAt: end, HeartbeatAfter: min(clip.MediaHeartbeatHint, end.Sub(now)/4)}, nil
}
func analysisVerificationTask(p clip.AnalysisPreparation) clip.AnalysisVerificationTask {
	t := clip.AnalysisVerificationTask{Version: 1, ProfileVersion: p.ProfileVersion, ManifestDigest: p.ManifestDigest}
	for _, c := range p.Copies {
		c.ObjectKey, c.State, c.PutExpiresAt = "", "", time.Time{}
		c.Info = clip.MediaInfo{}
		t.Copies = append(t.Copies, c)
	}
	return t
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func (s *Store) AuthorizeAnalysisLease(ctx context.Context, auth clip.MediaLeaseCredentials, now time.Time, accepted bool) (clip.AnalysisPreparation, error) {
	id, ok := clip.AnalysisPreparationStage(auth.StageID)
	if !ok {
		return clip.AnalysisPreparation{}, clip.ErrMediaLeaseLost
	}
	row, e := s.read.AnalysisPreparationForStage(ctx, id)
	if e != nil {
		return clip.AnalysisPreparation{}, clip.ErrMediaLeaseLost
	}
	lease, e := s.read.GetAnalysisVerificationAttempt(ctx, auth.AttemptID)
	if e != nil {
		return clip.AnalysisPreparation{}, clip.ErrMediaLeaseLost
	}
	if row.CurrentAttemptID.String != auth.AttemptID || lease.PreparationID != id || lease.WorkerID != auth.WorkerID || subtle.ConstantTimeCompare([]byte(lease.TokenHash), []byte(mediaTokenHash(auth.Token))) != 1 {
		return clip.AnalysisPreparation{}, clip.ErrMediaLeaseLost
	}
	p, e := analysisPreparationRow(ctx, s.read, row)
	if e != nil {
		return p, e
	}
	if p.State == "cancelled" || p.State == "expired" {
		return p, clip.ErrMediaCancelled
	}
	if accepted && (p.State == "accepted" || p.State == "consumed") {
		return p, nil
	}
	end, e := time.Parse(time.RFC3339Nano, lease.LeaseExpiresAt)
	if e != nil {
		return p, e
	}
	if p.State != "verifying" || lease.Outcome.Valid || !end.After(now) || !p.ExpiresAt.After(now) || !p.DeadlineAt.After(now) {
		return p, clip.ErrMediaLeaseLost
	}
	return p, nil
}
func (s *Store) RenewAnalysisLease(ctx context.Context, auth clip.MediaLeaseCredentials, progress int, l clip.AnalysisPreparationLimits, now time.Time) (time.Time, error) {
	p, e := s.AuthorizeAnalysisLease(ctx, auth, now, false)
	if e != nil {
		return time.Time{}, e
	}
	if progress < 0 || progress > 1000 {
		return time.Time{}, clip.ErrInvalid
	}
	end := minTime(now.Add(l.Stages.LeaseTTL), p.DeadlineAt)
	if e = affected(s.write.RenewAnalysisVerification(ctx, sqlc.RenewAnalysisVerificationParams{LeaseExpiresAt: stamp(end), ID: auth.AttemptID, WorkerID: auth.WorkerID, TokenHash: mediaTokenHash(auth.Token), Now: stamp(now)})); e != nil {
		return time.Time{}, e
	}
	e = s.write.UpdateAnalysisProgress(ctx, sqlc.UpdateAnalysisProgressParams{Progress: int64(progress), ID: p.ID})
	return end, e
}
func (s *Store) AcceptAnalysisReceipt(ctx context.Context, auth clip.MediaLeaseCredentials, receipt string, result clip.AnalysisVerificationResult, now time.Time) error {
	p, e := s.AuthorizeAnalysisLease(ctx, auth, now, true)
	if e != nil {
		return e
	}
	if p.State == "accepted" || p.State == "consumed" {
		if p.Receipt != receipt {
			return clip.ErrMediaConflict
		}
		return nil
	}
	for i, v := range result.Copies {
		if i >= len(p.Copies) {
			return clip.ErrInvalidMedia
		}
		raw, e := json.Marshal(v)
		if e != nil {
			return e
		}
		if e = affected(s.write.VerifyAnalysisCopy(ctx, sqlc.VerifyAnalysisCopyParams{VerificationJson: nullable(string(raw)), PreparationID: p.ID, Slot: v.Slot, ExactBytes: v.Bytes, Sha256: v.Digest})); e != nil {
			return e
		}
	}
	if e = affected(s.write.AcceptAnalysisVerification(ctx, sqlc.AcceptAnalysisVerificationParams{AcceptedResult: nullable(receipt), ID: p.ID, AttemptID: nullable(auth.AttemptID), Now: stamp(now)})); e != nil {
		return e
	}
	return s.write.StopAnalysisVerificationAttempt(ctx, sqlc.StopAnalysisVerificationAttemptParams{Outcome: nullable("succeeded"), Now: nullable(stamp(now)), ID: auth.AttemptID})
}
func (s *Store) SetAnalysisPreparationState(ctx context.Context, id, state, failure string) error {
	return s.write.SetAnalysisPreparationState(ctx, sqlc.SetAnalysisPreparationStateParams{State: state, Failure: nullable(failure), ID: id})
}
func (s *Store) ConsumeAnalysisPreparation(ctx context.Context, id, parent string, now time.Time) error {
	return affected(s.write.ConsumeAnalysisPreparation(ctx, sqlc.ConsumeAnalysisPreparationParams{ID: id, ParentJobID: nullable(parent), Now: stamp(now)}))
}
func (s *Store) StopAnalysisAttempt(ctx context.Context, id, outcome string, now time.Time) error {
	return s.write.StopAnalysisVerificationAttempt(ctx, sqlc.StopAnalysisVerificationAttemptParams{Outcome: nullable(outcome), Now: nullable(stamp(now)), ID: id})
}
func (s *Store) AnalysisLeaseStopped(ctx context.Context, p clip.AnalysisPreparation, now time.Time) (bool, error) {
	if p.CurrentAttemptID == "" {
		return true, nil
	}
	r, e := s.read.GetAnalysisVerificationAttempt(ctx, p.CurrentAttemptID)
	if e != nil {
		return false, e
	}
	end, e := time.Parse(time.RFC3339Nano, r.LeaseExpiresAt)
	return r.Outcome.Valid || !end.After(now), e
}
func (s *Store) RetireAnalysisPreparation(ctx context.Context, id string, now time.Time) error {
	row, e := s.read.AnalysisPreparationForStage(ctx, id)
	if e != nil {
		return e
	}
	if row.CleanupAt.Valid {
		return nil
	}
	if e := s.write.QueueAnalysisCopyCleanup(ctx, sqlc.QueueAnalysisCopyCleanupParams{PreparationID: id, Now: stamp(now)}); e != nil {
		return e
	}
	if e := s.write.RetireAnalysisCopies(ctx, id); e != nil {
		return e
	}
	return s.write.MarkAnalysisPreparationCleanup(ctx, sqlc.MarkAnalysisPreparationCleanupParams{CleanupAt: nullable(stamp(now)), ID: id})
}
func (s *Store) AnalysisPreparationsForRecovery(ctx context.Context) ([]clip.AnalysisPreparation, error) {
	rows, e := s.read.LiveAnalysisPreparations(ctx)
	if e != nil {
		return nil, e
	}
	var out []clip.AnalysisPreparation
	for _, r := range rows {
		p, e := analysisPreparationRow(ctx, s.read, r)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *Store) DueAnalysisCopyCleanup(ctx context.Context, now time.Time) ([]clip.MediaDeletion, error) {
	rows, e := s.read.DueAnalysisCopyDeletions(ctx, stamp(now))
	if e != nil {
		return nil, e
	}
	var out []clip.MediaDeletion
	for _, r := range rows {
		out = append(out, clip.MediaDeletion{Key: r.ObjectKey})
	}
	return out, nil
}
func (s *Store) FinishAnalysisCopyCleanup(ctx context.Context, key string) error {
	_, e := transact(ctx, s, func(q *sqlc.Queries) (bool, error) {
		if e := q.MarkAnalysisCopyDeleted(ctx, nullable(key)); e != nil {
			return false, e
		}
		return true, q.RemoveAnalysisCopyDeletion(ctx, key)
	})
	return e
}
func (s *Store) QueueAnalysisOrphan(ctx context.Context, key string, now time.Time) error {
	_, e := transact(ctx, s, func(q *sqlc.Queries) (bool, error) {
		n, e := q.AnalysisObjectKeyExists(ctx, nullable(key))
		if e != nil || n != 0 {
			return false, e
		}
		return true, q.QueueAnalysisOrphanDeletion(ctx, sqlc.QueueAnalysisOrphanDeletionParams{ObjectKey: key, DeleteAfter: stamp(now)})
	})
	return e
}
func (s *Store) AnalysisVerificationStatus(ctx context.Context, worker string, maxAttempts int, now time.Time) (clip.MediaRuntimeStatus, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.MediaRuntimeStatus, error) {
		var out clip.MediaRuntimeStatus
		var err error
		out.Waiting, err = q.AnalysisVerificationWaitingCount(ctx, sqlc.AnalysisVerificationWaitingCountParams{Now: stamp(now), MaxAttempts: int64(maxAttempts)})
		if err != nil {
			return out, err
		}
		out.Active, err = q.AnalysisVerificationActiveCount(ctx, stamp(now))
		if err != nil {
			return out, err
		}
		out.OwnActive, err = q.AnalysisVerificationOwnActiveCount(ctx, sqlc.AnalysisVerificationOwnActiveCountParams{Now: stamp(now), WorkerID: worker})
		return out, err
	})
}

func (s *Store) MarkAnalysisPreparationReconciled(ctx context.Context, id string, now time.Time) error {
	return s.write.MarkAnalysisPreparationReconciled(ctx, sqlc.MarkAnalysisPreparationReconciledParams{ReconciledAt: nullable(stamp(now)), ID: id})
}
