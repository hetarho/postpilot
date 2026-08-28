package provider

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// Store is the persistence this context needs.
type Store interface {
	UpsertSelection(ctx context.Context, userID string, s Selection) error
	ListSelections(ctx context.Context, userID string) ([]Selection, error)
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
}
