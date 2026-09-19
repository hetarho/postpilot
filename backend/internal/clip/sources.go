package clip

import (
	"context"
	"errors"
	"time"
)

var ErrSourceState = errors.New("clip sources are not available in this state")

const SourcePrefix = "clip-inputs/"

type SourceConfig struct {
	MaxCount, MaxDurationMS     int
	MaxFilenameChars            int
	MaxFileBytes, MaxBatchBytes int64
	Containers                  map[string][]string
	BatchTTL, PutTTL            time.Duration
	RetentionTTL, PlaybackTTL   time.Duration
}

// SourceMetadata is a browser declaration until the independent media probe verifies it.
type SourceMetadata struct {
	Filename, ContentType, Fingerprint string
	Bytes                              int64
	DurationMS, Width, Height          int
}
type SourceLease struct {
	ID, Key, State string
	SourceMetadata
	ActualBytes    int64
	ExpiresAt      time.Time
	CleanupPending bool
	Availability   string
	// The owner's 원본 소리 유지 choice for this source (CLIP-18). It is OWNER
	// data, not metadata: it is deliberately outside SourceMetadata so it can
	// never enter the quote digest or the planning recovery identity, and so
	// changing it never looks like a different source to a model.
	RetainOriginalAudio bool
}
type SourceBatch struct {
	ID, UserID, ProjectID, State, JobID string
	ProxyKeys                           []string
	CreatedAt, ExpiresAt                time.Time
	UploadExpiresAt, PutExpiresAt       time.Time
	Sources                             []SourceLease
	Current                             bool
	AccessDenied                        bool
}
type SignedSourcePut struct {
	URL     string
	Headers map[string]string
}
type SourceUpload struct {
	SourceID string
	SignedSourcePut
	ExpiresAt time.Time
}
type SourceBatchUpload struct {
	Batch   SourceBatch
	Uploads []SourceUpload
}
type SourceObjectInfo struct {
	Bytes       int64
	ContentType string
}

// SourceStore owns the atomic lifecycle transitions. Cleanup identities outlive projects.
type SourceStore interface {
	ReplaceSourceBatch(context.Context, SourceBatch) ([]SourceBatch, error)
	GetSourceBatch(context.Context, string, string) (SourceBatch, error)
	ConfirmSourceLease(context.Context, string, string, string, int64, time.Time) (SourceBatch, error)
	MarkSourceCleanup(context.Context, string, string, bool) (SourceBatch, error)
	BeginProjectSourceCleanup(context.Context, string, string) ([]SourceBatch, error)
	RevokeProjectSources(context.Context, string, string, time.Time) ([]SourceBatch, error)
	ReapSourceBatches(context.Context, time.Time) ([]SourceBatch, error)
	RemoveSourceBatch(context.Context, string, string, time.Time) error
	SourceKeyExists(context.Context, string) (bool, error)
	RemoveProxy(context.Context, string) error
	ReleaseSourceAttempt(context.Context, string, string, time.Time) error
	ProjectSourceBatches(context.Context, string, string) ([]SourceBatch, error)
	SetSourceOriginalAudio(context.Context, string, SourceAudioChange) (SourceBatch, Project, error)
}

// SourceAudioChange is one owner decision about one exact file. The fingerprint
// pins the file the choice was made about and the expected revision pins the
// plan the owner was looking at — zero before a plan exists (CLIP-100).
type SourceAudioChange struct {
	ProjectID, BatchID, SourceID, Fingerprint string
	RetainOriginal                            bool
	ExpectedRevision                          int
}
type ObjectStore interface {
	PresignSource(context.Context, string, string, time.Duration) (SignedSourcePut, error)
	PresignSourcePlayback(context.Context, string, string, time.Duration) (string, error)
	HeadSource(context.Context, string) (SourceObjectInfo, error)
	Delete(context.Context, string) error
	// A PUT already in flight can finish after discard deleted its lease. Prefix listing
	// closes that leak without retaining a source attachment or trusting client cancellation.
	ListSourceKeys(context.Context) ([]string, error)
}
