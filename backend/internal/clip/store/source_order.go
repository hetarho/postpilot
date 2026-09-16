package store

import (
	"context"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// ReorderSources records the order the owner arranged this batch's footage in
// (CLIP-136). It is the whole batch or nothing: a list that misses a source,
// names one twice or names one this batch does not hold is refused, because a
// partial order would leave the rest of the footage where nobody put it.
//
// Nothing else moves. The plan, the observations and the retention are
// untouched: the order is writer input for the NEXT generation, and a clip
// already written keeps the flow it was written with.
func (s *Store) ReorderSources(ctx context.Context, user, project, batch string, ids []string) (clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		b, err := getSourceBatch(ctx, q, user, batch)
		if err != nil {
			return clip.SourceBatch{}, err
		}
		access, err := q.SourceProjectAccess(ctx, sqlc.SourceProjectAccessParams{ID: project, UserID: user})
		if err != nil {
			return clip.SourceBatch{}, dbError(err)
		}
		if b.ProjectID != project || access.Deleting != 0 || access.SourceAccessRevokedAt.Valid ||
			access.SourceBatchID.String != batch || b.State == "cleanup_pending" || b.State == "consuming" {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		held := []string{}
		for _, source := range b.Sources {
			if !source.CleanupPending {
				held = append(held, source.ID)
			}
		}
		if len(ids) != len(held) {
			return clip.SourceBatch{}, clip.ErrInvalid
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] || !slices.Contains(held, id) {
				return clip.SourceBatch{}, clip.ErrInvalid
			}
			seen[id] = true
		}
		for position, id := range ids {
			// Positions start at one: zero is what an unarranged lease carries,
			// and a batch that has been arranged says so in every row.
			if err := affected(q.SetSourceLeasePosition(ctx, sqlc.SetSourceLeasePositionParams{Position: int64(position + 1), CanonicalID: id, BatchID: batch, UserID: user})); err != nil {
				return clip.SourceBatch{}, err
			}
		}
		return getSourceBatch(ctx, q, user, batch)
	})
}
