package provider

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Store is the persistence this context needs.
type Store interface {
	UpsertSelection(ctx context.Context, userID string, s Selection) error
	ListSelections(ctx context.Context, userID string) ([]Selection, error)
	ListSelectionSlots(ctx context.Context, userID string) ([]Selection, error)
	SaveSelections(ctx context.Context, userID string, selections []Selection) error
	// InsertDefaultSelections atomically inserts absent active slots only. It must never
	// overwrite a concurrent manual save or write any comparison slot.
	InsertDefaultSelections(ctx context.Context, userID string, selections []Selection) error
	// ReplaceLabExtraCandidates validates the current pair and atomically replaces C/D/E.
	ReplaceLabExtraCandidates(ctx context.Context, userID string, stage Stage, extras []Selection) error
	// DeleteSelection removes the stage's row only while it still holds `s.Ref`. The
	// clear of a vanished choice runs after a read, and a save the user made in between
	// must not be taken with it.
	DeleteSelection(ctx context.Context, userID string, s Selection) error

	// Recommendation sets are installation-wide rows the operator curates (MODEL-69), listed
	// in the operator's order. Every write is one transaction, and none of them touches
	// model_selections (MODEL-71).
	ListRecommendationSets(ctx context.Context) ([]RecommendationSet, error)
	// CreateRecommendationSet appends a set after the last one, refusing with
	// ErrRecommendationLimit when `limit` sets already exist — counted inside the write.
	CreateRecommendationSet(ctx context.Context, set RecommendationSet, limit int, at time.Time) error
	// ReplaceRecommendationSet rewrites an existing set's label and all of its slots.
	ReplaceRecommendationSet(ctx context.Context, set RecommendationSet, at time.Time) error
	DeleteRecommendationSet(ctx context.Context, id string) error
	// MoveRecommendationSet swaps a set with its neighbour, towards the top when `earlier`;
	// at either end it changes nothing. Unknown ids are ErrRecommendationNotFound throughout.
	MoveRecommendationSet(ctx context.Context, id string, earlier bool) error
}

// Catalog is what this context reads from the model registry — declared here by its
// consumer (ARCHITECTURE §2.2). *llm.Registry satisfies it.
type Catalog interface {
	Models() []llm.ModelInfo
	Lookup(ref llm.ModelRef) (llm.ModelInfo, bool)
}

// PlannedCall is one model some work would run, and how many times. It is this context's
// own shape rather than the ledger's: a port is declared by its consumer, and the
// composition root maps between the two.
type PlannedCall struct {
	Ref          llm.ModelRef
	Count        int
	Stage        Stage
	NativeEffort bool
}

// Credits prices work for the calling account.
//
// The picker uses the admission estimator for affordability. Stage membership,
// provider/capability availability and account model-grade entitlement are separate
// access checks; a sufficient balance alone never authorizes a selection.
type Credits interface {
	ForCalls(calls []PlannedCall) int
	// Balance reports what the account may spend, and whether it is exempt from the
	// balance entirely (the operator tier).
	Balance(ctx context.Context, userID string) (credits int, unlimited bool, err error)
}

// TestedModelAdoptions rechecks current execution rights, then commits selection and receipt
// atomically. Existing receipts are replayed before live checks and never undo later edits.
type TestedModelAdoptions interface {
	AdoptTestModel(context.Context, TestModelAdoption) (TestModelReceipt, error)
}
