package store_test

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

// Component fixtures isolate the clip store; composition-level transaction and
// cancellation races are exercised against clipFinisher in cmd/api.
type generationFinisher struct{ store *store.Store }

func (f generationFinisher) Complete(ctx context.Context, c clip.AttemptResult) error {
	if c.EditPlan != "" {
		return f.store.SaveGeneration(ctx, c.UserID, c.ProjectID, c.Analysis, c.EditPlan, c.Result)
	}
	return f.store.SaveRender(ctx, c.UserID, c.ProjectID, c.ExpectedRevision, c.Result)
}
func (generationFinisher) Recover(context.Context) error { return nil }

type generationGuard struct {
	admission *clipAdmitter
	jobs      *jobstore.Store
}

func (g generationGuard) Reserve(ctx context.Context, start job.Start) error {
	return g.admission.Hold(ctx, start)
}
func (g generationGuard) Authorize(ctx context.Context, user, id string) error {
	return g.jobs.AuthorizeClipDispatch(ctx, user, id)
}
