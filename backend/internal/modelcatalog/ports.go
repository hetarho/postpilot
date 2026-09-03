package modelcatalog

import (
	"context"
	"time"
)

// Store is the persistence this context needs. catalog_models is global rather than
// per-account: what an installation offers is an operator decision; affordability against
// a balance is what differentiates who may run it (plan 17).
type Store interface {
	List(ctx context.Context) ([]Model, error)
	Get(ctx context.Context, modelID string) (Model, error)
	// Upsert writes a curated row, replacing the upstream snapshot but keeping the row's
	// creation time when it already exists.
	Upsert(ctx context.Context, m Model) error
	// Patch applies a partial curation edit and returns the row as stored. A missing row is
	// ErrNotFound.
	Patch(ctx context.Context, modelID string, patch Patch, updatedAt time.Time) (Model, error)
	// RegisterPurpose writes the row snapshot and the purpose registration in ONE
	// transaction, so a failure can never leave a curated row whose checkbox silently did
	// not stick. Idempotent per (model, purpose).
	RegisterPurpose(ctx context.Context, m Model, purpose Purpose) error
	// DeregisterPurpose removes one registration and stamps the row's updated_at — a
	// deregistration is a curation edit. The catalog row itself always survives.
	DeregisterPurpose(ctx context.Context, modelID string, purpose Purpose, at time.Time) error
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

// ReasoningSpendReader is the usage ledger's published per-model aggregate for one stage,
// declared here by its consumer and wired in the composition root. It exists because this
// context must NOT read usage_events — that table belongs to the ledger (ARCHITECTURE §2.2)
// — and because the only reliable way to tell whether a model honors its reasoning effort
// is to measure what it spent.
//
// The returned rows are keyed by model ref string, the same form the ledger records.
type ReasoningSpendReader interface {
	ReasoningSpendByModel(ctx context.Context, stage string) ([]SpendRow, error)
}

// SpendRow is one model's recorded split at the stage that was asked for. It is this
// context's own shape, so the ledger's type never crosses inward.
type SpendRow struct {
	Model            string
	Calls            int64
	ReasoningTokens  int64
	CompletionTokens int64
}
