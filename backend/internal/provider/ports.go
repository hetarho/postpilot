package provider

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// Store is the persistence this context needs.
type Store interface {
	UpsertSelection(ctx context.Context, userID string, s Selection) error
	ListSelections(ctx context.Context, userID string) ([]Selection, error)
	ListSelectionSlots(ctx context.Context, userID string) ([]Selection, error)
	SaveSelections(ctx context.Context, userID string, selections []Selection) error
	// DeleteSelection removes the stage's row only while it still holds `s.Ref`. The
	// clear of a vanished choice runs after a read, and a save the user made in between
	// must not be taken with it.
	DeleteSelection(ctx context.Context, userID string, s Selection) error
}

// Catalog is what this context reads from the model registry — declared here by its
// consumer (ARCHITECTURE §2.2). *llm.Registry satisfies it.
type Catalog interface {
	Models() []llm.ModelInfo
	Lookup(ref llm.ModelRef) (llm.ModelInfo, bool)
	RecommendationSets() []llm.RecommendationSet
}

// PlannedCall is one model some work would run, and how many times. It is this context's
// own shape rather than the ledger's: a port is declared by its consumer, and the
// composition root maps between the two.
type PlannedCall struct {
	Ref   llm.ModelRef
	Count int
}

// Credits prices work for the calling account.
//
// The picker asks the SAME estimator the gate will apply when the work actually starts,
// so what a user is shown and what they are charged can never be computed two different
// ways. Affordability is the only access rule this context has left: there is no plan
// floor to compare against any more.
type Credits interface {
	ForCalls(calls []PlannedCall) int
	// Balance reports what the account may spend, and whether it is exempt from the
	// balance entirely (the operator tier).
	Balance(ctx context.Context, userID string) (credits int, unlimited bool, err error)
}
