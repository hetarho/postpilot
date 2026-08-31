package usage

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). Windows are half-open [from, to) so a boundary instant belongs to
// exactly one day and one month.
type Store interface {
	// InWriteTx runs fn inside ONE write transaction, handing it a store scoped to that
	// transaction. Admission needs it: counting a window and then inserting into it are one
	// decision, and two concurrent enqueues that each read the same count would otherwise both
	// be admitted past the limit.
	InWriteTx(ctx context.Context, fn func(Store) error) error

	CountAdmissions(ctx context.Context, userID string, from, to time.Time) (int64, error)
	InsertAdmission(ctx context.Context, admission Admission) error
	// DeleteAdmissionForJob undoes an admission whose job row was never created. It is
	// the one deletion this context performs: the ledger proper is append-only, but an
	// admission that never became a job start would otherwise charge the account a start
	// it did not get.
	DeleteAdmissionForJob(ctx context.Context, jobID string) error

	SumCost(ctx context.Context, userID string, from, to time.Time) (int64, error)
	InsertEvent(ctx context.Context, event Event) error
}

// Models resolves a ref's registry metadata. The ledger needs it to price a call the
// calling context only knows token counts for; nothing here learns which provider ran it.
type Models interface {
	Lookup(ref llm.ModelRef) (llm.ModelInfo, bool)
}
