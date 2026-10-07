package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/authoring/store/sqlc"
)

func bit(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
func candidateCount(n int) int { count, _ := authoring.NormalizeCandidateCount(n); return count }
func encodeArtifact(a *authoring.Artifact) sql.NullString {
	if a == nil {
		return sql.NullString{}
	}
	raw, _ := json.Marshal(artifactToSnapshot(*a))
	return sql.NullString{String: string(raw), Valid: true}
}
func decodeArtifact(raw sql.NullString) (*authoring.Artifact, error) {
	if !raw.Valid {
		return nil, nil
	}
	var a artifactSnapshot
	if err := json.Unmarshal([]byte(raw.String), &a); err != nil {
		return nil, err
	}
	result := artifactFromSnapshot(a)
	return &result, nil
}
func sameContent(a, b *authoring.Artifact) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Name == b.Name && a.Description == b.Description && a.Body == b.Body && a.TitleArea == b.TitleArea
}
func displayName(s authoring.Session) string {
	if s.WorkingSource != nil && strings.TrimSpace(s.WorkingSource.Name) != "" {
		return s.WorkingSource.Name
	}
	for _, a := range []*authoring.Artifact{s.SavedBaseline, s.Selected, s.WorkingSource} {
		if a == nil {
			continue
		}
		if strings.TrimSpace(a.Name) != "" {
			return a.Name
		}
		summary := []rune(strings.Join(strings.Fields(a.Body), " "))
		if len(summary) > authoring.DisplaySummaryMaxChars {
			summary = summary[:authoring.DisplaySummaryMaxChars]
		}
		if len(summary) > 0 {
			return string(summary)
		}
	}
	return ""
}
func mutationFingerprint(action string, revision uint32, payload any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", action, revision, raw)))
	return hex.EncodeToString(sum[:])
}
func rowReceipt(ctx context.Context, q *sqlc.Queries, user, id string) (string, error) {
	row, err := q.GetAuthoringSession(ctx, sqlc.GetAuthoringSessionParams{UserID: user, ID: id})
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(row)
	return string(raw), err
}

