package memory

import (
	"context"
	"time"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). Ownership is a property of the query rather than of a check the
// caller must remember: every method takes the account and scopes its SQL by it, so a
// same-shaped id from another account reads as missing.
type Store interface {
	// Insert applies the whole create rule in ONE transaction: the exact-text lookup, the
	// cap and the write. It cannot be split — the cap read in the service and the insert
	// here would let two concurrent creates both read max-1 and both commit, and a dedupe
	// checked in the service would let two approvals of one text both insert.
	//
	// A text the account already holds inserts nothing: the source link is added, the
	// memory's last use advances to seenAt, and the EXISTING row comes back with
	// deduplicated true. Its kind and tags are left exactly as they were — a re-approval
	// is a second sighting of a fact, not an edit of it (MEM-9).
	//
	// sourcePostSlug is empty for a memory written by hand, which then has no link at all.
	Insert(ctx context.Context, m Memory, sourcePostSlug string, maxPerAccount int) (stored Memory, deduplicated bool, err error)
	// List returns the account's memories in injection order — most recently used first,
	// then most recently created — with tags and source links populated.
	List(ctx context.Context, userID string) ([]Memory, error)
	Get(ctx context.Context, userID, id string) (Memory, error)
	// Update applies only the present parts of the patch in one transaction. A present tag
	// set replaces the whole set; an absent one is not written at all, so a text edit
	// cannot revert tags another tab saved.
	Update(ctx context.Context, userID, id string, patch Patch, updatedAt time.Time) (Memory, error)
	Delete(ctx context.Context, userID, id string) error
	// DropPostSources carries out MEM-17 in one transaction: it drops every link naming one
	// post and then deletes exactly those memories the dropped link was the last one of. A
	// memory written by hand has no links at all and is never touched by it, which is why
	// the deletion is scoped to the memories that just lost a link rather than to every
	// memory with none.
	DropPostSources(ctx context.Context, userID, postSlug string) error
}
