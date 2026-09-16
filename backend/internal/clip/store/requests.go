package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// What the owner asked the AI for, kept with the project (CLIP-133). The write
// point is ACCEPTANCE, never a save: the record answers what the AI actually
// read, so an instruction typed and never used leaves nothing here. Text is
// stored exactly as written — no trimming, no normalising, no deduplication
// against the entry before it.
func (s *Store) RecordProjectRequest(ctx context.Context, id, user, project string, r clip.ProjectRequest) error {
	if id == "" || !clip.ValidRequestKind(r.Kind) {
		return clip.ErrInvalid
	}
	_, err := transact(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		return struct{}{}, q.RecordClipProjectRequest(ctx, sqlc.RecordClipProjectRequestParams{
			ID: id, ProjectID: project, UserID: user, Kind: r.Kind, Body: r.Body, CreatedAt: stamp(r.CreatedAt),
		})
	})
	return err
}

// ListProjectRequests answers newest first.
func (s *Store) ListProjectRequests(ctx context.Context, user, project string) ([]clip.ProjectRequest, error) {
	rows, err := s.read.ListClipProjectRequests(ctx, sqlc.ListClipProjectRequestsParams{ProjectID: project, UserID: user})
	if err != nil {
		return nil, err
	}
	out := make([]clip.ProjectRequest, 0, len(rows))
	for _, r := range rows {
		at, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, clip.ProjectRequest{Kind: r.Kind, Body: r.Body, CreatedAt: at})
	}
	return out, nil
}
