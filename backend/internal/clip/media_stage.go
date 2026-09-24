package clip

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	ErrMediaConflict  = errors.New("media stage conflicts with frozen work")
	ErrMediaLeaseLost = errors.New("media lease is no longer current")
)

type MediaOperation string

const (
	MediaPrepare MediaOperation = "prepare"
	MediaRender  MediaOperation = "render"
)

type MediaStageState string

const (
	MediaQueued    MediaStageState = "queued"
	MediaRunning   MediaStageState = "running"
	MediaSucceeded MediaStageState = "succeeded"
	MediaFailed    MediaStageState = "failed"
	MediaCancelled MediaStageState = "cancelled"
)

type MediaFailure string

const (
	MediaFailureWorkerLost        MediaFailure = "worker_lost"
	MediaFailureWaitExpired       MediaFailure = "wait_expired"
	MediaFailureDeadlineExceeded  MediaFailure = "deadline_exceeded"
	MediaFailureAttemptsExhausted MediaFailure = "attempts_exhausted"
	MediaFailureInvalidInput      MediaFailure = "invalid_input"
	MediaFailureInvalidOutput     MediaFailure = "invalid_output"
	MediaFailureInternal          MediaFailure = "internal"
	MediaFailureWorkspaceLimit    MediaFailure = "workspace_limit"
	MediaFailureInputTooLarge     MediaFailure = "input_too_large"
	MediaFailureAnalysisTooLarge  MediaFailure = "analysis_too_large"
	// A control acknowledgement; never stored in the stage failure column.
	MediaFailureCancelled MediaFailure = "cancelled"
)

// MediaStageInput is the immutable, versioned work envelope. Payload is an
// edge-encoded domain snapshot, never media bytes or a signed access URL.
type MediaStageInput struct {
	ID, ParentJobID, UserID, ProjectID                  string
	ExpectedRevision                                    int
	Operation                                           MediaOperation
	ContractVersion                                     int
	InputDigest, Payload, RendererVersion, AssetVersion string
	Limits                                              MediaStageLimits
}

type MediaStageLimits struct {
	LeaseTTL, WaitTimeout, StageTimeout time.Duration
	MaxAttempts                         int
}

func (l MediaStageLimits) Validate() error {
	if l.LeaseTTL <= 0 || l.LeaseTTL >= l.WaitTimeout || l.WaitTimeout > l.StageTimeout || l.StageTimeout > MediaStageTimeoutMax || l.MaxAttempts < 1 || l.MaxAttempts > MediaAttemptsMax {
		return ErrInvalid
	}
	return nil
}

func MediaPayloadDigest(payload string) string {
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func ValidMediaLabel(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= MediaLabelMaxBytes && !strings.ContainsAny(value, "\x00\r\n")
}

func (in MediaStageInput) Validate() error {
	for _, v := range []string{in.ID, in.ParentJobID, in.UserID, in.ProjectID, in.RendererVersion, in.AssetVersion} {
		if !ValidMediaLabel(v) {
			return ErrInvalid
		}
	}
	if in.ExpectedRevision < 0 || in.ContractVersion < 1 || (in.Operation != MediaPrepare && in.Operation != MediaRender) || len(in.Payload) == 0 || len(in.Payload) > MediaPayloadMaxBytes || in.InputDigest != MediaPayloadDigest(in.Payload) {
		return ErrInvalid
	}
	return in.Limits.Validate()
}

type MediaStage struct {
	MediaStageInput
	State                                  MediaStageState
	CurrentAttemptID                       string
	AttemptCount                           int
	CreatedAt, QueueDeadlineAt, DeadlineAt time.Time
	AcceptedResult                         string
	Failure                                MediaFailure
	RetryNotBefore                         time.Time
}

type MediaWorkerProfile struct {
	WorkerID                                                string
	Operation                                               MediaOperation
	ContractVersion                                         int
	RendererVersion, AssetVersion, Profile, RuntimeManifest string
}

func (p MediaWorkerProfile) Validate() error {
	for _, v := range []string{p.WorkerID, p.RendererVersion, p.AssetVersion, p.Profile} {
		if !ValidMediaLabel(v) {
			return ErrInvalid
		}
	}
	if p.ContractVersion < 1 || (p.Operation != MediaPrepare && p.Operation != MediaRender) || len(p.RuntimeManifest) == 0 || len(p.RuntimeManifest) > MediaManifestMaxBytes {
		return ErrInvalid
	}
	return nil
}

// Credentials are ephemeral; only the token hash is persisted by the store.
type MediaLeaseCredentials struct{ StageID, AttemptID, WorkerID, Token string }
type MediaLease struct {
	Stage          MediaStage
	Credentials    MediaLeaseCredentials
	Profile        MediaWorkerProfile
	ExpiresAt      time.Time
	HeartbeatAfter time.Duration
}

// MediaArtifactReservation is metadata for a private output slot; signing and
// object I/O belong to the artifact use case, outside the writer transaction.
type MediaArtifactReservation struct {
	Slot, ObjectKey, ContentType string
	MaxBytes                     int64
}
