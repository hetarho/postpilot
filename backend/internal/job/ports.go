package job

import (
	"context"
	"time"
)

// Admitter authorizes one job start and records it. Declared here because Enqueue is the
// single seam every LLM-consuming kind passes through — gating it anywhere else would
// mean seven gates that must be kept in step.
//
// The queue stays plan-ignorant: it hands over the refs the job will run as strings and
// lets the implementation (internal/usage, wired in cmd/api) decide. Admit must write
// nothing when it refuses, and Release undoes an admission whose job row then failed to
// insert.
type Admitter interface {
	Hold(ctx context.Context, start Start) error
	Release(ctx context.Context, jobID string)
	// Settle reconciles a finished job's hold against what it actually spent. It takes no
	// error: a hold that cannot be settled must not turn a finished job into a failed one,
	// so the adapter logs and the boot sweep retries.
	Settle(ctx context.Context, jobID string)
	// OpenHolds lists jobs whose hold has not been reconciled yet. The queue matches them
	// against its own rows, so the ledger never has to read the job table.
	OpenHolds(ctx context.Context) ([]string, error)
}

// Start is what the gate needs to know about a job about to be created.
type Start struct {
	UserID string
	Kind   string
	JobID  string
	Calls  []PlannedCall
}

// PlannedCall is one model the job will run, and how many times. The count is the queue's
// to state because only the enqueuing service knows it: photo observation batches four
// photos per call, and a validation repeats its stages per sampled post.
type PlannedCall struct {
	Ref   string
	Count int
}

// Store is the persistence behavior the queue needs. SQL rows stop in store/.
type Store interface {
	Insert(ctx context.Context, found Job) error
	PickNextQueued(ctx context.Context, now time.Time) (Job, error)
	UpdateProgress(ctx context.Context, id, stage string, done, total int, now time.Time) error
	Finish(ctx context.Context, id, status string, failure *Failure, now time.Time) error
	FailQueued(ctx context.Context, id, userID string, failure Failure, now time.Time) (bool, error)
	SweepRunning(ctx context.Context, failure Failure, now time.Time) (int64, error)
	SweepQueuedPersonalization(ctx context.Context, failure Failure, now time.Time) (int64, error)
	ActiveForPost(ctx context.Context, slug string) (*Job, error)
	ActiveForPostUser(ctx context.Context, slug, userID string) (*Job, error)
	ActiveForUserKind(ctx context.Context, userID, kind string) (*Job, error)
	ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*Job, error)
	ActiveForVoice(ctx context.Context, voiceID string) (*Job, error)
	ActiveModelExperiment(ctx context.Context, experimentID string) (*Job, error)
	GetByID(ctx context.Context, id string) (Job, error)
}
