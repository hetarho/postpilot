package template

import (
	"context"
	"time"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). Ownership is a property of the query rather than of a check the
// caller must remember: every method takes the account and scopes its SQL by it, so a
// same-shaped id from another account reads as missing.
type Store interface {
	// Insert refuses past the cap INSIDE its own transaction, so two concurrent creates
	// cannot both pass a count taken before either wrote.
	Insert(ctx context.Context, t Template, maxPerAccount int) error
	// List returns the account's templates with PostCount populated, ordered by name then id.
	List(ctx context.Context, userID string) ([]Template, error)
	Get(ctx context.Context, userID, id string) (Template, error)
	// Update applies only the present fields of the patch, one statement per field in one
	// transaction, so a field saved concurrently from elsewhere is not read-modify-written
	// back to its old value. It reports ErrNotFound for an unknown or foreign id and
	// ErrDuplicateName on collision.
	Update(ctx context.Context, userID, id string, patch Patch, updatedAt time.Time) (Template, error)
	// Delete removes the template and detaches the posts pointing at it IN ONE TRANSACTION,
	// returning how many were detached. Splitting the two would let a crash in between
	// leave posts naming a template that no longer exists.
	Delete(ctx context.Context, userID, id string) (int, error)
}
