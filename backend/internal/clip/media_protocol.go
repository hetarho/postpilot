package clip

import (
	"errors"
	"time"
)

// These are compatibility identifiers, not host or library version detection.
// A changed execution/output contract must change the matching identifier.
const (
	MediaContractVersion = 1
	MediaRendererVersion = "cpu-v1"
	MediaAssetVersion    = "assets-v1"
	MediaCPUProfile      = "cpu"
	MediaRequestMaxBytes = 4 << 20
	MediaUnaryTimeout    = 10 * time.Second
	MediaPollInterval    = 2 * time.Second
	MediaPollJitter      = 500 * time.Millisecond
)

var (
	ErrMediaIncompatible    = errors.New("media runtime incompatible")
	ErrMediaCancelled       = errors.New("media stage cancelled")
	ErrMediaUnsupported     = errors.New("media operation unavailable")
	ErrMediaUnavailable     = errors.New("media API unavailable")
	ErrMediaUnauthenticated = errors.New("media worker unauthenticated")
)

func (p MediaWorkerProfile) Compatible() bool {
	return p.ContractVersion == MediaContractVersion && p.RendererVersion == MediaRendererVersion && p.AssetVersion == MediaAssetVersion && p.Profile == MediaCPUProfile
}

func (f MediaFailure) Valid() bool {
	switch f {
	case MediaFailureWorkerLost, MediaFailureWaitExpired, MediaFailureDeadlineExceeded, MediaFailureAttemptsExhausted, MediaFailureInvalidInput, MediaFailureInvalidOutput, MediaFailureInternal, MediaFailureCancelled, MediaFailureWorkspaceLimit, MediaFailureInputTooLarge, MediaFailureAnalysisTooLarge:
		return true
	}
	return false
}

type MediaRuntimeStatus struct {
	Waiting, Active, OwnActive int64
}

// MediaWork is the execution-only projection. It deliberately has no user id,
// parent job id, billing data or project authority.
type MediaWork struct {
	Credentials                                         MediaLeaseCredentials
	Operation                                           MediaOperation
	ContractVersion                                     int
	RendererVersion, AssetVersion, InputDigest, Payload string
	LeaseRemaining, HeartbeatAfter, StageRemaining      time.Duration
	// Set by the worker adapter from its monotonic request-start clock.
	LeaseExpiresAt time.Time
}
