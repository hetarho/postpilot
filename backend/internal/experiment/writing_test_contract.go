package experiment

import (
	"context"
	"time"
)

// TestQuote is server-owned private preparation, not a client-supplied cost ceiling.
type TestQuote struct {
	Key, UserID, Fingerprint, TestID string
	Credits                          int
	Free                             bool
	Start                            TestStart
	Retry                            TestRetryQuoteRequest
	Plan                             TestPlan
	CreatedAt, ExpiresAt             time.Time
	ConsumedRequestKey               string
	Retention                        time.Duration
}
type TestRetry struct {
	TestMutation
	CandidateIDs []string
	QuoteKey     string
}
type WritingTestConfig struct{ QuoteTTL, Retention time.Duration }
type TestCost struct {
	Credits int
	Free    bool
}
type WritingTestPricing interface {
	PriceWritingTest(context.Context, string, []TestCall) (TestCost, error)
}

// Revision is the admission's immutable execution epoch, not the changing public
// bracket revision. Current attempt/job/purge/cancellation checks fence every write.
type TestExecutionFence struct {
	NonMetered                        bool
	UserID, TestID, JobID, RequestKey string
	Revision                          uint32
	PurgeFence                        uint64
}
type TestExecutionWork struct {
	Fence                TestExecutionFence
	Test                 WritingTest
	Plan                 TestPlan
	CandidateIDs         []string
	CandidateIndices     map[string]int
	SharedCheckpoint     []byte
	CandidateCheckpoints map[string][]byte
}
type WritingTestRuntimeStore interface {
	BeginTestExecution(context.Context, TestExecutionFence) (TestExecutionWork, error)
	SaveTestCheckpoint(context.Context, TestExecutionFence, string, []byte) error
	CompleteTestCandidate(context.Context, TestExecutionFence, string, []byte, []byte, *Failure) error
	FinishTestExecution(context.Context, TestExecutionFence, int, *Failure) (WritingTest, error)
	ConfirmTestSettlement(context.Context, TestExecutionFence, int) error
	RecoverInterruptedTests(context.Context) error
}

// The queue bridge persists a deferred job, binds its owner attempt, and activates
// only after exact admission. Recovery looks up the same stable request identity.
type WritingTestQueue interface {
	StartWritingTestJob(context.Context, TestExecutionWork) (string, error)
	ActivateWritingTestJob(context.Context, string, string) error
	CancelWritingTestJob(context.Context, string, string) error
}
type WritingTestStorage interface {
	PutTestQuote(context.Context, TestQuote) error
	GetTestQuote(context.Context, string, string) (TestQuote, error)
	TestByRequest(context.Context, string, string) (WritingTest, error)
	AdmitTest(context.Context, TestStart, TestPlan) (WritingTest, bool, error)
	GetTest(context.Context, string, string) (WritingTest, error)
	ListTests(context.Context, string, int, string) ([]WritingTest, string, error)
	DecideMatch(context.Context, MatchDecision) (WritingTest, error)
	CancelTest(context.Context, TestMutation) (WritingTest, error)
	ReserveTestRetry(context.Context, TestRetry, TestPlan) (WritingTest, bool, error)
	PreparedTestWork(context.Context, string, string) (TestExecutionWork, error)
	BindTestJob(context.Context, TestExecutionFence, string) (WritingTest, error)
	RejectTestAdmission(context.Context, TestExecutionFence, *Failure) error
	PurgeWritingTestPost(context.Context, string, string) error
	PurgeExpiredWritingTests(context.Context, time.Time) (int64, error)
}
type WritingTestDependencies struct {
	Store             WritingTestStorage
	Variants          TestVariants
	Preparation       TestPreparation
	FailedPreparation FailedTestPreparation
	Pricing           WritingTestPricing
	Queue             WritingTestQueue
}

// Terminal reconciliation consumes only authoritative generic job facts. It closes
// the execution epoch before settlement so late provider callbacks cannot restore it.
type TestExecutionOutcome string

const (
	TestExecutionSucceeded TestExecutionOutcome = "succeeded"
	TestExecutionFailed    TestExecutionOutcome = "failed"
	TestExecutionCancelled TestExecutionOutcome = "cancelled"
)

type WritingTestExecutionTerminalStore interface {
	EndTestExecution(context.Context, TestExecutionFence, TestExecutionOutcome, *Failure) error
}
type WritingTestSettlementStore interface {
	PendingTestSettlements(context.Context) ([]TestExecutionFence, error)
	ConfirmTestSettlement(context.Context, TestExecutionFence, int) error
}
