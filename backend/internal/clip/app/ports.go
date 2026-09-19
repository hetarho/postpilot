// Package app is the clip context's application layer: the use-cases that must
// commit across the clip, job and usage tables in one writer transaction
// (ARCH-6). Every port here is consumer-declared and names only what a saga
// calls, so the stores of the other contexts publish behaviour, never tables
// (ARCH-7).
package app

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

// JobReader is the job context's read side the sagas consult outside a
// transaction, for idempotency and post-failure classification.
type JobReader interface {
	GetByID(context.Context, string) (job.Job, error)
}

// JobTx is the job behaviour a saga needs inside the writer transaction.
type JobTx interface {
	JobReader
	Finish(ctx context.Context, id, status string, failure *job.Failure, now time.Time) error
	ActiveForClip(ctx context.Context, user, id string) (*job.Job, error)
}

// ClipStore is the clip persistence the sagas use outside a transaction: the
// staging row that survives a crash and the reads that decide idempotency.
type ClipStore interface {
	GetProject(ctx context.Context, user, id string) (clip.Project, error)
	StageAttemptResult(context.Context, clip.AttemptResult) error
	GetAttemptResult(context.Context, string) (clip.AttemptResult, error)
	DiscardAttemptResult(context.Context, string) error
	PendingAttemptResults(context.Context) ([]clip.AttemptResult, error)
}

// ClipTx is the clip persistence a saga touches inside the writer transaction.
type ClipTx interface {
	GetProject(ctx context.Context, user, id string) (clip.Project, error)
	GetAttemptResult(context.Context, string) (clip.AttemptResult, error)
	ApplyAttemptResult(context.Context, clip.AttemptResult) error
	DeleteAttemptResult(context.Context, string) error
	RecordFinalization(context.Context, clip.FinalizationRequest, time.Time) (clip.Project, error)
}

// Admission is the credit hold, already bound to the transaction it must
// commit with. The composition root builds it from the ledger's tx-scoped store.
type Admission interface {
	Hold(context.Context, job.Start) error
}

// Ports is what one writer transaction exposes to a saga.
type Ports struct {
	Jobs      JobTx
	Clips     ClipTx
	Admission Admission
}

// Binder turns the open transaction into the tx-scoped ports. It is the one
// place that knows which store constructors wrap a *sql.Tx, and it lives at the
// composition root.
type Binder func(*sql.Tx) Ports

// Authorizer serializes dispatch authorization with cancellation through the
// job store's conditional writer statement.
type Authorizer interface {
	AuthorizeClipDispatch(ctx context.Context, user, id string) error
}

// Queue is the job queue surface the clip jobs adapter drives.
type Queue interface {
	Enqueue(context.Context, job.NewJob) (string, error)
	ActivateClip(ctx context.Context, user, id string) error
	FailQueued(ctx context.Context, id, user string, failure job.Failure) (bool, error)
	ActiveForClip(ctx context.Context, user, id string) (*job.JobSummary, error)
	Get(ctx context.Context, id, user string) (*job.JobSummary, error)
	ClipJobSnapshot(ctx context.Context, user, project, id string) (*job.Job, error)
	LatestClipSnapshot(ctx context.Context, user, id string) (*job.Job, error)
	ReserveClip(ctx context.Context, user, id string, calls []job.PlannedCall, approval ...job.ClipReservation) (context.Context, error)
}

// Freezer is the model registry as the quote and the admission see it: frozen
// execution policies and the catalogue, nothing that calls a model.
type Freezer interface {
	FreezeExecution(ctx context.Context, ref llm.ModelRef, stage string, budget int, reasoning llm.ReasoningEffort, delivery llm.ExecutionDelivery) (llm.CallPolicy, error)
	Models() []llm.ModelInfo
}

// AccountingLedger is the ledger's per-job clip settlement view.
type AccountingLedger interface {
	ClipAccounting(ctx context.Context, user, job string) (*usage.ClipAccounting, error)
}
