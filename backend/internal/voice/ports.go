package voice

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Store is persistence behavior owned by the voice context. SQL rows stop in store/.
type Store interface {
	GetProfile(ctx context.Context, userID string) (Profile, error)
	UpsertProfile(ctx context.Context, profile Profile) error
	SetStyleguideIfCorpusVersion(ctx context.Context, userID, styleguide string, version int64, now time.Time) (bool, error)
	SetStyleguide(ctx context.Context, userID, styleguide string, now time.Time) error
	SetRules(ctx context.Context, userID, rules string, now time.Time) error
	InsertSample(ctx context.Context, sample Sample) error
	ListSamples(ctx context.Context, userID string) ([]Sample, error)
	ListSampleBodies(ctx context.Context, userID string) ([]Sample, error)
	GetSampleBody(ctx context.Context, userID, sampleID string) (*Sample, error)
	CorpusSnapshot(ctx context.Context, userID string) ([]Sample, int64, error)
	DeleteSample(ctx context.Context, userID, sampleID string, now time.Time) (bool, error)
	CountSamples(ctx context.Context, userID string) (int, error)
}

// PersonalizationStore owns the new versioned learning aggregates. Keeping this port
// focused preserves the existing sample-analysis store contract and its test doubles.
type PersonalizationStore interface {
	ListProfileVersions(ctx context.Context, userID string) ([]ProfileVersion, error)
	GetProfileVersion(ctx context.Context, userID string, version int64) (ProfileVersion, error)
	PublishProfileVersion(ctx context.Context, userID string, profile StructuredProfile, origin string, restoredFrom int64, now time.Time) (ProfileVersion, error)
	PublishProfileVersionIfHead(ctx context.Context, userID string, profile StructuredProfile, origin string, expectedHead int64, now time.Time) (ProfileVersion, bool, error)
	ListManualOverrides(ctx context.Context, userID string) ([]ManualOverride, error)
	SetManualOverride(ctx context.Context, value ManualOverride) error
	DeleteManualOverride(ctx context.Context, userID string, layer RuleLayer, field string) (bool, error)
	ApplyOverrideAndPublish(ctx context.Context, override ManualOverride, value *string, profile StructuredProfile, now time.Time) error
	InsertLearningEvent(ctx context.Context, event LearningEvent) error
	FindLearningEvent(ctx context.Context, userID, postSlug string, baselineRevision int64, inputHash string) (*LearningEvent, error)
	GetLearningEvent(ctx context.Context, userID, eventID string) (*LearningEvent, error)
	SetLearningEventJob(ctx context.Context, userID, eventID, jobID string) error
	SetLearningEventStatus(ctx context.Context, userID, eventID, status, message string, processedAt *time.Time) error
	ListAuthoredSources(ctx context.Context, userID string) ([]AuthoredSource, error)
	GetAuthoredSource(ctx context.Context, userID, sourceID string) (AuthoredSource, error)
	ApplyLearningResult(ctx context.Context, event LearningEvent, result LearningResult, cfg PersonalizationConfig, now time.Time) error
	ListRules(ctx context.Context, userID string) ([]ContrastRule, error)
	GetRule(ctx context.Context, userID, ruleID string) (ContrastRule, error)
	SetRuleStatus(ctx context.Context, userID, ruleID string, status RuleStatus, now time.Time) error
	RetireStaleRules(ctx context.Context, userID string, before time.Time) (int, error)
	ApplyRuleStatusAndPublish(ctx context.Context, userID, ruleID string, status RuleStatus, profile StructuredProfile, now time.Time) error
	RetireStaleRulesAndPublish(ctx context.Context, userID string, before, now time.Time) (int, error)
	InsertFeedback(ctx context.Context, feedback Feedback) error
	ListFeedback(ctx context.Context, userID string) ([]Feedback, error)
	ListConfirmations(ctx context.Context, userID string) ([]RuleConfirmation, error)
	ResolveConfirmation(ctx context.Context, userID, confirmationID string, replace bool, now time.Time) error
	ResolveConfirmationAndPublish(ctx context.Context, userID, confirmationID string, replace bool, now time.Time) error
	InsertRuleComparison(ctx context.Context, comparison RuleComparison) error
	SetRuleComparisonJob(ctx context.Context, userID, comparisonID, jobID string) error
	GetRuleComparison(ctx context.Context, userID, comparisonID string) (RuleComparison, error)
	UpdateRuleComparison(ctx context.Context, comparison RuleComparison) error
	InsertProfileValidation(ctx context.Context, validation ProfileValidation) error
	SetProfileValidationJob(ctx context.Context, userID, validationID, jobID string) error
	GetProfileValidation(ctx context.Context, userID, validationID string) (ProfileValidation, error)
	ListProfileValidations(ctx context.Context, userID string) ([]ProfileValidation, error)
	UpdateProfileValidation(ctx context.Context, validation ProfileValidation) error
}

type Posts interface {
	LearningSnapshot(ctx context.Context, userID, slug string) (FinalizationInput, error)
}

// Models resolves the acting user's current analyze selection and performs calls
// through the provider-neutral llm boundary.
type Models interface {
	AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
}
type PersonalizationModels interface{ ModelEnabled(ref llm.ModelRef) bool }

// Jobs is the shared queue behavior this context consumes. Its types are defined here;
// the composition root adapts the job context without coupling sibling domains.
type Jobs interface {
	Enqueue(ctx context.Context, request AnalysisJobRequest) (string, error)
	ActiveForUserKind(ctx context.Context, userID, kind string) (*ActiveJob, error)
}
type PersonalizationJobs interface {
	EnqueuePersonalization(ctx context.Context, request PersonalizationJobRequest) (string, error)
	IsPersonalizationJobActive(ctx context.Context, jobID, userID string) (bool, error)
	FailQueuedPersonalization(ctx context.Context, jobID, userID, message string) (bool, error)
}

type Progress func(stage string, done, total int)
