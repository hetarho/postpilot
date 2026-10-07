package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/authoring/store/sqlc"
)

const stampLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Store struct {
	writer      *sql.DB
	read, write *sqlc.Queries
	now         func() time.Time
}

func New(writer, reader *sql.DB) *Store {
	return &Store{writer: writer, read: sqlc.New(reader), write: sqlc.New(writer), now: time.Now}
}
func missing(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return authoring.ErrNotFound
	}
	return err
}
func fromSession(r sqlc.ConfigurationAuthoringSession) (authoring.Session, error) {
	s, e := decodeSession(r.Snapshot)
	if e != nil {
		return s, e
	}
	if r.Revision < 0 || r.Revision > math.MaxUint32 {
		return s, authoring.ErrInvalid
	}
	s.ID, s.UserID, s.Kind, s.TargetID, s.Phase = r.ID, r.UserID, authoring.Kind(r.Kind), r.TargetID, r.Phase
	s.Revision = uint32(r.Revision)
	s.RequestedCandidateCount = int(r.CandidateCount)
	s.DraftState = authoring.DraftState(r.DraftState)
	s.HasUnpublishedChanges, s.SavedAvailable = r.HasUnpublishedChanges != 0, r.SavedAvailable != 0
	if r.WorkingSource.Valid {
		a, err := decodeArtifact(r.WorkingSource)
		if err != nil {
			return s, err
		}
		s.WorkingSource = a
	} else if s.Selected != nil {
		a := *s.Selected
		s.WorkingSource = &a
	}
	if r.SavedBaseline.Valid {
		a, err := decodeArtifact(r.SavedBaseline)
		if err != nil {
			return s, err
		}
		s.SavedBaseline = a
	}
	// Older rows did not capture a baseline. Only they need the compatibility
	// projection; an explicit missing-target verdict must stay unavailable.
	if s.SavedBaseline == nil && (s.TargetID != "" || s.Saved != nil) && s.FailureReason != "AUTHORING_SAVE_CONFLICT" {
		s.SavedAvailable = true
	}
	if s.WorkingSource != nil && s.SavedBaseline == nil && s.Phase != "saved" {
		s.HasUnpublishedChanges = true
	}
	s.CreatedAt, e = time.Parse(time.RFC3339Nano, r.CreatedAt)
	if e != nil {
		return s, e
	}
	s.UpdatedAt, e = time.Parse(time.RFC3339Nano, r.UpdatedAt)
	return s, e
}
func fromOperation(r sqlc.ConfigurationAuthoringOperation) (authoring.Operation, error) {
	t, e := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if e != nil {
		return authoring.Operation{}, e
	}
	if r.BaseRevision < 0 || r.BaseRevision > math.MaxUint32 {
		return authoring.Operation{}, authoring.ErrInvalid
	}
	return authoring.Operation{ID: r.ID, UserID: r.UserID, SessionID: r.SessionID, RequestID: r.RequestID, Fingerprint: r.Fingerprint, JobID: r.JobID, Status: r.Status, FailureReason: r.FailureReason, Mode: authoring.Mode(r.Mode), BaseRevision: uint32(r.BaseRevision), Payload: append([]byte(nil), r.Payload...), CreatedAt: t}, nil
}
func load(ctx context.Context, q *sqlc.Queries, user, id string) (authoring.Session, error) {
	r, e := q.GetAuthoringSession(ctx, sqlc.GetAuthoringSessionParams{UserID: user, ID: id})
	if e != nil {
		return authoring.Session{}, missing(e)
	}
	return fromSession(r)
}
func (s *Store) writeSession(ctx context.Context, q *sqlc.Queries, state authoring.Session, old uint32) error {
	if state.Revision < old {
		return authoring.ErrStale
	}
	state.UpdatedAt = s.now().UTC()
	raw, e := encodeSession(state)
	if e != nil {
		return e
	}
	n, e := q.UpdateAuthoringSession(ctx, sqlc.UpdateAuthoringSessionParams{Revision: int64(state.Revision), Phase: state.Phase, Snapshot: raw, SavedBaseline: encodeArtifact(state.SavedBaseline), WorkingSource: encodeArtifact(state.WorkingSource), DraftState: string(state.DraftState), HasUnpublishedChanges: bit(state.HasUnpublishedChanges), SavedAvailable: bit(state.SavedAvailable), PublicationPending: bit(state.Phase == "saving"), TargetConflict: bit(state.FailureReason == "AUTHORING_SAVE_CONFLICT"), DisplayName: displayName(state), CandidateCount: int64(candidateCount(state.RequestedCandidateCount)), UpdatedAt: state.UpdatedAt.Format(stampLayout), UserID: state.UserID, ID: state.ID, Revision_2: int64(old)})
	if e == nil && n != 1 {
		return authoring.ErrStale
	}
	return e
}
func bump(s *authoring.Session) error {
	if s.Revision == math.MaxUint32 {
		return authoring.ErrStale
	}
	s.Revision++
	return nil
}
func active(s authoring.Session) bool { return s.ActiveRequestID != "" || s.Phase == "saving" }
func (s *Store) Get(ctx context.Context, user, id string) (authoring.Session, error) {
	return load(ctx, s.read, user, id)
}
func (s *Store) Created(ctx context.Context, user, requestID string) (*authoring.Session, error) {
	r, e := s.read.GetAuthoringSessionByRequest(ctx, sqlc.GetAuthoringSessionByRequestParams{UserID: user, RequestID: requestID})
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	state, e := fromSession(r)
	return &state, e
}
func (s *Store) Create(ctx context.Context, state authoring.Session, requestID string) (authoring.Session, error) {
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return state, e
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	prior, e := q.GetAuthoringSessionByRequest(ctx, sqlc.GetAuthoringSessionByRequestParams{UserID: state.UserID, RequestID: requestID})
	if e == nil {
		p, e := fromSession(prior)
		if e != nil {
			return p, e
		}
		if p.Kind != state.Kind || p.TargetID != state.TargetID {
			return p, authoring.ErrStale
		}
		return p, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return state, e
	}
	raw, e := encodeSession(state)
	if e != nil {
		return state, e
	}
	e = q.InsertAuthoringSession(ctx, sqlc.InsertAuthoringSessionParams{ID: state.ID, UserID: state.UserID, Kind: string(state.Kind), TargetID: state.TargetID, RequestID: requestID, Revision: int64(state.Revision), Phase: state.Phase, Snapshot: raw, SavedBaseline: encodeArtifact(state.SavedBaseline), WorkingSource: encodeArtifact(state.WorkingSource), DraftState: string(state.DraftState), HasUnpublishedChanges: bit(state.HasUnpublishedChanges), SavedAvailable: bit(state.SavedAvailable), DisplayName: displayName(state), CandidateCount: int64(candidateCount(state.RequestedCandidateCount)), CreatedAt: state.CreatedAt.UTC().Format(stampLayout), UpdatedAt: state.UpdatedAt.UTC().Format(stampLayout)})
	if e != nil {
		return state, e
	}
	return state, tx.Commit()
}
func (s *Store) Latest(ctx context.Context, user string, kind authoring.Kind, target string) (*authoring.Session, error) {
	r, e := s.read.LatestAuthoringSession(ctx, sqlc.LatestAuthoringSessionParams{UserID: user, Kind: string(kind), TargetID: target})
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	state, e := fromSession(r)
	return &state, e
}
func (s *Store) Operation(ctx context.Context, user, session, req string) (authoring.Operation, error) {
	r, e := s.read.GetAuthoringOperation(ctx, sqlc.GetAuthoringOperationParams{UserID: user, SessionID: session, RequestID: req})
	if e != nil {
		return authoring.Operation{}, missing(e)
	}
	return fromOperation(r)
}
func (s *Store) ActiveOperation(ctx context.Context, user, session string) (authoring.Operation, error) {
	r, e := s.read.ActiveAuthoringOperation(ctx, sqlc.ActiveAuthoringOperationParams{UserID: user, SessionID: session})
	if e != nil {
		return authoring.Operation{}, missing(e)
	}
	return fromOperation(r)
}
func (s *Store) Reserve(ctx context.Context, user, id string, revision uint32, op authoring.Operation) (authoring.Session, authoring.Operation, bool, error) {
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return authoring.Session{}, op, false, e
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	state, e := load(ctx, q, user, id)
	if e != nil {
		return state, op, false, e
	}
	prior, e := q.GetAuthoringOperation(ctx, sqlc.GetAuthoringOperationParams{UserID: user, SessionID: id, RequestID: op.RequestID})
	if e == nil {
		p, e := fromOperation(prior)
		if e != nil {
			return state, p, false, e
		}
		if p.Fingerprint != op.Fingerprint {
			return state, p, false, authoring.ErrStale
		}
		return state, p, false, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return state, op, false, e
	}
	if state.Revision != revision {
		return state, op, false, authoring.ErrStale
	}
	if active(state) || state.Phase == "saved" {
		return state, op, false, authoring.ErrBusy
	}
	if op.Mode == authoring.Refine {
		if state.WorkingSource == nil && state.Selected == nil {
			return state, op, false, authoring.ErrNoSelection
		}
		done := 0
		for _, t := range state.Turns {
			if t.Status == "done" {
				done++
			}
		}
		if done >= authoring.MaxTurns {
			return state, op, false, authoring.ErrHistoryFull
		}
	}
	e = q.InsertAuthoringOperation(ctx, sqlc.InsertAuthoringOperationParams{ID: op.ID, UserID: user, SessionID: id, RequestID: op.RequestID, Fingerprint: op.Fingerprint, BaseRevision: int64(revision), Mode: string(op.Mode), Payload: op.Payload, JobID: "", Status: "pending", FailureReason: "", CreatedAt: op.CreatedAt.UTC().Format(stampLayout)})
	if e != nil {
		if strings.Contains(e.Error(), "UNIQUE constraint failed: configuration_authoring_operations.user_id") {
			e = authoring.ErrBusy
		}
		return state, op, false, e
	}
	if e = bump(&state); e != nil {
		return state, op, false, e
	}
	state.ActiveRequestID = op.ID
	state.ActiveJobID = ""
	state.FailureReason = ""
	state.PendingRequest = operationPrompt(op.Payload)
	state.Phase = "generating"
	if op.Mode == authoring.Refine {
		state.Phase = "refining"
		kept := []authoring.Turn{}
		for _, t := range state.Turns {
			if t.Status == "done" {
				kept = append(kept, t)
			}
		}
		state.Turns = append(kept, authoring.Turn{ID: op.ID, Request: state.PendingRequest, Status: "pending"})
	}
	if e = s.writeSession(ctx, q, state, revision); e != nil {
		return state, op, false, e
	}
	op.UserID, op.SessionID, op.BaseRevision, op.Status = user, id, revision, "pending"
	return state, op, true, tx.Commit()
}

