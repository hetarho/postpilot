package usage

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2).
type Store interface {
	// InWriteTx runs fn inside ONE write transaction, handing it a store scoped to that
	// transaction. The hold needs it: reading a balance and spending it are one decision,
	// and two concurrent starts that each read the same balance would otherwise both be
	// admitted past it.
	InWriteTx(ctx context.Context, fn func(Store) error) error

	// LotsInConsumptionOrder returns the account's unexpired lots, ordered by expiry
	// ascending with the non-expiring ones last. That single ordering is what makes the
	// monthly grant spend before a bonus without a second rule.
	LotsInConsumptionOrder(ctx context.Context, userID string, now time.Time) ([]Lot, error)
	// ActiveMonthlyLot is the account's current monthly grant, if it has one that has not
	// expired. Its expiry IS the account's renewal instant — a separate stored column
	// would be a second place for one fact to live.
	ActiveMonthlyLot(ctx context.Context, userID string, now time.Time) (Lot, bool, error)
	InsertLot(ctx context.Context, lot Lot) error
	// InsertLotIfAbsent is the same write for a grant whose id identifies what it is for
	// rather than being random, so re-running the thing that grants it is harmless.
	InsertLotIfAbsent(ctx context.Context, lot Lot) error
	// SpendFromLot and RefundToLot move credits within one lot. Both are guarded in SQL by
	// the amount available, so a concurrent write cannot drive a lot past its own bounds
	// even if a caller's arithmetic is stale.
	SpendFromLot(ctx context.Context, lotID string, credits int) error
	RefundToLot(ctx context.Context, lotID string, credits int) error

	InsertAdmission(ctx context.Context, admission Admission) error
	InsertHoldDebits(ctx context.Context, jobID string, debits []LotDebit) error
	// HoldForJob returns the admission and the lots its hold came from. Missing means the
	// job was never admitted through this gate.
	HoldForJob(ctx context.Context, jobID string) (Admission, []LotDebit, bool, error)
	MarkSettled(ctx context.Context, jobID string, credits int, at time.Time) error
	// UnsettledHoldJobs lists job ids whose hold is still open. The boot sweep asks the
	// job context which of them are terminal — this context never reads another's tables.
	UnsettledHoldJobs(ctx context.Context) ([]string, error)
	// DeleteAdmissionForJob undoes an admission whose job row was never created. It is the
	// one deletion this context performs: the ledger proper is append-only, but a start
	// that never happened must not stay charged.
	DeleteAdmissionForJob(ctx context.Context, jobID string) error

	InsertEvent(ctx context.Context, event Event) error
	// ReasoningSpend aggregates recorded calls at one stage since `since`, per model.
	ReasoningSpend(ctx context.Context, stage string, since time.Time) ([]ReasoningSpend, error)
	// SumCostForJob is what the job actually spent, the figure settlement charges.
	SumCostForJob(ctx context.Context, jobID string) (int64, error)
}

// Models resolves a ref's registry metadata. The hold needs its prices to estimate a
// worst case, and the ledger needs them to price a call the calling context only knows
// token counts for; nothing here learns which provider ran it.
type Models interface {
	Lookup(ref llm.ModelRef) (llm.ModelInfo, bool)
}
