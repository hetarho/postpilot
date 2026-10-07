package voice

import (
	"context"
	"errors"
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
	// InsertVoice writes the directory row; a voice has no analysis until it is made.
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

// AnalysisStore is a voice's current analysis and at most one previous (VOICE-30).
type AnalysisStore interface {
	// CurrentAnalysis is the current analysis, or nil for a voice not yet made.
	CurrentAnalysis(ctx context.Context, userID, voiceID string) (*Analysis, error)
	HasPreviousAnalysis(ctx context.Context, userID, voiceID string) (bool, error)
	// PublishAnalysis moves the current analysis to previous — discarding the one there — and
	// stores the new current, in one transaction (VOICE-25).
	PublishAnalysis(ctx context.Context, userID, voiceID string, analysis Analysis) error
	// RestorePreviousAnalysis makes the previous analysis current and discards the one it
	// replaced; ErrNoPreviousAnalysis without one.
	RestorePreviousAnalysis(ctx context.Context, userID, voiceID string) error
}

// SampleStore is the 학습 글 a voice is made from (VOICE-59).
type SampleStore interface {
	InsertSample(ctx context.Context, sample Sample) error
	ListSampleBodies(ctx context.Context, userID, voiceID string) ([]Sample, error)
	// ListSampleBodiesForVoices is several voices' 학습 글 with their bodies in one read, by voice.
	ListSampleBodiesForVoices(ctx context.Context, userID string, voiceIDs []string) (map[string][]Sample, error)
	GetSampleBody(ctx context.Context, userID, voiceID, sampleID string) (*Sample, error)
	// GetPromptAnswer is the one answer a prompt holds in the voice, or nil.
	GetPromptAnswer(ctx context.Context, userID, voiceID, promptKey string) (*Sample, error)
	// DeleteSample removes the row and reports the photo key it held, so the object can go
	// after it (row first, then object →POST-39).
	DeleteSample(ctx context.Context, userID, voiceID, sampleID string, now time.Time) (photoKey string, deleted bool, err error)
	CountSamples(ctx context.Context, userID, voiceID string) (int, error)
	// AnswerPrompt writes an answer and, for a photo prompt, drops its pending upload in one
	// transaction; a non-empty replaceID is the prompt's previous answer, removed in the same
	// transaction. A prompt that still holds another answer is ErrPromptAnswered.
	AnswerPrompt(ctx context.Context, sample Sample, uploadID, replaceID string) error
}

// PhotoUploadStore is a photo prompt's photo between its presign and its answer.
type PhotoUploadStore interface {
	InsertPhotoUpload(ctx context.Context, upload PhotoUpload) error
	// GetPhotoUpload is the pending upload of this voice and prompt, or ErrPhotoRequired.
	GetPhotoUpload(ctx context.Context, userID, voiceID, uploadID string) (PhotoUpload, error)
	DeletePhotoUpload(ctx context.Context, id string) error
}

// PhotoSweepLedger is what the photo sweep asks the database (→POST-40).
type PhotoSweepLedger interface {
	ListPhotoUploadsExpiredBefore(ctx context.Context, cutoff time.Time) ([]PhotoUpload, error)
	PhotoKeyInUse(ctx context.Context, key string) (bool, error)
	// AllReferencedPhotoKeys is every key a 학습 글 or a pending upload names, read in one
	// transaction.
	AllReferencedPhotoKeys(ctx context.Context) (map[string]struct{}, error)
	DeletePhotoUpload(ctx context.Context, id string) error
}

// ErrObjectNotFound is a HEAD of a key storage does not hold.
var ErrObjectNotFound = errors.New("object not found")

// ObjectHead is what a HEAD reports about a stored photo.
type ObjectHead struct {
	Size        int64
	ContentType string
}

// StoredObject is one listed object.
type StoredObject struct {
	Key          string
	LastModified time.Time
}

// ObjectStore is the private bucket as the voice context needs it: the post port's shape,
// adapted over internal/storage in cmd/api (ARCH-6).
type ObjectStore interface {
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Head(ctx context.Context, key string) (ObjectHead, error)
	// Read is a stored photo's bytes for a model call (a 검증 on a photo prompt), capped at the
	// photo limit.
	Read(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]StoredObject, error)
}

// Storage is every behaviour the voice context's SQL store happens to implement. It is the
// composition root's handle, NOT a port: each use-case below holds only the narrow interfaces
// it calls (ARCH-6).
type Storage interface {
	VoiceDirectoryStore
	AnalysisStore
	SampleStore
	PhotoUploadStore
	CheckStore
}

// PostContents is how the voice context reads one of the caller's posts to count it (VOICE-62,
// POST-102); the composition root adapts the post context, and this context never reads post
// tables. A foreign post is ErrPostForbidden and an unknown one ErrPostNotFound; voiceID is ""
// for a post with 말투 없음 and blocks is nil before its first write.
type PostContents interface {
	PostForFingerprint(ctx context.Context, userID, slug string) (voiceID string, revision int64, blocks []Block, err error)
}

// Models resolves a model the request names and performs calls through the provider-neutral
// llm boundary. The client names the model explicitly, like every start RPC (MODEL-23).
type Models interface {
	Resolve(ref llm.ModelRef) (llm.ModelInfo, bool)
	Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error)
}

// Jobs is the shared queue behavior this context consumes. Its types are defined here;
// the composition root adapts the job context without coupling sibling domains. Voice-owned
// work is guarded per voice so two voices may analyze at once.
type Jobs interface {
	Enqueue(ctx context.Context, request AnalysisJobRequest) (string, error)
	// EnqueueCheck starts one check_voice job holding one write call (VOICE-43).
	EnqueueCheck(ctx context.Context, request CheckJobRequest) (string, error)
	ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*ActiveJob, error)
	// HasActiveForVoice reports any queued/running job frozen to the voice, whatever its kind
	// or post — the whole set a soft delete must wait for.
	HasActiveForVoice(ctx context.Context, voiceID string) (bool, error)
}
type Progress func(stage string, done, total int)

// MaterialEdits preserves sample kind/prompt, accepts only owned uploaded photos and records
// its operation receipt atomically with content CAS. Label-only changes keep content revision.
type MaterialEdits interface {
	UpdateVoiceSample(context.Context, SampleMutation) (Sample, error)
}

// AcceptedProfiles resolves exactly the owned accepted snapshot, excluding withdrawn excerpts.
type AcceptedProfiles interface {
	AcceptedProfile(context.Context, string, string, string) (Analysis, error)
}
