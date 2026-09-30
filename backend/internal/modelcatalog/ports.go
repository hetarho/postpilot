package modelcatalog

import (
	"context"
	"github.com/postpilot/backend/internal/llm"
	"time"
)

type FreeQualifier interface {
	QualifyFree(ctx context.Context, modelID string, path llm.FreePath) (bool, error)
}

// Store is the persistence this context needs. catalog_models is global rather than
// per-account: what an installation offers is an operator decision; affordability against
// a balance is what differentiates who may run it (QUOTA-19).
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
	// a fresh snapshot and last_seen_at, everything else is marked unlisted AND loses every
	// registration (MODEL-20) — the row survives, the registration-bound effort does not,
	// as on an operator's uncheck. It is one transaction so the catalog is never
	// half-refreshed.
	RefreshAvailability(ctx context.Context, seen []Candidate, at time.Time) error
	// SyncDocument applies a whole paste in ONE transaction (MODEL-53, MODEL-73): every
	// registration the document adds and every one it drops and, when `sets` is non-nil, the
	// complete recommendation-set list — or nothing at all. Each registration write is the
	// same one the single-model path makes — a registration upserts the row snapshot, a
	// deregistration deletes the registration row and takes its effort override with it.
	SyncDocument(ctx context.Context, writes []PurposeWrite, sets *[]StoredSet, at time.Time) error
	// RecommendationSets reads the stored sets in the operator's order, for the document's
	// diff and export (MODEL-72).
	RecommendationSets(ctx context.Context) ([]StoredSet, error)
	// ListCombos returns every estimator assignment that exists, in combo order. A combo
	// with no row is simply absent.
	ListCombos(ctx context.Context) ([]ComboAssignment, error)
	// AssignCombo replaces one combo's pair. The foreign keys mean an id that is not a
	// curated model is refused by the database, not only by the service.
	AssignCombo(ctx context.Context, a ComboAssignment, at time.Time) error
}

// RecommendationRows is the recommendation-set table as the models document reads and
// replaces it, bound to the document's own transaction (ARCH-6). It is declared here by its
// consumer and implemented in the composition root over the provider context's rows, so
// neither context imports the other.
type RecommendationRows interface {
	List(ctx context.Context) ([]StoredSet, error)
	// Replace makes the stored sets exactly `sets`, in order: a set carrying a stored ID keeps
	// its identity, a stored set the list omits is deleted, and an empty ID is a new set.
	Replace(ctx context.Context, sets []StoredSet, at time.Time) error
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
	Model                string
	Calls                int64
	ReasoningTokens      int64
	CompletionTokens     int64
	ReasoningTruncations int64
}
