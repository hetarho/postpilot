package generation

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/llm"
)

var (
	ErrWritingTestCheckpointInvalid   = errors.New("writing test checkpoint does not match frozen work")
	ErrWritingTestCheckpointRequired  = errors.New("writing test requires durable execution checkpoints")
	ErrWritingTestPreparationRequired = errors.New("writing test common observations are not prepared")
	ErrWritingTestExecutionUncertain  = errors.New("writing test has an issued unconfirmed call")
	ErrWritingTestRetryRequired       = errors.New("writing test failed work requires an explicit retry plan")
)

// WritingTestCheckpoint is private execution state, separate from immutable snapshot identity.
// Index -1 belongs to shared preparation; all other indices belong to exactly one entrant.
// Persisting InFlightStage before issuing work fences interrupted/uncertain calls from replay.
type WritingTestCheckpoint struct {
	SnapshotHash                                                                                      string
	Index                                                                                             int
	ObserveModel, WriteModel                                                                          llm.ModelRef
	BatchSize, ObservePromptTokens, ObserveCompletionTokens, WritePromptTokens, WriteCompletionTokens int
	Reasoning                                                                                         ReasoningPolicy
	ObserveStructuredOutput, WriteStructuredOutput                                                    bool
	Observations                                                                                      []Observation
	CompletedObserveCalls                                                                             int
	Prepared                                                                                          bool
	InFlightStage, FailedStage                                                                        string
	Answer                                                                                            *WriteAnswer
}

// SaveWritingTestCheckpoint must commit the test's ownership/revision/purge/cancellation fence
// together with this checkpoint. A rejected callback stops execution before another call.
type SaveWritingTestCheckpoint func(context.Context, WritingTestCheckpoint) error

type WritingTestRunOptions struct {
	Checkpoint *WritingTestCheckpoint
	// RetryFailed is supplied only after the caller explicitly admits the remaining failed plan.
	RetryFailed    bool
	Progress       Progress
	SaveCheckpoint SaveWritingTestCheckpoint
}

type WritingTestRunResult struct {
	Checkpoint      WritingTestCheckpoint
	Answer          WriteAnswer
	ContentLanguage Language
	// Usage contains only work issued by this invocation, never a replay's original accounting.
	Usage    CandidateUsage
	Replayed bool
}