// The response is committed in the same transaction as the CAS. A lost response
// returns the original result even after subsequent edits.
func (s *Store) mutate(ctx context.Context, in authoring.ResetMutation, action string, payload any, apply func(*authoring.Session) error) (authoring.Session, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return authoring.Session{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	state, err := load(ctx, q, in.UserID, in.SessionID)
	if err != nil {
		return state, err
	}
	fingerprint := mutationFingerprint(action, in.ExpectedRevision, payload)
	if in.OperationKey != "" {
		prior, err := q.GetAuthoringMutation(ctx, sqlc.GetAuthoringMutationParams{UserID: in.UserID, OperationKey: in.OperationKey})
		if err == nil {
			if prior.SessionID != in.SessionID || prior.Action != action || prior.Fingerprint != fingerprint {
				return state, authoring.ErrStale
			}
			var row sqlc.ConfigurationAuthoringSession
			if err = json.Unmarshal([]byte(prior.Response), &row); err != nil {
				return state, err
			}
			state, err = fromSession(row)
			if err != nil {
				return state, err
			}
			return state, tx.Commit()
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return state, err
		}
	}
	if state.Revision != in.ExpectedRevision {
		return state, authoring.ErrStale
	}
	recoveringSave := action == "save" && state.Phase == "saving" && state.Publication != nil
	if state.Phase == "saving" && !recoveringSave {
		return state, authoring.ErrBusy
	}
	if err = apply(&state); err != nil {
		return state, err
	}
	if !recoveringSave {
		if err = bump(&state); err != nil {
			return state, err
		}
	}
	state.UpdatedAt = s.now().UTC()
	if err = s.writeSession(ctx, q, state, in.ExpectedRevision); err != nil {
		return state, err
	}
	if in.OperationKey != "" {
		raw, err := rowReceipt(ctx, q, in.UserID, in.SessionID)
		if err != nil {
			return state, err
		}
		err = q.InsertAuthoringMutation(ctx, sqlc.InsertAuthoringMutationParams{UserID: in.UserID, SessionID: in.SessionID, OperationKey: in.OperationKey, Action: action, ExpectedRevision: int64(in.ExpectedRevision), Fingerprint: fingerprint, Response: raw, CreatedAt: state.UpdatedAt.Format(stampLayout)})
		if err != nil {
			return state, err
		}
	}
	return state, tx.Commit()
}
func (s *Store) PatchDraftState(ctx context.Context, in authoring.DraftMutation, validity authoring.DraftState) (authoring.Session, error) {
	return s.mutate(ctx, authoring.ResetMutation{UserID: in.UserID, SessionID: in.SessionID, ExpectedRevision: in.ExpectedRevision, OperationKey: in.OperationKey}, "patch", in.WorkingSource, func(state *authoring.Session) error {
		a := in.WorkingSource
		// Identity is server-owned. Manual fields cannot substitute a contestant.
		if state.WorkingSource != nil {
			a.ID = state.WorkingSource.ID
		} else {
			a.ID = state.ID + "-manual"
		}
		a.Revision = state.Revision + 1
		state.WorkingSource = &a
		state.DraftState = validity
		if validity == authoring.DraftValid {
			copy := a
			state.Selected = &copy
		}
		state.HasUnpublishedChanges = !sameContent(state.WorkingSource, state.SavedBaseline)
		state.Publication = nil
		state.FailureReason = ""
		if state.ActiveRequestID == "" {
			state.Phase = "editing"
		}
		return nil
	})
}
func (s *Store) ResetChat(ctx context.Context, in authoring.ResetMutation) (authoring.Session, error) {
	return s.mutate(ctx, in, "reset_chat", nil, func(state *authoring.Session) error {
		if active(*state) {
			return authoring.ErrBusy
		}
		state.Turns = []authoring.Turn{}
		state.PendingRequest = ""
		state.FailureReason = ""
		if state.Phase == "saved" {
			state.Phase = "editing"
			state.Publication = nil
		}
		return nil
	})
}
func (s *Store) ResetBaseline(ctx context.Context, in authoring.ResetMutation) (authoring.Session, error) {
	return s.mutate(ctx, in, "reset_baseline", nil, func(state *authoring.Session) error {
		if active(*state) {
			return authoring.ErrBusy
		}
		if state.SavedBaseline == nil {
			return authoring.ErrNoSelection
		}
		a := *state.SavedBaseline
		a.Revision = state.Revision + 1
		state.WorkingSource = &a
		copy := a
		state.Selected = &copy
		state.DraftState = authoring.DraftValid
		state.HasUnpublishedChanges = false
		state.Phase = "editing"
		state.Publication = nil
		state.FailureReason = ""
		state.Turns = []authoring.Turn{}
		state.PendingRequest = ""
		return nil
	})
}
func (s *Store) SelectWithKey(ctx context.Context, in authoring.ResetMutation, candidateID string) (authoring.Session, error) {
	return s.mutate(ctx, in, "select", candidateID, func(state *authoring.Session) error {
		if active(*state) || state.Phase == "saved" {
			return authoring.ErrBusy
		}
		for _, candidate := range state.Candidates {
			if candidate.ID == candidateID {
				candidate.Revision = state.Revision + 1
				state.Selected = &candidate
				copy := candidate
				state.WorkingSource = &copy
				state.DraftState = authoring.DraftValid
				state.HasUnpublishedChanges = !sameContent(state.WorkingSource, state.SavedBaseline)
				state.Phase = "editing"
				state.FailureReason = ""
				return nil
			}
		}
		return authoring.ErrNoSelection
	})
}

type summaryCursor struct{ Time, ID string }

func (s *Store) ListSummaries(ctx context.Context, in authoring.SummaryQuery) ([]authoring.Summary, string, error) {
	limit := in.PageSize
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, "", authoring.ErrInvalid
	}
	var cursor summaryCursor
	if in.PageToken != "" {
		raw, err := base64.RawURLEncoding.DecodeString(in.PageToken)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.ID == "" {
			return nil, "", authoring.ErrInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, cursor.Time); err != nil {
			return nil, "", authoring.ErrInvalid
		}
	}
	rows, err := s.read.ListAuthoringSummaries(ctx, sqlc.ListAuthoringSummariesParams{UserID: in.UserID, Kind: string(in.Kind), UnsavedOnly: bit(in.UnsavedOnly), CursorTime: cursor.Time, CursorID: cursor.ID, PageLimit: int64(limit + 1)})
	if err != nil {
		return nil, "", err
	}
	out := make([]authoring.Summary, 0, len(rows))
	next := ""
	for i, row := range rows {
		if i == limit {
			prior := rows[i-1]
			raw, _ := json.Marshal(summaryCursor{prior.UpdatedAt, prior.ID})
			next = base64.RawURLEncoding.EncodeToString(raw)
			break
		}
		if row.Revision < 0 || row.Revision > math.MaxUint32 {
			return nil, "", authoring.ErrInvalid
		}
		updated, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
		if err != nil {
			return nil, "", err
		}
		summary := authoring.Summary{SessionID: row.ID, Kind: authoring.Kind(row.Kind), TargetID: row.TargetID, Revision: uint32(row.Revision), SavedAvailable: row.SavedAvailable != 0, HasUnpublishedChanges: row.HasUnpublishedChanges != 0, ActiveJobID: fmt.Sprint(row.ActiveJobID), PublicationPending: row.PublicationPending != 0, TargetConflict: row.TargetConflict != 0, DisplayName: row.DisplayName, DraftState: authoring.DraftState(row.DraftState), UpdatedAt: updated}
		var saved *savedSnapshot
		if err := json.Unmarshal([]byte(fmt.Sprint(row.LastPublication)), &saved); err != nil {
			return nil, "", err
		}
		if saved != nil {
			summary.LastPublication = &authoring.SavedRef{Kind: authoring.Kind(saved.Kind), ID: saved.ID, Name: saved.Name}
		}
		out = append(out, summary)
	}
	return out, next, nil
}
func (s *Store) PublicationFailure(ctx context.Context, user, id, reason string, missing bool) error {
	_, err := s.operationFreeUpdate(ctx, user, id, func(state *authoring.Session) {
		state.FailureReason = reason
		if missing {
			state.SavedAvailable = false
		}
	})
	return err
}
func (s *Store) operationFreeUpdate(ctx context.Context, user, id string, apply func(*authoring.Session)) (authoring.Session, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return authoring.Session{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	state, err := load(ctx, q, user, id)
	if err != nil {
		return state, err
	}
	old := state.Revision
	apply(&state)
	if err = s.writeSession(ctx, q, state, old); err != nil {
		return state, err
	}
	return state, tx.Commit()
}

func (s *Store) PrepareSaveWithKey(ctx context.Context, in authoring.ResetMutation, makeDefault bool, validate func(authoring.Kind, authoring.Artifact) error) (authoring.Session, authoring.Publication, error) {
	state, err := s.mutate(ctx, in, "save", makeDefault, func(state *authoring.Session) error {
		if state.Phase == "saving" && state.Publication != nil {
			return nil
		}
		if active(*state) || state.Phase == "saved" {
			return authoring.ErrBusy
		}
		if state.Selected == nil {
			return authoring.ErrNoSelection
		}
		if state.DraftState != authoring.DraftValid {
			return authoring.ErrDraftInvalid
		}
		if err := validate(state.Kind, *state.Selected); err != nil {
			return authoring.ErrDraftInvalid
		}
		target := state.TargetID
		if target == "" && state.Saved != nil {
			target = state.Saved.ID
		}
		state.Publication = &authoring.Publication{Key: fmt.Sprintf("%s:%d", state.ID, state.Revision), UserID: in.UserID, SessionID: state.ID, Kind: state.Kind, Revision: state.Revision, Artifact: *state.Selected, TargetID: target, TargetVersion: state.TargetVersion, MakeDefault: makeDefault, WriteModel: state.WriteModel}
		state.Phase = "saving"
		state.FailureReason = ""
		return nil
	})
	if err != nil || state.Publication == nil {
		return state, authoring.Publication{}, err
	}
	return state, *state.Publication, nil
}

// Candidate provenance belongs to the admitted recommendation, not to later
// session publication or direct edits.
func (s *Store) CandidateContext(ctx context.Context, user string, ref authoring.OwnedCandidateRef) (string, string, error) {
	operationID, _, found := strings.Cut(ref.CandidateID, "-")
	if !found {
		return "", "", authoring.ErrNotFound
	}
	row, err := s.read.GetAuthoringOperationByID(ctx, sqlc.GetAuthoringOperationByIDParams{UserID: user, ID: operationID})
	if err != nil {
		return "", "", missing(err)
	}
	if row.SessionID != ref.SessionID || row.Mode != string(authoring.Recommend) || row.Status != "done" || uint64(row.BaseRevision)+2 != uint64(ref.Revision) {
		return "", "", authoring.ErrNotFound
	}
	var input struct {
		TargetID      string `json:"target_id"`
		TargetVersion string `json:"target_version"`
	}
	if err = json.Unmarshal(row.Payload, &input); err != nil {
		return "", "", err
	}
	return input.TargetID, input.TargetVersion, nil
}
