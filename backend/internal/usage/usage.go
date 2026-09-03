// Package usage is the account ledger and the credit gate: what an account has spent on
// model calls, what its balance is, and whether it may start one more piece of LLM work.
//
// It owns four tables for three different questions. credit_lots holds the balance, one
// row per grant, consumed in expiry order. usage_admissions answers "what was held for
// this job" and is written once per admitted start, with credit_hold_lots recording which
// lots that hold came from so settlement can return the remainder to the same places.
// usage_events answers "how much money" and is written once per provider call, including
// a failed call whose usage the provider reported — those tokens were bought.
package usage

import (
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// PlannedCall is one model the work will run, and how many times.
//
// The count matters because the hold prices every call the job will make: photo
// observation batches four photos per call, an A/B comparison runs each candidate, and a
// profile validation repeats its stages per sampled post. A caller that undercounts does
// not break the balance floor — the lots refuse to go negative — but it does let work
// start on credits the account turns out not to have, which the account never pays back.
type PlannedCall struct {
	Ref   llm.ModelRef
	Count int
}

// Start is one request to begin LLM work.
type Start struct {
	UserID string
	Plan   plan.Plan
	Kind   string
	JobID  string
	Calls  []PlannedCall
}

// Admission is the durable record of an admitted start and the credits held for it.
type Admission struct {
	UserID      string
	Kind        string
	JobID       string
	HoldCredits int
	CreatedAt   time.Time
}

// LotDebit is how much of one hold came out of one lot. Settlement refunds against these
// rather than against the consumption order: by the time a job ends a lot may have
// expired or a new one opened, and refunding into the wrong lot would move credits
// between expiry dates.
type LotDebit struct {
	LotID   string
	Credits int
}

// LotKind separates the monthly grant from a bonus. It is not an authorization
// distinction — consumption orders by expiry, not by kind — but a display one, and the
// thing that tells a renewal which lot it replaces.
type LotKind string

const (
	LotMonthly LotKind = "monthly"
	LotBonus   LotKind = "bonus"
)

// Lot is one grant of credits.
type Lot struct {
	ID        string
	UserID    string
	Kind      LotKind
	Granted   int
	Remaining int
	// ExpiresAt is nil for a grant that does not expire. Consumption puts those last, so a
	// non-expiring bonus is always spent after the monthly grant that would otherwise be
	// lost.
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// Expired reports whether this lot no longer contributes to a balance at the given
// instant. Expiry is read-time rather than a scheduled sweep, so a balance is correct
// even if nothing ran at the boundary.
func (l Lot) Expired(at time.Time) bool {
	return l.ExpiresAt != nil && !at.Before(*l.ExpiresAt)
}

// Call is one completed provider call, as the context that made it saw it. The cost is
// not here: the ledger resolves it from the registry's prices so every row is priced by
// the same rule, whatever the caller happened to know.
type Call struct {
	UserID string
	Kind   string
	JobID  string
	Stage  string
	Model  llm.ModelRef
	Usage  llm.Usage
}

// Event is one ledger row.
type Event struct {
	UserID           string
	Kind             string
	JobID            string
	Stage            string
	Model            string
	PromptTokens     int64
	CompletionTokens int64
	// ReasoningTokens is the part of CompletionTokens the provider attributed to reasoning,
	// or 0 when it reported none. It is recorded for DIAGNOSIS — a model spending its whole
	// budget thinking and writing nothing was previously indistinguishable from one writing
	// a long post — and is deliberately not re-priced.
	ReasoningTokens int64
	CostMicrousd    int64
	CostSource      llm.CostSource
	CreatedAt       time.Time
}

// ReasoningSpendWindow is how far back the reasoning-spend signal looks. Long enough that a
// model curated last week still has evidence, short enough that a fixed effort shows its
// effect rather than being averaged away by the weeks before it.
const ReasoningSpendWindow = 14 * 24 * time.Hour

// ReasoningSpend is one model's recent completion-budget split at one stage, over the
// window the store defines. It is the published aggregate the curation surface reads.
type ReasoningSpend struct {
	Model            string
	Stage            string
	Calls            int64
	ReasoningTokens  int64
	CompletionTokens int64
}

// Balance is the account's spendable position. GetMyPlan returns it verbatim, so a client
// can explain a refusal without re-deriving a total it never summed.
type Balance struct {
	// Credits is the sum of Remaining over the unexpired lots. It is zero for an
	// unlimited account, which reads Unlimited instead.
	Credits   int
	Unlimited bool
	Lots      []Lot
	// RenewsAt is when the next monthly grant opens. It is the instant every refusal
	// names, so a user is never told "later" without being told when.
	RenewsAt time.Time
}
