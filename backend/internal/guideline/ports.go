package guideline

import (
	"context"
	"time"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). Ownership is a property of the query rather than of a check the
// caller must remember: every method takes the account and scopes its SQL by it, so a
// same-shaped id from another account reads as missing.
type Store interface {
	// Insert refuses past maxPerAccount INSIDE its own transaction. The cap cannot be
	// checked in the service and enforced here: two concurrent creates would both read
	// max-1 and both win.
	Insert(ctx context.Context, g Guideline, maxPerAccount int) error
	// List returns the account's guidelines in injection order — the global group first,
	// then the scoped group, each by created_at then id — with TemplateIDs populated.
	List(ctx context.Context, userID string) ([]Guideline, error)
	Get(ctx context.Context, userID, id string) (Guideline, error)
	// Update applies only the present parts of the patch in one transaction. A present scope
	// replaces the kind and the whole link set together; an absent one is not written at all,
	// so a text edit cannot revert a scope another tab saved.
	Update(ctx context.Context, userID, id string, patch Patch, updatedAt time.Time) (Guideline, error)
	Delete(ctx context.Context, userID, id string) error
	// ApplicableTexts returns the texts that apply to one post, in injection order: the
	// account's global guidelines plus those linked to templateID. An empty templateID is a
	// post with no template and matches no link, so it yields the global group alone.
	ApplicableTexts(ctx context.Context, userID, templateID string) ([]string, error)
}

// TemplateDirectory is the template context's published directory, consumed here for two
// things and nothing else: proving a scoped id is owned by the account at write time, and
// projecting names on read. It is a port rather than a join because templates are another
// context's table (ARCHITECTURE §2.2).
type TemplateDirectory interface {
	Templates(ctx context.Context, userID string) ([]TemplateRef, error)
}
