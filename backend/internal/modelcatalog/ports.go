package modelcatalog

import (
	"context"
	"time"
)

// Store is the persistence this context needs. catalog_models is global rather than
// per-account: what an installation offers is an operator decision, and plan 17's per-model
// floor is what differentiates who may run it.
type Store interface {
	List(ctx context.Context) ([]Model, error)
	Get(ctx context.Context, modelID string) (Model, error)
	// Upsert writes a curated row, replacing the upstream snapshot but keeping the row's
	// creation time when it already exists.
	Upsert(ctx context.Context, m Model) error
	// Patch applies a partial curation edit and returns the row as stored. A missing row is
	// ErrNotFound.
	Patch(ctx context.Context, modelID string, patch Patch, updatedAt time.Time) (Model, error)
	// RefreshAvailability records what a SUCCESSFUL upstream read saw: the seen models get
	// a fresh snapshot and last_seen_at, everything else is marked unlisted. It is one
	// transaction so the catalog is never half-refreshed.
	RefreshAvailability(ctx context.Context, seen []Candidate, at time.Time) error
}

// Upstream is the provider's own catalog of models that exist — declared here by its
// consumer. Only the operator path calls it.
type Upstream interface {
	Fetch(ctx context.Context, refresh bool) (Snapshot, error)
}
