package clip

import "time"

const (
	MediaAnalysisPrefix    = "clip-media/"
	MediaArtifactAccessTTL = 10 * time.Minute
)

// MediaTask freezes execution inputs without exposing storage keys or account
// authority to the worker. The API resolves Source.ID through the owning attempt.
type MediaTask struct {
	Version        int
	Sources        []MediaTaskSource
	Plan           string
	HideDisclosure bool
}

type MediaTaskSource struct {
	ID string
	SourceMetadata
	Info         MediaInfo
	ReusedChunks []int
}

type MediaVerifiedSource struct {
	ID, Fingerprint string
	Info            MediaInfo
}

// MediaOutput describes an encoded file, never a local filename or object key.
// Digest is the worker's SHA-256 measurement. It is not an S3 ETag; a consumer
// verifies it again when it downloads an analysis copy.
type MediaOutput struct {
	Slot, SourceID              string
	Index, OffsetMS, DurationMS int
	Bytes                       int64
	ContentType, Digest         string
	Info                        MediaInfo
}

type MediaResult struct {
	Version int
	Sources []MediaVerifiedSource
	Outputs []MediaOutput
	Plan    string
}

type MediaArtifact struct {
	MediaOutput
	StageID, AttemptID, ObjectKey, State string
	CreatedAt, PutExpiresAt              time.Time
}

type MediaArtifactAccess struct {
	Slot, URL, ContentType string
	Headers                map[string]string
	Bytes                  int64
	ExpiresAfter           time.Duration
}