// Only the latest request is decoded here; model input remains an opaque frozen receipt.
func operationPrompt(payload []byte) string {
	var p struct {
		Prompt string `json:"prompt"`
	}
	_ = json.Unmarshal(payload, &p)
	return p.Prompt
}
func (s *Store) operationMutation(ctx context.Context, user, opID string, apply func(*authoring.Session, *authoring.Operation) error) (authoring.Session, error) {
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return authoring.Session{}, e
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	row, e := q.GetAuthoringOperationByID(ctx, sqlc.GetAuthoringOperationByIDParams{UserID: user, ID: opID})
	if e != nil {
		return authoring.Session{}, missing(e)
	}
	op, e := fromOperation(row)
	if e != nil {
		return authoring.Session{}, e
	}
	state, e := load(ctx, q, user, op.SessionID)
	if e != nil {
		return state, e
	}
	old := state.Revision
	if e = apply(&state, &op); e != nil {
		return state, e
	}
	if _, e = q.SetAuthoringOperation(ctx, sqlc.SetAuthoringOperationParams{UserID: user, ID: op.ID, JobID: op.JobID, Status: op.Status, FailureReason: op.FailureReason}); e != nil {
		return state, e
	}
	if e = s.writeSession(ctx, q, state, old); e != nil {
		return state, e
	}
	return state, tx.Commit()
}
func (s *Store) Bind(ctx context.Context, user, opID, jobID string) (authoring.Session, error) {
	return s.operationMutation(ctx, user, opID, func(state *authoring.Session, op *authoring.Operation) error {
		if op.JobID != "" && op.JobID != jobID {
			return authoring.ErrStale
		}
		if op.Status != "pending" && op.Status != "admitted" {
			return authoring.ErrStale
		}
		if state.ActiveRequestID != op.ID {
			return authoring.ErrStale
		}
		op.JobID, op.Status = jobID, "admitted"
		state.ActiveJobID = jobID
		for i := range state.Turns {
			if state.Turns[i].ID == op.ID {
				state.Turns[i].JobID = jobID
			}
		}
		return nil
	})
}
func (s *Store) Reject(ctx context.Context, user, opID, reason string) (authoring.Session, error) {
	return s.Reconcile(ctx, user, opID, "failed", reason, nil)
}
func (s *Store) Reconcile(ctx context.Context, user, opID, status, reason string, result *authoring.OperationResult) (authoring.Session, error) {
	return s.operationMutation(ctx, user, opID, func(state *authoring.Session, op *authoring.Operation) error {
		if op.Status == "done" || op.Status == "failed" || op.Status == "cancelled" {
			return nil
		}
		if status != "done" && status != "failed" && status != "cancelled" {
			return authoring.ErrInvalid
		}
		if state.ActiveRequestID != op.ID {
			return authoring.ErrStale
		}
		if state.Revision != op.BaseRevision+1 {
			// Settle the issued job without replacing newer direct work.
			op.Status, op.FailureReason = status, reason
			state.ActiveJobID, state.ActiveRequestID = "", ""
			state.Phase = "editing"
			for i := range state.Turns {
				if state.Turns[i].ID == op.ID {
					state.Turns[i].Status = "superseded"
				}
			}
			return bump(state)
		}
		if status == "done" && result == nil {
			return authoring.ErrOutput
		}
		if e := bump(state); e != nil {
			return e
		}
		op.Status, op.FailureReason = status, reason
		state.ActiveJobID, state.ActiveRequestID = "", ""
		state.FailureReason = reason
		if status == "done" {
			state.FailureReason = ""
			state.PendingRequest = ""
			state.WriteModel = result.WriteModel
			if op.Mode == authoring.Recommend {
				var input struct {
					CandidateCount int `json:"candidate_count"`
				}
				if err := json.Unmarshal(op.Payload, &input); err != nil {
					return authoring.ErrOutput
				}
				requested := candidateCount(input.CandidateCount)
				if requested == 0 || len(result.Candidates) != requested {
					return authoring.ErrOutput
				}
				// The visible batch keeps its previous count until a complete
				// replacement has passed validation; failed/cancelled work keeps it.
				state.RequestedCandidateCount = requested
				state.Candidates = append([]authoring.Artifact(nil), result.Candidates...)
				for i := range state.Candidates {
					state.Candidates[i].Revision = state.Revision
					if source := state.WorkingSource; source != nil {
						state.Candidates[i].TargetLength, state.Candidates[i].TagCount, state.Candidates[i].Scope = source.TargetLength, source.TagCount, source.Scope
						state.Candidates[i].TemplateIDs, state.Candidates[i].Fields = append([]string(nil), source.TemplateIDs...), append([]string(nil), source.Fields...)
					}
				}
				state.Purpose = result.Purpose
				state.Phase = "choosing"
			} else {
				if result.Selected == nil {
					return authoring.ErrOutput
				}
				a := *result.Selected
				// The model owns content only. Retain explicit owner numbers and scope from the admitted source.
				if source := state.WorkingSource; source != nil {
					a.TargetLength, a.TagCount, a.Scope = source.TargetLength, source.TagCount, source.Scope
					a.TemplateIDs, a.Fields = append([]string(nil), source.TemplateIDs...), append([]string(nil), source.Fields...)
					a.BuilderState = source.BuilderState
				}
				a.Revision = state.Revision
				state.Selected = &a
				state.WorkingSource = &a
				state.DraftState = authoring.DraftValid
				state.HasUnpublishedChanges = !sameContent(state.WorkingSource, state.SavedBaseline)
				state.Phase = "editing"
			}
			for i := range state.Turns {
				if state.Turns[i].ID == op.ID {
					state.Turns[i].Status = "done"
					state.Turns[i].Reply = result.Reply
				}
			}
		} else {
			state.Phase = "failed"
			for i := range state.Turns {
				if state.Turns[i].ID == op.ID {
					state.Turns[i].Status = status
				}
			}
		}
		return nil
	})
}
func (s *Store) Select(ctx context.Context, user, id string, revision uint32, candidateID string) (authoring.Session, error) {
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return authoring.Session{}, e
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	state, e := load(ctx, q, user, id)
	if e != nil {
		return state, e
	}
	if state.Revision != revision {
		return state, authoring.ErrStale
	}
	if active(state) || state.Phase == "saved" {
		return state, authoring.ErrBusy
	}
	var selected *authoring.Artifact
	for _, a := range state.Candidates {
		if a.ID == candidateID {
			copy := a
			selected = &copy
			break
		}
	}
	if selected == nil {
		return state, authoring.ErrNoSelection
	}
	if e = bump(&state); e != nil {
		return state, e
	}
	selected.Revision = state.Revision
	state.Selected = selected
	state.WorkingSource = selected
	state.DraftState = authoring.DraftValid
	state.HasUnpublishedChanges = !sameContent(state.WorkingSource, state.SavedBaseline)
	state.Phase = "editing"
	state.FailureReason = ""
	if e = s.writeSession(ctx, q, state, revision); e != nil {
		return state, e
	}
	return state, tx.Commit()
}
func (s *Store) PrepareSave(ctx context.Context, user, id string, revision uint32, makeDefault bool) (authoring.Session, authoring.Publication, error) {
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return authoring.Session{}, authoring.Publication{}, e
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	state, e := load(ctx, q, user, id)
	if e != nil {
		return state, authoring.Publication{}, e
	}
	if state.Publication != nil && (state.Phase == "saving" || state.Phase == "saved") {
		return state, *state.Publication, tx.Commit()
	}
	if state.Revision != revision {
		return state, authoring.Publication{}, authoring.ErrStale
	}
	if active(state) {
		return state, authoring.Publication{}, authoring.ErrBusy
	}
	if state.Selected == nil {
		return state, authoring.Publication{}, authoring.ErrNoSelection
	}
	if state.DraftState != authoring.DraftValid {
		return state, authoring.Publication{}, authoring.ErrDraftInvalid
	}
	target := state.TargetID
	if target == "" && state.Saved != nil {
		target = state.Saved.ID
	}
	p := authoring.Publication{Key: fmt.Sprintf("%s:%d", state.ID, state.Revision), UserID: user, SessionID: id, Kind: state.Kind, Revision: state.Revision, Artifact: *state.Selected, TargetID: target, TargetVersion: state.TargetVersion, MakeDefault: makeDefault, WriteModel: state.WriteModel}
	if e = bump(&state); e != nil {
		return state, p, e
	}
	state.Phase = "saving"
	state.Publication = &p
	state.FailureReason = ""
	if e = s.writeSession(ctx, q, state, revision); e != nil {
		return state, p, e
	}
	return state, p, tx.Commit()
}
func (s *Store) FinalizeSave(ctx context.Context, user, id, key string, ref authoring.SavedRef) (authoring.Session, error) {
	return s.FinalizeSaveVersion(ctx, user, id, key, ref, "")
}
func (s *Store) FinalizeSaveVersion(ctx context.Context, user, id, key string, ref authoring.SavedRef, version string) (authoring.Session, error) {
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return authoring.Session{}, e
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	state, e := load(ctx, q, user, id)
	if e != nil {
		return state, e
	}
	if state.Publication == nil || state.Publication.Key != key {
		return state, authoring.ErrStale
	}
	if state.Phase == "saved" {
		return state, tx.Commit()
	}
	if state.Phase != "saving" || ref.Kind != state.Kind || ref.ID == "" {
		return state, authoring.ErrPublication
	}
	old := state.Revision
	if e = bump(&state); e != nil {
		return state, e
	}
	state.Saved = &ref
	if version != "" {
		state.TargetVersion = version
	}
	state.SavedAvailable = true
	state.HasUnpublishedChanges = false
	if state.Selected != nil {
		a := *state.Selected
		state.SavedBaseline = &a
		state.WorkingSource = &a
	}
	state.Phase = "saved"
	state.FailureReason = ""
	if e = s.writeSession(ctx, q, state, old); e != nil {
		return state, e
	}
	raw, e := rowReceipt(ctx, q, user, id)
	if e != nil {
		return state, e
	}
	if e = q.ConfirmAuthoringSaveMutations(ctx, sqlc.ConfirmAuthoringSaveMutationsParams{Response: raw, UserID: user, SessionID: id, ExpectedRevision: int64(state.Publication.Revision)}); e != nil {
		return state, e
	}
	// A reopened view can confirm the same pending publication with a new key
	// at its prepared revision. Confirm that receipt atomically as well.
	if e = q.ConfirmAuthoringSaveMutations(ctx, sqlc.ConfirmAuthoringSaveMutationsParams{Response: raw, UserID: user, SessionID: id, ExpectedRevision: int64(old)}); e != nil {
		return state, e
	}
	return state, tx.Commit()
}

var _ authoring.Store = (*Store)(nil)
