package app

import (
	"context"
	"database/sql"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/post"
)

type OrdinaryWriteJobs interface {
	ActiveFor(context.Context, job.Subject, job.Filter) (*job.Job, error)
}
type OrdinaryWriteJobBinder func(*sql.Tx) OrdinaryWriteJobs
type PostTestJobGuard struct {
	bind  OrdinaryWriteJobBinder
	kinds []string
}

func NewPostTestJobGuard(bind OrdinaryWriteJobBinder, kinds []string) *PostTestJobGuard {
	if bind == nil || len(kinds) == 0 {
		panic("experiment/app: transactional ordinary job binding and kinds are required")
	}
	for _, kind := range kinds {
		if kind == "" {
			panic("experiment/app: ordinary write kind cannot be empty")
		}
	}
	return &PostTestJobGuard{bind: bind, kinds: append([]string(nil), kinds...)}
}
func (g *PostTestJobGuard) HasOrdinaryWrite(ctx context.Context, tx *sql.Tx, user, slug string) (bool, error) {
	jobs := g.bind(tx)
	if jobs == nil {
		panic("experiment/app: ordinary job binder returned no owner port")
	}
	for _, kind := range g.kinds {
		active, err := jobs.ActiveFor(ctx, job.Subject{Dimension: post.JobSubject, ID: slug}, job.Filter{UserID: user, Kind: kind})
		if err != nil {
			return false, err
		}
		if active != nil {
			return true, nil
		}
	}
	return false, nil
}
