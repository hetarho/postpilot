package auth

import (
	"context"

	"github.com/postpilot/backend/internal/plan"
)

// Actor is the authenticated caller as every downstream handler sees it: who is acting,
// and with what authority. The plan rides along because authorization questions are
// asked far from the session — a model floor, a quota axis, a master-only procedure —
// and none of them may re-read a table to find out who is asking.
type Actor struct {
	UserID string
	Plan   plan.Plan
}

// actorKeyType is unexported so no other package can write this context value — the
// acting user is set by the interceptor and only by the interceptor.
type actorKeyType struct{}

var actorKey actorKeyType

// WithActor returns a context carrying the authenticated caller.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// WithUser returns a context carrying an authenticated user id and no plan. It is the
// narrow form for callers that only prove identity; anything that gates on authority
// reads no plan from such a context and must therefore fail closed.
func WithUser(ctx context.Context, userID string) context.Context {
	return WithActor(ctx, Actor{UserID: userID})
}

// UserFromContext returns the authenticated user id placed by the interceptor.
//
// Every authenticated handler takes the acting user from here and never from a request
// payload — a user id in a message is a claim by the caller, not a fact.
func UserFromContext(ctx context.Context) (string, bool) {
	actor, ok := ctx.Value(actorKey).(Actor)
	return actor.UserID, ok && actor.UserID != ""
}

// PlanFromContext returns the acting account's plan. A false result means the caller's
// authority is unknown, which every gate must treat as "not allowed" rather than as a
// default tier.
func PlanFromContext(ctx context.Context) (plan.Plan, bool) {
	actor, ok := ctx.Value(actorKey).(Actor)
	return actor.Plan, ok && actor.Plan.Valid()
}
