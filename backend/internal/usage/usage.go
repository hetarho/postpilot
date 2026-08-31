// Package usage is the account ledger and the admission gate: what an account has spent
// on model calls, and whether it may start one more piece of LLM work.
//
// It owns two tables for two different questions. usage_admissions answers "how many
// starts today" and is written once per admitted job, so a refusal costs nothing and an
// A/B comparison that fans out to two candidate calls still costs one. usage_events
// answers "how much money" and is written once per provider call, including a failed call
// whose usage the provider reported — those tokens were bought.
package usage

import (
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// Start is one request to begin LLM work. Models carries every ref the job will run, with
// the floor each one declares, so the gate can refuse an unaffordable model before the
// account's own numeric axes are even consulted.
type Start struct {
	UserID string
	Plan   plan.Plan
	Kind   string
	JobID  string
	Models []plan.ModelFloor
}

// Admission is the durable record of an admitted start.
type Admission struct {
	UserID    string
	Kind      string
	JobID     string
	CreatedAt time.Time
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
	CostMicrousd     int64
	CostSource       llm.CostSource
	CreatedAt        time.Time
}

// Summary is the account's live position in both windows, with the instant each one
// resets. GetMyPlan returns it verbatim, so a client can explain a refusal without
// re-deriving a window it never measured.
type Summary struct {
	JobsStartedToday  int
	CostTodayMicrousd int64
	CostMonthMicrousd int64
	DayResetsAt       time.Time
	MonthResetsAt     time.Time
}
