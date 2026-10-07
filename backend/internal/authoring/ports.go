package authoring

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// Targets keeps canonical validation and publication in the target's owning domain.
// Seed reads only the explicitly requested owned setting. Publish is idempotent by Key.
type Targets interface {
	Seed(context.Context, string, Kind, string) (Seed, error)
	CanStart(context.Context, string, Kind, string) error
	Validate(Kind, Artifact) error
	Guide(Kind) string
	Publish(context.Context, Publication) (SavedRef, error)
}
type Models interface {
	Resolve(llm.ModelRef) (llm.ModelInfo, bool)
	Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error)
}
type JobRequest struct {
	UserID, WriteModel             string
	Payload                        []byte
	CompletionTokens, PromptTokens int
}
type Job struct {
	ID, Status, WriteModel, FailureReason string
	Payload                               []byte
}
type Jobs interface {
	Enqueue(context.Context, JobRequest) (string, error)
	Get(context.Context, string, string) (Job, error)
	Latest(context.Context, string) (*Job, error)
	SaveResult(context.Context, string, []byte) error
	Cancel(context.Context, string, string) error
}
type Budget interface {
	CompletionCap(Kind, Mode, int, bool) int
}

type Store interface {
	Create(context.Context, Session, string) (Session, error)
	Created(context.Context, string, string) (*Session, error)
	Get(context.Context, string, string) (Session, error)
	Latest(context.Context, string, Kind, string) (*Session, error)
	Operation(context.Context, string, string, string) (Operation, error)
	ActiveOperation(context.Context, string, string) (Operation, error)
	Reserve(context.Context, string, string, uint32, Operation) (Session, Operation, bool, error)
	Bind(context.Context, string, string, string) (Session, error)
	Reject(context.Context, string, string, string) (Session, error)
	Reconcile(context.Context, string, string, string, string, *OperationResult) (Session, error)
	Select(context.Context, string, string, uint32, string) (Session, error)
	PrepareSave(context.Context, string, string, uint32, bool) (Session, Publication, error)
	FinalizeSave(context.Context, string, string, string, SavedRef) (Session, error)
}
type Estimator interface {
	CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool)
}

// WorkingDrafts is separate from the baseline Store until the durable implementation lands.
// Every method atomically records its owner-scoped operation receipt with the revision CAS.
type WorkingDrafts interface {
	PatchDraft(context.Context, DraftMutation) (Session, error)
	ResetChat(context.Context, ResetMutation) (Session, error)
	ResetBaseline(context.Context, ResetMutation) (Session, error)
	ListSummaries(context.Context, SummaryQuery) ([]Summary, string, error)
}
type TestCandidates interface {
	FreezeCandidate(context.Context, string, OwnedCandidateRef) (FrozenCandidate, error)
}
