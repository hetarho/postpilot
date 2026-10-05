package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
	"time"
)

func decodeSpeechRun(raw, id string) (clip.SpeechRun, error) {
	r, e := decodeSpeechManifest(raw)
	r.JobID = id
	return r, e
}
func (s *Store) FindSpeechRun(ctx context.Context, owner, project, key string) (clip.SpeechRun, error) {
	row, e := s.read.FindClipSpeechRun(ctx, sqlc.FindClipSpeechRunParams{OwnerID: owner, ProjectID: project, RequestKey: key})
	if e != nil {
		return clip.SpeechRun{}, dbError(e)
	}
	return decodeSpeechRun(row.OperationJson, row.JobID)
}
func (s *Store) GetSpeechRun(ctx context.Context, owner, id string) (clip.SpeechRun, error) {
	row, e := s.read.GetClipSpeechRun(ctx, sqlc.GetClipSpeechRunParams{OwnerID: owner, ID: id})
	if e != nil {
		return clip.SpeechRun{}, dbError(e)
	}
	return decodeSpeechRun(row.OperationJson, row.JobID)
}
func (s *Store) ReserveSpeechRun(ctx context.Context, r clip.SpeechRun) (clip.SpeechRun, error) {
	raw, e := encodeSpeechManifest(r)
	if e != nil {
		return r, e
	}
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SpeechRun, error) {
		_, e := q.ReserveClipSpeechRun(ctx, sqlc.ReserveClipSpeechRunParams{ID: r.ID, OwnerID: r.OwnerID, ProjectID: r.ProjectID, RequestKey: r.RequestKey, RequestDigest: r.RequestDigest, OperationJson: raw, CreatedAt: stamp(time.Now()), ProjectCheck: r.ProjectID, OwnerCheck: r.OwnerID, RevisionCheck: int64(r.Revision)})
		if e != nil {
			return r, e
		}
		row, e := q.FindClipSpeechRun(ctx, sqlc.FindClipSpeechRunParams{OwnerID: r.OwnerID, ProjectID: r.ProjectID, RequestKey: r.RequestKey})
		if e != nil {
			return r, dbError(e)
		}
		prior, e := decodeSpeechRun(row.OperationJson, row.JobID)
		if e != nil {
			return r, e
		}
		if prior.RequestDigest != r.RequestDigest {
			return r, clip.ErrPlanConflict
		}
		return prior, nil
	})
}
func (s *Store) BindSpeechRun(ctx context.Context, owner, id, job string) error {
	_, e := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		row, e := q.GetClipSpeechRun(ctx, sqlc.GetClipSpeechRunParams{OwnerID: owner, ID: id})
		if e != nil {
			return struct{}{}, e
		}
		r, e := decodeSpeechRun(row.OperationJson, row.JobID)
		if e != nil {
			return struct{}{}, e
		}
		if e = affected(q.BindClipSpeechRun(ctx, sqlc.BindClipSpeechRunParams{OwnerID: owner, ID: id, JobID: job, SameJob: job})); e != nil {
			return struct{}{}, e
		}
		for _, c := range r.Calls {
			raw, e := encodeSpeechManifest(r)
			if e != nil {
				return struct{}{}, e
			}
			now := stamp(time.Now())
			if e = q.ReserveClipSpeechCall(ctx, sqlc.ReserveClipSpeechCallParams{ID: id + ":" + c.SegmentID, OwnerID: owner, ProjectID: r.ProjectID, JobID: job, PlanRevision: int64(r.Revision), SegmentID: c.SegmentID, InputHash: c.InputHash, BindingDigest: r.Voice.Binding.Digest, OperationJson: raw, CreatedAt: now, UpdatedAt: now}); e != nil {
				return struct{}{}, e
			}
		}
		return struct{}{}, nil
	})
	return e
}
func (s *Store) ClaimSpeechCall(ctx context.Context, r clip.SpeechRun, c clip.SpeechCall) error {
	n, e := s.write.ClaimClipSpeechCall(ctx, sqlc.ClaimClipSpeechCallParams{OwnerID: r.OwnerID, ProjectID: r.ProjectID, JobID: r.JobID, SegmentID: c.SegmentID, InputHash: c.InputHash, BindingDigest: r.Voice.Binding.Digest, UpdatedAt: stamp(time.Now())})
	if e != nil {
		return e
	}
	if n != 1 {
		return clip.ErrPlanConflict
	}
	return nil
}
func (s *Store) FinishSpeechCall(ctx context.Context, r clip.SpeechRun, c clip.SpeechCall, state, asset string) error {
	prior, e := s.read.GetClipSpeechCallState(ctx, sqlc.GetClipSpeechCallStateParams{OwnerID: r.OwnerID, ProjectID: r.ProjectID, JobID: r.JobID, SegmentID: c.SegmentID})
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	allowed := prior == "reserved" && (state == "failed" || state == "cancelled" || state == "unresolved") || prior == "claimed" && (state == "received" || state == "failed" || state == "cancelled" || state == "unresolved") || prior == "received" && (state == "published" || state == "obsolete")
	if !allowed {
		return nil
	}
	_, e = s.write.FinishClipSpeechCall(ctx, sqlc.FinishClipSpeechCallParams{Owner: r.OwnerID, Project: r.ProjectID, Job: r.JobID, Segment: c.SegmentID, NextState: state, ExpectedState: prior, Asset: nullable(asset), Now: stamp(time.Now())})
	return e
}

func (s *Store) ListIncompleteSpeechRuns(ctx context.Context) ([]clip.SpeechRun, error) {
	rows, e := s.read.ListIncompleteClipSpeechRuns(ctx)
	if e != nil {
		return nil, e
	}
	out := make([]clip.SpeechRun, 0, len(rows))
	for _, row := range rows {
		r, e := decodeSpeechRun(row.OperationJson, row.JobID)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, nil
}

type speechManifest struct {
	Version int
	Run     clip.SpeechRun
}

func encodeSpeechManifest(r clip.SpeechRun) (string, error) {
	b, e := json.Marshal(speechManifest{1, r})
	return string(b), e
}
func decodeSpeechManifest(raw string) (clip.SpeechRun, error) {
	var envelope speechManifest
	if e := clip.StrictJSON(raw, &envelope); e != nil || envelope.Version != 1 {
		return clip.SpeechRun{}, clip.ErrInvalid
	}
	return envelope.Run, nil
}
