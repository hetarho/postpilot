package voice

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Store is persistence behavior owned by the voice context. SQL rows stop in store/. Every
// profile query names the voice AND the account: the voice partitions the aggregate, the
// account keeps a crafted same-shape id from another user out.
type Store interface {
	// InsertVoice writes the directory row and the voice's empty profile row together, so a
	// read never has to create a profile.
	InsertVoice(ctx context.Context, v Voice) error
	ListVoices(ctx context.Context, userID string) ([]Voice, error)
	GetVoice(ctx context.Context, userID, voiceID string) (Voice, error)
	DefaultVoice(ctx context.Context, userID string) (Voice, bool, error)
	CountActiveVoices(ctx context.Context, userID string) (int, error)
	RenameVoice(ctx context.Context, userID, voiceID, name string, now time.Time) error
	SetDefaultVoice(ctx context.Context, userID, voiceID string, now time.Time) error
	SoftDeleteVoice(ctx context.Context, userID, voiceID string, now time.Time) (bool, error)
	RestoreVoice(ctx context.Context, userID, voiceID string, now time.Time) (bool, error)
	// CountUndecidedVoiceWork counts the comparisons and validations this context owns that
	// have not reached a terminal state; jobs and experiments are asked through their ports.
	CountUndecidedVoiceWork(ctx context.Context, voiceID string) (int, error)

	GetProfile(ctx context.Context, userID, voiceID string) (Profile, error)
	// ClaimCorpusVersion is the concurrency guard a finished analysis has to win before it may
	// publish. False means a sample changed while the provider was working, so the analysis
	// describes a corpus the voice has already moved past. It writes no text: change 16
	// removed the styleguide column the guard used to piggyback on.
	ClaimCorpusVersion(ctx context.Context, userID, voiceID string, version int64, now time.Time) (bool, error)
	// SetRules is reachable only from AppendRule, which is the refine step's "save as rule"
	// checkbox. There is no editor and no RPC for this value any more (change 16).
	SetRules(ctx context.Context, userID, voiceID, rules string, now time.Time) error
	InsertSample(ctx context.Context, sample Sample) error
	ListSamples(ctx context.Context, userID, voiceID string) ([]Sample, error)
	ListSampleBodies(ctx context.Context, userID, voiceID string) ([]Sample, error)
	GetSampleBody(ctx context.Context, userID, voiceID, sampleID string) (*Sample, error)
	CorpusSnapshot(ctx context.Context, userID, voiceID string) ([]Sample, int64, error)
	DeleteSample(ctx context.Context, userID, voiceID, sampleID string, now time.Time) (bool, error)
	CountSamples(ctx context.Context, userID, voiceID string) (int, error)

	// The per-version generation snapshot (change 16). `content` is OPAQUE TEXT to this
	// context: voice records what a profile version produced without learning the shape of a
	// post's content. One row per (voice, version) — a later generation replaces it.
	UpsertVersionSample(ctx context.Context, sample VersionSample) error
	GetVersionSample(ctx context.Context, userID, voiceID string, version int64) (VersionSample, error)
}

