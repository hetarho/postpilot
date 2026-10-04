package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice/spoken"
	"github.com/postpilot/backend/internal/voice/spoken/store/sqlc"
	"math/big"
	"time"
)

var _ spoken.OperationStorage = (*Store)(nil)

func (s *Store) GetOperationRequest(ctx context.Context, owner, kind, key string) (spoken.Operation, error) {
	r, err := s.read.GetOperationRequest(ctx, sqlc.GetOperationRequestParams{OwnerID: owner, Kind: kind, IdempotencyKey: key})
	if err != nil {
		return spoken.Operation{}, missing(err)
	}
	return operationFromRow(r)
}
func (s *Store) RecordOperationEvidence(ctx context.Context, owner, id string, index int, evidence llm.SpeechEvidence) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		if index < 0 || index > 1 {
			return spoken.ErrInvalid
		}
		o := e.Operation
		for len(o.Evidence) <= index {
			o.Evidence = append(o.Evidence, llm.SpeechEvidence{})
		}
		if previous := o.Evidence[index].RequestID; previous != "" && previous != evidence.RequestID {
			return spoken.ErrConflict
		}
		o.Evidence[index] = evidence
		return nil
	})
}

func (s *Store) GetOperation(ctx context.Context, owner, id string) (spoken.Operation, error) {
	r, err := s.read.GetOperation(ctx, sqlc.GetOperationParams{ID: id, OwnerID: owner})
	if err != nil {
		return spoken.Operation{}, missing(err)
	}
	return operationFromRow(r)
}
func (s *Store) ReserveOperation(ctx context.Context, o spoken.Operation) (out spoken.Operation, created bool, err error) {
	if o.ID == "" || o.OwnerID == "" || o.IdempotencyKey == "" || !spoken.IsJobKind(o.Kind) || o.RequestDigest == "" || o.ScopeDigest == "" {
		return out, false, spoken.ErrInvalid
	}
	err = s.transaction(ctx, func(tx *Store) error {
		r, e := tx.read.GetOperationRequest(ctx, sqlc.GetOperationRequestParams{OwnerID: o.OwnerID, Kind: o.Kind, IdempotencyKey: o.IdempotencyKey})
		if e == nil {
			out, e = operationFromRow(r)
			if e == nil && out.RequestDigest != o.RequestDigest {
				e = spoken.ErrConflict
			}
			return e
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if err := tx.operationInput(ctx, o); err != nil {
			return err
		}
		if o.Kind == spoken.JobKindConfirm {
			n, err := tx.read.UnresolvedConfirmation(ctx, sqlc.UnresolvedConfirmationParams{OwnerID: o.OwnerID, CandidateID: o.CandidateID})
			if err != nil {
				return err
			}
			if n > 0 {
				return spoken.ErrOperationUnresolved
			}
		}
		if o.QualificationSessionID != "" {
			limit, ok := new(big.Rat).SetString(o.QualificationLimitUSD)
			if !ok || limit.Sign() < 0 {
				return spoken.ErrQualificationBudget
			}
			total, ok := new(big.Rat).SetString(o.QualificationReservedUSD)
			if !ok || total.Sign() < 0 {
				return spoken.ErrQualificationBudget
			}
			previous, err := tx.ListQualificationOperations(ctx, o.OwnerID, o.QualificationSessionID)
			if err != nil {
				return err
			}
			for _, operation := range previous {
				priorLimit, ok := new(big.Rat).SetString(operation.QualificationLimitUSD)
				if !ok || priorLimit.Cmp(limit) != 0 {
					return spoken.ErrQualificationBudget
				}
				bound, ok := new(big.Rat).SetString(operation.QualificationReservedUSD)
				if !ok || bound.Sign() < 0 {
					return spoken.ErrQualificationBudget
				}
				total.Add(total, bound)
			}
			if total.Cmp(limit) > 0 {
				return spoken.ErrQualificationBudget
			}
		}
		o.State = spoken.OperationReserved
		o.CreatedAt = time.Now().UTC()
		o.UpdatedAt = o.CreatedAt
		data, err := encodeOperation(o)
		if err != nil {
			return err
		}
		if err := tx.write.InsertOperation(ctx, sqlc.InsertOperationParams{SampleAssetID: sql.NullString{String: o.SampleAssetID, Valid: o.SampleAssetID != ""}, ID: o.ID, OwnerID: o.OwnerID, Kind: o.Kind, State: o.State, JobID: o.JobID, IdempotencyKey: o.IdempotencyKey, RequestDigest: o.RequestDigest, ScopeDigest: o.ScopeDigest, CandidateID: o.CandidateID, ReceivedHandle: string(o.ReceivedHandle), SnapshotJson: data, CreatedAt: stamp(o.CreatedAt), UpdatedAt: stamp(o.UpdatedAt)}); err != nil {
			return err
		}
		out = o
		created = true
		return nil
	})
	return
}
func (s *Store) operationInput(ctx context.Context, o spoken.Operation) error {
	if o.Kind == spoken.JobKindProbe {
		v, err := s.GetVoice(ctx, o.OwnerID, o.VoiceID)
		if err != nil {
			return err
		}
		if v.RemovedAt != nil {
			return spoken.ErrNotFound
		}
		if v.Revision != o.ExpectedRevision || v.Profile != o.Profile || v.Handle != o.VoiceHandle {
			return spoken.ErrConflict
		}
		return nil
	}
	d, err := s.GetDraft(ctx, o.OwnerID, o.DraftID)
	if err != nil {
		return err
	}
	if d.ConfirmedVoiceID != "" {
		return spoken.ErrImmutable
	}
	if d.Revision != o.ExpectedRevision || d.Profile != o.Profile || d.Name != o.Name || d.Description != o.Description || d.PreviewText != o.PreviewText {
		return spoken.ErrConflict
	}
	if o.Kind == spoken.JobKindConfirm {
		if d.SelectedCandidateID != o.CandidateID {
			return spoken.ErrAuditionRequired
		}
		for _, c := range d.Candidates {
			if c.ID == o.CandidateID && c.Handle == o.CandidateHandle && c.AssetID == o.SampleAssetID && c.AuditionedAt != nil {
				return nil
			}
		}
		return spoken.ErrAuditionRequired
	}
	return nil
}
func (s *Store) updateOperation(ctx context.Context, o spoken.Operation, expected string) error {
	data, err := encodeOperation(o)
	if err != nil {
		return err
	}
	return changed(s.write.UpdateOperation(ctx, sqlc.UpdateOperationParams{NextState: o.State, Job: o.JobID, Handle: string(o.ReceivedHandle), Snapshot: data, At: stamp(time.Now()), ID: o.ID, Owner: o.OwnerID, Expected: expected}))
}
func (s *Store) changeOperation(ctx context.Context, owner, id string, f func(*OperationEdit) error) error {
	return s.transaction(ctx, func(tx *Store) error {
		o, err := tx.GetOperation(ctx, owner, id)
		if err != nil {
			return err
		}
		before := o.State
		if err := f(&OperationEdit{Operation: &o, store: tx, ctx: ctx}); err != nil {
			return err
		}
		return tx.updateOperation(ctx, o, before)
	})
}

type OperationEdit struct {
	Operation *spoken.Operation
	store     *Store
	ctx       context.Context
}

func (s *Store) BindOperation(ctx context.Context, owner, id, jobID string) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		o := e.Operation
		if jobID == "" || (o.JobID != "" && o.JobID != jobID) {
			return spoken.ErrConflict
		}
		o.JobID = jobID
		if o.State == spoken.OperationReserved {
			o.State = spoken.OperationQueued
		}
		return nil
	})
}
func (s *Store) ClaimOperation(ctx context.Context, owner, id, jobID string) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		o := e.Operation
		if (o.State != spoken.OperationReserved && o.State != spoken.OperationQueued) || (o.JobID != "" && o.JobID != jobID) || jobID == "" {
			return spoken.ErrOperationStopped
		}
		if err := e.store.operationInput(e.ctx, *o); err != nil {
			return err
		}
		o.JobID = jobID
		o.State = spoken.OperationClaimed
		if o.Kind == spoken.JobKindConfirm {
			o.Calling[0] = true
		}
		return nil
	})
}
func (s *Store) ClaimProbeCall(ctx context.Context, owner, id string, index int) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		o := e.Operation
		if index < 0 || index >= 2 || o.Kind != spoken.JobKindProbe || o.State != spoken.OperationClaimed || o.Calling[index] || o.AssetIDs[index] != "" {
			return spoken.ErrOperationStopped
		}
		if err := e.store.operationInput(e.ctx, *o); err != nil {
			return err
		}
		o.Calling[index] = true
		return nil
	})
}
func (s *Store) RecordConfirmationResult(ctx context.Context, owner, id string, handle llm.VoiceHandle, evidence llm.SpeechEvidence) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		o := e.Operation
		if o.Kind != spoken.JobKindConfirm || handle == "" || (o.ReceivedHandle != "" && o.ReceivedHandle != handle) {
			return spoken.ErrConflict
		}
		o.ReceivedHandle = handle
		o.Evidence = []llm.SpeechEvidence{evidence}
		if o.State == spoken.OperationClaimed {
			o.State = spoken.OperationReceived
		}
		return nil
	})
}
func (s *Store) PublishOperation(ctx context.Context, owner, id, result string) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		o := e.Operation
		if o.State == spoken.OperationPublished {
			if o.ResultID != result {
				return spoken.ErrConflict
			}
			return nil
		}
		if o.State != spoken.OperationClaimed && o.State != spoken.OperationReceived {
			return spoken.ErrOperationStopped
		}
		o.State = spoken.OperationPublished
		o.ResultID = result
		return nil
	})
}
func (s *Store) FailOperation(ctx context.Context, owner, id, reason string, uncertain bool) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		o := e.Operation
		if o.Terminal() {
			return nil
		}
		o.FailureReason = reason
		o.State = spoken.OperationFailed
		if uncertain && o.Kind == spoken.JobKindConfirm {
			o.State = spoken.OperationUnresolved
		}
		if o.ReceivedHandle != "" && !uncertain {
			o.State = spoken.OperationReceived
		}
		return nil
	})
}
func (s *Store) CancelOperation(ctx context.Context, owner, id string) error {
	return s.changeOperation(ctx, owner, id, func(e *OperationEdit) error {
		o := e.Operation
		if o.Terminal() {
			return nil
		}
		o.State = spoken.OperationCancelled
		return nil
	})
}
func (s *Store) ListRecoverableOperations(ctx context.Context) ([]spoken.Operation, error) {
	rs, err := s.read.RecoverableOperations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]spoken.Operation, 0, len(rs))
	for _, r := range rs {
		o, err := operationFromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}
