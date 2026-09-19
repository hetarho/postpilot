package store_test

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/store"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

// Component fixtures isolate the clip store; composition-level transaction and
// cancellation races are exercised against clipFinisher in cmd/api.
type generationFinisher struct{ store *store.Store }

func (f generationFinisher) Complete(ctx context.Context, c clip.AttemptResult) error {
	if c.EditPlan != "" {
		// A generation stops at the plan and carries no result of its own
		// (CLIP-151); only a legacy staged completion still brings one.
		if c.Result.Key == "" {
			return f.store.SaveGeneratedPlan(ctx, c.UserID, c.ProjectID, c.Analysis, c.EditPlan, time.Now())
		}
		return f.store.SaveGeneration(ctx, c.UserID, c.ProjectID, c.Analysis, c.EditPlan, c.Result)
	}
	return f.store.SaveRender(ctx, c.UserID, c.ProjectID, c.ExpectedRevision, c.Result)
}
func (generationFinisher) Recover(context.Context) error { return nil }

type generationGuard struct {
	admission *clipAdmitter
	jobs      *jobstore.Store
}

func (g generationGuard) Reserve(ctx context.Context, hold clipapp.Hold) error {
	return g.admission.Hold(ctx, hold)
}
func (g generationGuard) Authorize(ctx context.Context, user, id string) error {
	return g.jobs.AuthorizeDispatch(ctx, user, id)
}
