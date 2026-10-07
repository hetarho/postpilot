package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store/sqlc"
)

func materialFingerprint(in voice.SampleMutation) string {
	// Optional-field presence is part of the request. Derived validated content
	// must not turn a lost-response retry into a different operation.
	raw, _ := json.Marshal(struct {
		VoiceID, SampleID          string
		Revision                   int64
		Label, Body, PhotoUploadID *string
		PhotoWidth, PhotoHeight    *int
	}{in.VoiceID, in.SampleID, in.ExpectedContentRevision, in.Label, in.Body, in.PhotoUploadID, in.PhotoWidth, in.PhotoHeight})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func sampleMutationReceipt(ctx context.Context, q *sqlc.Queries, in voice.SampleMutation) (*voice.Sample, error) {
	row, err := q.GetVoiceSampleMutation(ctx, sqlc.GetVoiceSampleMutationParams{UserID: in.UserID, OperationKey: in.OperationKey})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.VoiceID != in.VoiceID || row.SampleID != in.SampleID || row.Fingerprint != materialFingerprint(in) {
		return nil, voice.ErrSampleRevisionConflict
	}
	var sample voice.Sample
	if err := json.Unmarshal([]byte(row.Response), &sample); err != nil {
		return nil, err
	}
	if sample.UserID != in.UserID || sample.VoiceID != in.VoiceID || sample.ID != in.SampleID {
		return nil, voice.ErrSampleRevisionConflict
	}
	return &sample, nil
}

func (s *Store) VoiceSampleMutationReceipt(ctx context.Context, in voice.SampleMutation) (*voice.Sample, error) {
	return sampleMutationReceipt(ctx, s.read, in)
}

func (s *Store) UpdateVoiceSample(ctx context.Context, in voice.SampleMutation) (voice.Sample, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return voice.Sample{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	if prior, err := sampleMutationReceipt(ctx, q, in); err != nil {
		return voice.Sample{}, err
	} else if prior != nil {
		return *prior, tx.Commit()
	}
	owner, err := q.GetVoice(ctx, sqlc.GetVoiceParams{ID: in.VoiceID, UserID: in.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.Sample{}, voice.ErrVoiceNotFound
	}
	if err != nil {
		return voice.Sample{}, err
	}
	if owner.DeletedAt.Valid {
		return voice.Sample{}, voice.ErrVoiceDeleted
	}
	row, err := q.GetSampleBody(ctx, sqlc.GetSampleBodyParams{ID: in.SampleID, VoiceID: in.VoiceID, UserID: in.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		return voice.Sample{}, voice.ErrSampleNotFound
	}
	if err != nil {
		return voice.Sample{}, err
	}
	current, err := toSample(in.UserID, in.VoiceID, row)
	if err != nil {
		return voice.Sample{}, err
	}
	if current.ContentRevision != in.ExpectedContentRevision {
		return voice.Sample{}, voice.ErrSampleRevisionConflict
	}
	if in.Validated == nil {
		return voice.Sample{}, voice.ErrSampleUpdateInvalid
	}
	updated := *in.Validated
	if updated.ID != current.ID || updated.UserID != current.UserID || updated.VoiceID != current.VoiceID || updated.Kind != current.Kind || updated.PromptKey != current.PromptKey {
		return voice.Sample{}, voice.ErrSampleUpdateInvalid
	}
	updated.CreatedAt, updated.ContentRevision = current.CreatedAt, current.ContentRevision
	changed := updated.Body != current.Body || updated.PhotoKey != current.PhotoKey || updated.PhotoWidth != current.PhotoWidth || updated.PhotoHeight != current.PhotoHeight
	if changed {
		if current.ContentRevision == math.MaxInt64 {
			return voice.Sample{}, voice.ErrSampleRevisionConflict
		}
		updated.ContentRevision++
	}
	if in.ValidatedUploadID != "" {
		upload, err := q.GetPhotoUpload(ctx, sqlc.GetPhotoUploadParams{ID: in.ValidatedUploadID, VoiceID: in.VoiceID, UserID: in.UserID})
		if errors.Is(err, sql.ErrNoRows) {
			return voice.Sample{}, voice.ErrPhotoRequired
		}
		if err != nil {
			return voice.Sample{}, err
		}
		expires, err := parseTime(upload.ExpiresAt)
		if err != nil || !expires.After(time.Now()) || upload.PromptKey != current.PromptKey || upload.ObjectKey != updated.PhotoKey {
			return voice.Sample{}, voice.ErrPhotoRequired
		}
		if err := q.DeletePhotoUpload(ctx, upload.ID); err != nil {
			return voice.Sample{}, err
		}
	}
	photoWidth, photoHeight := sql.NullInt64{}, sql.NullInt64{}
	if updated.HasPhoto() {
		photoWidth, photoHeight = sql.NullInt64{Int64: int64(updated.PhotoWidth), Valid: true}, sql.NullInt64{Int64: int64(updated.PhotoHeight), Valid: true}
	}
	n, err := q.UpdateOwnedVoiceSample(ctx, sqlc.UpdateOwnedVoiceSampleParams{Label: updated.Label, Body: updated.Body, PhotoKey: nullableString(updated.PhotoKey), PhotoWidth: photoWidth, PhotoHeight: photoHeight, ContentRevision: updated.ContentRevision, UserID: in.UserID, VoiceID: in.VoiceID, ID: in.SampleID, ContentRevision_2: in.ExpectedContentRevision})
	if err != nil {
		return voice.Sample{}, err
	}
	if n != 1 {
		return voice.Sample{}, voice.ErrSampleRevisionConflict
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		return voice.Sample{}, err
	}
	err = q.InsertVoiceSampleMutation(ctx, sqlc.InsertVoiceSampleMutationParams{UserID: in.UserID, VoiceID: in.VoiceID, SampleID: in.SampleID, OperationKey: in.OperationKey, ExpectedContentRevision: in.ExpectedContentRevision, ResultingContentRevision: updated.ContentRevision, Fingerprint: materialFingerprint(in), Response: string(encoded), CreatedAt: formatTime(time.Now())})
	if err != nil {
		return voice.Sample{}, err
	}
	return updated, tx.Commit()
}