// PersonalizationStore owns the versioned learning aggregates. Rows that hang off an owned
// parent (a rule, a confirmation, a comparison, a validation) are looked up by id and
// account and carry their voice out, so a caller derives the voice from the aggregate
// instead of nominating one.
type PersonalizationStore interface {
	ListProfileVersions(ctx context.Context, userID, voiceID string) ([]ProfileVersion, error)
	GetProfileVersion(ctx context.Context, userID, voiceID string, version int64) (ProfileVersion, error)
	PublishProfileVersion(ctx context.Context, userID, voiceID string, profile StructuredProfile, origin string, restoredFrom int64, now time.Time) (ProfileVersion, error)
	PublishProfileVersionIfHead(ctx context.Context, userID, voiceID string, profile StructuredProfile, origin string, expectedHead int64, now time.Time) (ProfileVersion, bool, error)
	ListManualOverrides(ctx context.Context, userID, voiceID string) ([]ManualOverride, error)
	SetManualOverride(ctx context.Context, value ManualOverride) error
	DeleteManualOverride(ctx context.Context, userID, voiceID string, layer RuleLayer, field string) (bool, error)
	ApplyOverrideAndPublish(ctx context.Context, override ManualOverride, value *string, profile StructuredProfile, now time.Time) error
	InsertLearningEvent(ctx context.Context, event LearningEvent) error
	FindLearningEvent(ctx context.Context, userID, voiceID, postSlug string, baselineRevision int64, inputHash string) (*LearningEvent, error)
	GetLearningEvent(ctx context.Context, userID, eventID string) (*LearningEvent, error)
	SetLearningEventJob(ctx context.Context, userID, eventID, jobID string) error
	SetLearningEventStatus(ctx context.Context, userID, eventID, status string, failure *Failure, processedAt *time.Time) error
	ListAuthoredSources(ctx context.Context, userID, voiceID string) ([]AuthoredSource, error)
	GetAuthoredSource(ctx context.Context, userID, voiceID, sourceID string) (AuthoredSource, error)
	ApplyLearningResult(ctx context.Context, event LearningEvent, result LearningResult, cfg PersonalizationConfig, now time.Time) error
	ListRules(ctx context.Context, userID, voiceID string) ([]ContrastRule, error)
	GetRule(ctx context.Context, userID, ruleID string) (ContrastRule, error)
	SetRuleStatus(ctx context.Context, userID, voiceID, ruleID string, status RuleStatus, now time.Time) error
	RetireStaleRules(ctx context.Context, userID, voiceID string, before time.Time) (int, error)
	ApplyRuleStatusAndPublish(ctx context.Context, userID, voiceID, ruleID string, status RuleStatus, profile StructuredProfile, now time.Time) error
	RetireStaleRulesAndPublish(ctx context.Context, userID, voiceID string, before, now time.Time) (int, error)
	InsertFeedback(ctx context.Context, feedback Feedback) error
	ListFeedback(ctx context.Context, userID, voiceID string) ([]Feedback, error)
	ListConfirmations(ctx context.Context, userID, voiceID string) ([]RuleConfirmation, error)
	GetConfirmation(ctx context.Context, userID, confirmationID string) (RuleConfirmation, error)
	ResolveConfirmation(ctx context.Context, userID, confirmationID string, replace bool, now time.Time) error
	ResolveConfirmationAndPublish(ctx context.Context, userID, confirmationID string, replace bool, now time.Time) error
	InsertRuleComparison(ctx context.Context, comparison RuleComparison) error
	SetRuleComparisonJob(ctx context.Context, userID, comparisonID, jobID string) error
	GetRuleComparison(ctx context.Context, userID, comparisonID string) (RuleComparison, error)
	UpdateRuleComparison(ctx context.Context, comparison RuleComparison) error
	InsertProfileValidation(ctx context.Context, validation ProfileValidation) error
	SetProfileValidationJob(ctx context.Context, userID, validationID, jobID string) error
	GetProfileValidation(ctx context.Context, userID, validationID string) (ProfileValidation, error)
	ListProfileValidations(ctx context.Context, userID, voiceID string) ([]ProfileValidation, error)
	UpdateProfileValidation(ctx context.Context, validation ProfileValidation) error
}

// Posts is the post context's published finalization hand-off; it never exposes post rows.
type Posts interface {
	LearningSnapshot(ctx context.Context, userID, slug string) (FinalizationInput, error)
}

// Models resolves the acting user's current analyze selection and performs calls
// through the provider-neutral llm boundary. Model selection stays account-scoped: two
// voices share the account's analyze model, never its profile.
type Models interface {
	AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error)
	Resolve(ref llm.ModelRef) (llm.ModelInfo, bool)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
}

// PersonalizationModels answers whether a client-supplied ref may run for a stage — the
// same per-purpose membership the pickers enforce (change 20).
type PersonalizationModels interface {
	ModelEnabled(ref llm.ModelRef, stage string) bool
}

// Jobs is the shared queue behavior this context consumes. Its types are defined here;
// the composition root adapts the job context without coupling sibling domains. Voice-owned
// work is guarded per voice so two voices may analyze at once.
type Jobs interface {
	Enqueue(ctx context.Context, request AnalysisJobRequest) (string, error)
	ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*ActiveJob, error)
	// HasActiveForVoice reports any queued/running job frozen to the voice, whatever its kind
	// or post — the whole set a soft delete must wait for.
	HasActiveForVoice(ctx context.Context, voiceID string) (bool, error)
}
type PersonalizationJobs interface {
	EnqueuePersonalization(ctx context.Context, request PersonalizationJobRequest) (string, error)
	IsPersonalizationJobActive(ctx context.Context, jobID, userID string) (bool, error)
	FailQueuedPersonalization(ctx context.Context, jobID, userID string, failure Failure) (bool, error)
}

// Experiments is the model-experiment context's published guard, consumed only by
// DeleteVoice: an experiment frozen to the voice that could still publish into it (a
// styleguide, a machine baseline) keeps the voice alive until it is decided and applied.
type Experiments interface {
	HasPublishableExperimentForVoice(ctx context.Context, userID, voiceID string) (bool, error)
}

type Progress func(stage string, done, total int)
