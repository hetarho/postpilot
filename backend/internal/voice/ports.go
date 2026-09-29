package voice

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// JobSubject is what the voice context calls itself when it addresses the job queue.
const JobSubject = "voice"

// Store is persistence behavior owned by the voice context. SQL rows stop in store/. Every
// profile query names the voice AND the account: the voice partitions the aggregate, the
// account keeps a crafted same-shape id from another user out.
// VoiceDirectoryStore is the account's voices as a directory: minting, listing, naming,
// defaulting, retiring and restoring one.
type VoiceDirectoryStore interface {
	// InsertVoice writes the directory row and the voice's empty profile row together, so a
	// read never has to create a profile.
	InsertVoice(ctx context.Context, v Voice) error
	ListVoices(ctx context.Context, userID string) ([]Voice, error)
	GetVoice(ctx context.Context, userID, voiceID string) (Voice, error)
	RenameVoice(ctx context.Context, userID, voiceID, name string, now time.Time) error
	SetDefaultVoice(ctx context.Context, userID, voiceID string, now time.Time) error
	// ClearDefaultVoice leaves the account with no 기본 (VOICE-2).
	ClearDefaultVoice(ctx context.Context, userID string, now time.Time) error
	SoftDeleteVoice(ctx context.Context, userID, voiceID string, now time.Time) (bool, error)
	RestoreVoice(ctx context.Context, userID, voiceID string, now time.Time) (bool, error)
}

// ProfileStore is the voice's current profile text and the guard an analysis has to win
// before it may publish one.
type ProfileStore interface {
	GetProfile(ctx context.Context, userID, voiceID string) (Profile, error)
	// ClaimCorpusVersion is the concurrency guard a finished analysis has to win before it may
	// publish. False means a sample changed while the provider was working, so the analysis
	// describes a corpus the voice has already moved past. It writes no text (VOICE-22): the
	// styleguide column the guard used to piggyback on is gone.
	ClaimCorpusVersion(ctx context.Context, userID, voiceID string, version int64, now time.Time) (bool, error)
}

// SampleStore is the corpus a voice is learned from.
type SampleStore interface {
	InsertSample(ctx context.Context, sample Sample) error
	ListSamples(ctx context.Context, userID, voiceID string) ([]Sample, error)
	ListSampleBodies(ctx context.Context, userID, voiceID string) ([]Sample, error)
	GetSampleBody(ctx context.Context, userID, voiceID, sampleID string) (*Sample, error)
	CorpusSnapshot(ctx context.Context, userID, voiceID string) ([]Sample, int64, error)
	DeleteSample(ctx context.Context, userID, voiceID, sampleID string, now time.Time) (bool, error)
	CountSamples(ctx context.Context, userID, voiceID string) (int, error)
}

// VersionSampleStore is the per-version generation snapshot (VOICE-29): what a profile
// version produced, kept as opaque text.
type VersionSampleStore interface {
	// The per-version generation snapshot (VOICE-29). `content` is OPAQUE TEXT to this
	// context: voice records what a profile version produced without learning the shape of a
	// post's content. One row per (voice, version) — a later generation replaces it.
	UpsertVersionSample(ctx context.Context, sample VersionSample) error
	GetVersionSample(ctx context.Context, userID, voiceID string, version int64) (VersionSample, error)
}

// Storage is every behaviour the voice context's SQL store happens to implement. It is the
// composition root's handle, NOT a port: each use-case below holds only the narrow interfaces
// it calls (ARCH-6).
type Storage interface {
	VoiceDirectoryStore
	ProfileStore
	SampleStore
	VersionSampleStore
}

// ProfileVersionStore is the published history of a voice's structured profile.
type ProfileVersionStore interface {
	ListProfileVersions(ctx context.Context, userID, voiceID string) ([]ProfileVersion, error)
	GetProfileVersion(ctx context.Context, userID, voiceID string, version int64) (ProfileVersion, error)
	PublishProfileVersion(ctx context.Context, userID, voiceID string, profile StructuredProfile, origin string, restoredFrom int64, now time.Time) (ProfileVersion, error)
	PublishProfileVersionIfHead(ctx context.Context, userID, voiceID string, profile StructuredProfile, origin string, expectedHead int64, now time.Time) (ProfileVersion, bool, error)
}

// ManualOverrideStore is what the owner said by hand about their own voice.
type ManualOverrideStore interface {
	ListManualOverrides(ctx context.Context, userID, voiceID string) ([]ManualOverride, error)
	SetManualOverride(ctx context.Context, value ManualOverride) error
	DeleteManualOverride(ctx context.Context, userID, voiceID string, layer RuleLayer, field string) (bool, error)
	ApplyOverrideAndPublish(ctx context.Context, override ManualOverride, value *string, profile StructuredProfile, now time.Time) error
}

// PersonalizationStorage is the versioned profile store as the composition root hands it
// over. Like Storage it is a handle, not a port: the use-cases hold the narrow interfaces
// above.
type PersonalizationStorage interface {
	ProfileVersionStore
	ManualOverrideStore
}

// Models resolves the acting user's current analyze selection and performs calls
// through the provider-neutral llm boundary. Model selection stays account-scoped: two
// voices share the account's analyze model, never its profile.
type Models interface {
	AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error)
	Resolve(ref llm.ModelRef) (llm.ModelInfo, bool)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
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
type Progress func(stage string, done, total int)