func (s *Store) GetProbeAudio(ctx context.Context, owner, voice, input string) (spoken.ProbeAudio, error) {
	r, err := s.read.GetProbeAudio(ctx, sqlc.GetProbeAudioParams{OwnerID: owner, VoiceID: voice, InputDigest: input})
	if err != nil {
		return spoken.ProbeAudio{}, missing(err)
	}
	p, err := decodeProbe(r)
	if err != nil {
		return p, err
	}
	a, err := s.GetAsset(ctx, owner, p.AssetID)
	if err != nil {
		return p, err
	}
	if a.RevokedAt != nil {
		return p, spoken.ErrNotFound
	}
	return p, nil
}
func (s *Store) SaveProbeAudio(ctx context.Context, o spoken.Operation, index int, a spoken.Asset, p spoken.ProbeAudio) error {
	if index < 0 || index >= 2 || a.OwnerID != o.OwnerID || p.OwnerID != o.OwnerID || p.OperationID != o.ID || p.JobID != o.JobID || p.VoiceID != o.VoiceID || p.InputDigest != o.SpeechInputs[index] || p.AssetID != a.ID {
		return spoken.ErrInvalid
	}
	return s.transaction(ctx, func(tx *Store) error {
		current, err := tx.GetOperation(ctx, o.OwnerID, o.ID)
		if err != nil {
			return err
		}
		if current.State != spoken.OperationClaimed || !current.Calling[index] {
			return spoken.ErrOperationStopped
		}
		e, t, err := encodeProbe(p)
		if err != nil {
			return err
		}
		if err := tx.write.InsertAsset(ctx, sqlc.InsertAssetParams{ID: a.ID, OwnerID: a.OwnerID, ObjectKey: a.ObjectKey, Sha256: a.SHA256, Format: a.Format, Bytes: a.Bytes, Samples: a.Samples, SampleRate: int64(a.SampleRate), Channels: int64(a.Channels), ProvenanceDigest: a.ProvenanceDigest, CreatedAt: stamp(a.CreatedAt)}); err != nil {
			return err
		}
		if err := tx.write.InsertProbeAudio(ctx, sqlc.InsertProbeAudioParams{OriginOperationID: p.OperationID, OriginJobID: p.JobID, OwnerID: p.OwnerID, VoiceID: p.VoiceID, InputDigest: p.InputDigest, AssetID: p.AssetID, EvidenceJson: e, TimingJson: t}); err != nil {
			return err
		}
		current.AssetIDs[index] = a.ID
		for len(current.Evidence) <= index {
			current.Evidence = append(current.Evidence, llm.SpeechEvidence{})
		}
		current.Evidence[index] = p.Evidence
		return tx.updateOperation(ctx, current, current.State)
	})
}

func (s *Store) ListQualificationOperations(ctx context.Context, owner, session string) ([]spoken.Operation, error) {
	rows, err := s.read.QualificationOperations(ctx, sqlc.QualificationOperationsParams{Owner: owner, Session: session})
	if err != nil {
		return nil, err
	}
	out := make([]spoken.Operation, 0, len(rows))
	for _, row := range rows {
		o, err := operationFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}
