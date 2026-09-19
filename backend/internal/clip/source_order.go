package clip

import (
	"context"
)

// SourceOrderStore is the one write this action needs.
type SourceOrderStore interface {
	ReorderSources(ctx context.Context, user, project, batch string, ids []string) (SourceBatch, error)
}
