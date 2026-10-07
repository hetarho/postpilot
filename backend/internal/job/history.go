package job

import (
	"context"
	"errors"
)

// HistoryReads is a generic bulk read. Kind selection is the caller's domain policy.
// The latest relevant row is selected first; an older failure cannot mask a later success.
type HistoryReads interface {
	LatestForSubjects(context.Context, string, string, []string, []string) (map[string]Job, error)
}

func (q *Queue) LatestForSubjects(ctx context.Context, userID, dimension string, ids, kinds []string) (map[string]JobSummary, error) {
	if userID == "" || dimension == "" {
		return nil, ErrInvalidTarget
	}
	source, ok := q.store.(HistoryReads)
	if !ok {
		return nil, errors.New("job store does not support bulk history")
	}
	rows, err := source.LatestForSubjects(ctx, userID, dimension, ids, kinds)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	out := map[string]JobSummary{}
	for id, row := range rows {
		if row.UserID != userID || row.Subject(dimension) != id || !allowed[id] {
			return nil, ErrNotFound
		}
		out[id] = *q.summarize(row)
	}
	return out, nil
}
