// Package store owns the spoken aggregate's SQLite mapping and writer boundary.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/voice/spoken"
	"github.com/postpilot/backend/internal/voice/spoken/store/sqlc"
)

const layout = "2006-01-02T15:04:05.000000000Z07:00"

func stamp(t time.Time) string { return t.UTC().Format(layout) }
func parse(s string) time.Time { t, _ := time.Parse(time.RFC3339Nano, s); return t }
func optional(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t := parse(s.String)
	return &t
}
func nullable(t time.Time) sql.NullString { return sql.NullString{String: stamp(t), Valid: true} }
func missing(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return spoken.ErrNotFound
	}
	return err
}
func changed(n int64, err error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return spoken.ErrConflict
	}
	return nil
}

type Store struct {
	writer      *sql.DB
	read, write *sqlc.Queries
}

var _ spoken.Storage = (*Store)(nil)

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, read: sqlc.New(reader), write: sqlc.New(writer)}
}
func NewTx(tx *sql.Tx) *Store { q := sqlc.New(tx); return &Store{read: q, write: q} }
func (s *Store) transaction(ctx context.Context, f func(*Store) error) error {
	if s.writer == nil {
		return f(s)
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := f(NewTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Mutate(ctx context.Context, key spoken.RequestIdentity, f func(spoken.Storage) (spoken.MutationResult, error)) (result spoken.MutationResult, err error) {
	if key.OwnerID == "" || key.Key == "" || key.Operation == "" || key.Digest == "" {
		return result, spoken.ErrInvalid
	}
	err = s.transaction(ctx, func(tx *Store) error {
		row, e := tx.read.GetRequest(ctx, sqlc.GetRequestParams{OwnerID: key.OwnerID, Operation: key.Operation, RequestKey: key.Key})
		if e == nil {
			if row.InputDigest != key.Digest {
				return spoken.ErrConflict
			}
			result = spoken.MutationResult{ID: row.ResultID, Revision: row.ResultRevision}
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		result, e = f(tx)
		if e != nil {
			return e
		}
		return tx.write.SaveRequest(ctx, sqlc.SaveRequestParams{OwnerID: key.OwnerID, Operation: key.Operation, RequestKey: key.Key, InputDigest: key.Digest, ResultID: result.ID, ResultRevision: result.Revision})
	})
	return
}
func (s *Store) ListDrafts(ctx context.Context, owner string) ([]spoken.Draft, error) {
	rows, err := s.read.ListDrafts(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]spoken.Draft, 0, len(rows))
	for _, row := range rows {
		d, err := s.draft(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
func (s *Store) GetDraft(ctx context.Context, owner, id string) (spoken.Draft, error) {
	row, err := s.read.GetDraft(ctx, sqlc.GetDraftParams{ID: id, OwnerID: owner})
	if err != nil {
		return spoken.Draft{}, missing(err)
	}
	return s.draft(ctx, row)
}
func (s *Store) InsertDraft(ctx context.Context, d spoken.Draft) error {
	p, err := encodeProfile(d.Profile)
	if err != nil {
		return err
	}
	return s.write.InsertDraft(ctx, sqlc.InsertDraftParams{ID: d.ID, OwnerID: d.OwnerID, Revision: d.Revision, Name: d.Name, Description: d.Description, PreviewText: d.PreviewText, ProfileJson: p, QualificationSessionID: d.QualificationSessionID, GenerationID: d.GenerationID, SelectedCandidateID: d.SelectedCandidateID, ConfirmedVoiceID: d.ConfirmedVoiceID, CreatedAt: stamp(d.CreatedAt), UpdatedAt: stamp(d.UpdatedAt)})
}
func (s *Store) UpdateDraft(ctx context.Context, d spoken.Draft, expected int64) error {
	before, err := s.GetDraft(ctx, d.OwnerID, d.ID)
	if err != nil {
		return err
	}
	if before.ConfirmedVoiceID != "" {
		return spoken.ErrImmutable
	}
	p, err := encodeProfile(d.Profile)
	if err != nil {
		return err
	}
	if err := changed(s.write.UpdateDraft(ctx, sqlc.UpdateDraftParams{Name: d.Name, Description: d.Description, PreviewText: d.PreviewText, Profile: p, Qualification: d.QualificationSessionID, Generation: d.GenerationID, Selected: d.SelectedCandidateID, At: stamp(d.UpdatedAt), ID: d.ID, Owner: d.OwnerID, Expected: expected})); err != nil {
		return err
	}
	if before.GenerationID != d.GenerationID {
		return s.write.DeleteCandidates(ctx, sqlc.DeleteCandidatesParams{DraftID: d.ID, OwnerID: d.OwnerID})
	}
	return nil
}
func (s *Store) DeleteDraft(ctx context.Context, owner, id string, rev int64) error {
	if _, err := s.GetDraft(ctx, owner, id); err != nil {
		return err
	}
	return changed(s.write.DeleteDraft(ctx, sqlc.DeleteDraftParams{ID: id, OwnerID: owner, Revision: rev}))
}
func (s *Store) ListVoices(ctx context.Context, owner string, removed bool) ([]spoken.Voice, error) {
	include := int64(0)
	if removed {
		include = 1
	}
	rows, err := s.read.ListVoices(ctx, sqlc.ListVoicesParams{OwnerID: owner, IncludeRemoved: include})
	if err != nil {
		return nil, err
	}
	out := make([]spoken.Voice, 0, len(rows))
	for _, r := range rows {
		v, err := voice(sqlc.GetVoiceRow(r))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) GetVoice(ctx context.Context, owner, id string) (spoken.Voice, error) {
	r, err := s.read.GetVoice(ctx, sqlc.GetVoiceParams{ID: id, OwnerID: owner})
	if err != nil {
		return spoken.Voice{}, missing(err)
	}
	return voice(r)
}
func (s *Store) RenameVoice(ctx context.Context, owner, id string, rev int64, name string) error {
	if _, err := s.GetVoice(ctx, owner, id); err != nil {
		return err
	}
	return changed(s.write.RenameVoice(ctx, sqlc.RenameVoiceParams{Name: name, ID: id, OwnerID: owner, Revision: rev}))
}
func (s *Store) RemoveVoice(ctx context.Context, owner, id string, rev int64, at time.Time) error {
	if _, err := s.GetVoice(ctx, owner, id); err != nil {
		return err
	}
	return changed(s.write.RemoveVoice(ctx, sqlc.RemoveVoiceParams{RemovedAt: nullable(at), ID: id, OwnerID: owner, Revision: rev}))
}
func (s *Store) mutableDraft(ctx context.Context, owner, id string, rev int64) (spoken.Draft, error) {
	d, err := s.GetDraft(ctx, owner, id)
	if err != nil {
		return d, err
	}
	if d.ConfirmedVoiceID != "" {
		return d, spoken.ErrImmutable
	}
	if d.Revision != rev {
		return d, spoken.ErrConflict
	}
	return d, nil
}
func (s *Store) SelectCandidate(ctx context.Context, owner, id string, rev int64, candidate string) error {
	if _, err := s.mutableDraft(ctx, owner, id, rev); err != nil {
		return err
	}
	return changed(s.write.SelectCandidate(ctx, sqlc.SelectCandidateParams{Candidate: candidate, At: stamp(time.Now()), Draft: id, Owner: owner, Expected: rev}))
}
func (s *Store) AcknowledgeCandidate(ctx context.Context, owner, id string, rev int64, candidate, ticket string, at time.Time) error {
	d, err := s.mutableDraft(ctx, owner, id, rev)
	if err != nil {
		return err
	}
	found := false
	for _, c := range d.Candidates {
		if c.ID == candidate {
			found = true
		}
	}
	if !found {
		return spoken.ErrNotFound
	}
	if err := changed(s.write.MarkCandidateAuditioned(ctx, sqlc.MarkCandidateAuditionedParams{At: nullable(at), Candidate: candidate, Owner: owner, Draft: id, Playback: ticket})); err != nil {
		return spoken.ErrAuditionRequired
	}
	return changed(s.write.AdvanceDraftRevision(ctx, sqlc.AdvanceDraftRevisionParams{At: stamp(at), ID: id, Owner: owner, Expected: rev}))
}
func (s *Store) AttachCandidates(ctx context.Context, owner, id string, rev int64, generation string, candidates []spoken.Candidate, assets []spoken.Asset) error {
	if _, err := s.mutableDraft(ctx, owner, id, rev); err != nil {
		return err
	}
	if len(candidates) != 3 || len(assets) != 3 {
		return spoken.ErrInvalid
	}
	if err := s.write.DeleteCandidates(ctx, sqlc.DeleteCandidatesParams{DraftID: id, OwnerID: owner}); err != nil {
		return err
	}
	for i, c := range candidates {
		a := assets[i]
		if c.OwnerID != owner || c.DraftID != id || c.GenerationID != generation || c.AssetID != a.ID || a.OwnerID != owner {
			return spoken.ErrInvalid
		}
		if err := s.write.InsertAsset(ctx, sqlc.InsertAssetParams{ID: a.ID, OwnerID: a.OwnerID, ObjectKey: a.ObjectKey, Sha256: a.SHA256, Format: a.Format, Bytes: a.Bytes, Samples: a.Samples, SampleRate: int64(a.SampleRate), Channels: int64(a.Channels), ProvenanceDigest: a.ProvenanceDigest, CreatedAt: stamp(a.CreatedAt)}); err != nil {
			return err
		}
		if err := s.write.InsertCandidate(ctx, sqlc.InsertCandidateParams{ID: c.ID, OwnerID: owner, DraftID: id, GenerationID: generation, SupplierHandle: string(c.Handle), Ordinal: int64(c.Ordinal), AssetID: c.AssetID}); err != nil {
			return err
		}
	}
	return changed(s.write.SetCandidatesReady(ctx, sqlc.SetCandidatesReadyParams{Generation: generation, At: stamp(time.Now()), ID: id, Owner: owner, Expected: rev}))
}
func (s *Store) ConfirmVoice(ctx context.Context, owner, id string, rev int64, candidate string, v spoken.Voice) error {
	d, err := s.mutableDraft(ctx, owner, id, rev)
	if err != nil {
		return err
	}
	if d.SelectedCandidateID != candidate || v.OwnerID != owner {
		return spoken.ErrAuditionRequired
	}
	found := false
	for _, c := range d.Candidates {
		if c.ID == candidate && c.AuditionedAt != nil && c.AssetID == v.SampleAssetID {
			found = true
		}
	}
	if !found {
		return spoken.ErrAuditionRequired
	}
	p, err := encodeProfile(v.Profile)
	if err != nil {
		return err
	}
	original, err := encodeProfile(d.Profile)
	if err != nil {
		return err
	}
	if p != original || v.Name != d.Name || v.Description != d.Description || v.PreviewText != d.PreviewText {
		return spoken.ErrImmutable
	}
	if err := s.write.InsertVoice(ctx, sqlc.InsertVoiceParams{ID: v.ID, OwnerID: owner, Revision: 1, Name: v.Name, Description: v.Description, PreviewText: v.PreviewText, ProfileJson: p, SupplierHandle: string(v.Handle), DraftID: sql.NullString{String: id, Valid: true}, CandidateID: sql.NullString{String: candidate, Valid: true}, SampleAssetID: v.SampleAssetID, CreatedAt: stamp(v.CreatedAt)}); err != nil {
		return err
	}
	return changed(s.write.SetConfirmed(ctx, sqlc.SetConfirmedParams{Voice: v.ID, At: stamp(v.CreatedAt), Draft: id, Owner: owner, Expected: rev, Candidate: candidate}))
}
func (s *Store) AcquireVoice(ctx context.Context, owner, id string, rev int64, ref string) (v spoken.Voice, err error) {
	err = s.transaction(ctx, func(tx *Store) error {
		existing, e := tx.read.GetVoiceUse(ctx, ref)
		if e == nil {
			if existing.OwnerID != owner {
				return spoken.ErrNotFound
			}
			if existing.VoiceID != id || existing.VoiceRevision != rev {
				return spoken.ErrConflict
			}
			v, e = tx.GetVoice(ctx, owner, id)
			return e
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		v, e = tx.GetVoice(ctx, owner, id)
		if e != nil {
			return e
		}
		if v.RemovedAt != nil {
			return spoken.ErrNotFound
		}
		if v.Revision != rev {
			return spoken.ErrConflict
		}
		return tx.write.RecordVoiceUse(ctx, sqlc.RecordVoiceUseParams{ID: ref, OwnerID: owner, VoiceID: id, VoiceRevision: rev, CreatedAt: stamp(time.Now())})
	})
	return
}

func (s *Store) GetAsset(ctx context.Context, owner, id string) (spoken.Asset, error) {
	row, err := s.read.GetAsset(ctx, sqlc.GetAssetParams{ID: id, OwnerID: owner})
	if err != nil {
		return spoken.Asset{}, missing(err)
	}
	return asset(row), nil
}
func (s *Store) SavePlayback(ctx context.Context, p spoken.Playback) error {
	return s.transaction(ctx, func(tx *Store) error {
		a, err := tx.GetAsset(ctx, p.OwnerID, p.AssetID)
		if err != nil {
			return err
		}
		if a.RevokedAt != nil {
			return spoken.ErrNotFound
		}
		if err := tx.write.PurgePlayback(ctx, stamp(time.Now())); err != nil {
			return err
		}
		return tx.write.SavePlayback(ctx, sqlc.SavePlaybackParams{ID: p.ID, OwnerID: p.OwnerID, AssetID: p.AssetID, ExpiresAt: stamp(p.ExpiresAt)})
	})
}
func (s *Store) GetPlayback(ctx context.Context, owner, id string, now time.Time) (spoken.Playback, spoken.Asset, error) {
	r, err := s.read.GetPlayback(ctx, sqlc.GetPlaybackParams{ID: id, OwnerID: owner, ExpiresAt: stamp(now)})
	if err != nil {
		return spoken.Playback{}, spoken.Asset{}, missing(err)
	}
	a, err := s.GetAsset(ctx, owner, r.AssetID)
	if err != nil {
		return spoken.Playback{}, spoken.Asset{}, err
	}
	if a.RevokedAt != nil {
		return spoken.Playback{}, spoken.Asset{}, spoken.ErrNotFound
	}
	return spoken.Playback{ID: r.ID, OwnerID: r.OwnerID, AssetID: r.AssetID, ExpiresAt: parse(r.ExpiresAt), ServedAt: optional(r.ServedAt)}, a, nil
}
func (s *Store) MarkPlaybackServed(ctx context.Context, owner, id string, now time.Time) error {
	return s.transaction(ctx, func(tx *Store) error {
		if _, _, err := tx.GetPlayback(ctx, owner, id, now); err != nil {
			return err
		}
		return changed(tx.write.MarkPlaybackServed(ctx, sqlc.MarkPlaybackServedParams{ServedAt: nullable(now), ID: id, OwnerID: owner, ExpiresAt: stamp(now)}))
	})
}
func (s *Store) RevokeAsset(ctx context.Context, owner, id string, now time.Time) error {
	if _, err := s.GetAsset(ctx, owner, id); err != nil {
		return err
	}
	return changed(s.write.RevokeAsset(ctx, sqlc.RevokeAssetParams{RevokedAt: nullable(now), ID: id, OwnerID: owner}))
}
func (s *Store) PrepareCleanup(ctx context.Context, c spoken.Cleanup) error {
	return s.write.PrepareCleanup(ctx, sqlc.PrepareCleanupParams{ID: c.ID, ObjectKey: c.ObjectKey, CreatedAt: stamp(c.CreatedAt)})
}
func (s *Store) PendingCleanup(ctx context.Context, before time.Time) ([]spoken.Cleanup, error) {
	rows, err := s.read.PendingCleanup(ctx, stamp(before))
	if err != nil {
		return nil, err
	}
	out := make([]spoken.Cleanup, 0, len(rows))
	for _, r := range rows {
		out = append(out, spoken.Cleanup{ID: r.ID, ObjectKey: r.ObjectKey, CreatedAt: parse(r.CreatedAt)})
	}
	return out, nil
}
func (s *Store) CompleteCleanup(ctx context.Context, id string) error {
	return s.write.CompleteCleanup(ctx, id)
}
func (s *Store) DiscardRetainedCleanup(ctx context.Context, id string) error {
	return s.write.DiscardRetainedCleanup(ctx, id)
}
func (s *Store) AssetRetained(ctx context.Context, id string) (bool, error) {
	n, err := s.read.AssetRetained(ctx, id)
	return n > 0, err
}
func (s *Store) DeleteOwner(ctx context.Context, owner string) error {
	return s.transaction(ctx, func(tx *Store) error {
		if err := tx.write.DeleteOwnerVoices(ctx, owner); err != nil {
			return err
		}
		if err := tx.write.DeleteOwnerDrafts(ctx, owner); err != nil {
			return err
		}
		return tx.write.DeleteOwnerAssets(ctx, owner)
	})
}
