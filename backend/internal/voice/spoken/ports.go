package spoken

import (
	"context"
	"github.com/postpilot/backend/internal/plan"
	"time"
)

type ProfileResolver interface {
	ResolveSpokenProfile(context.Context, string, plan.Plan, string, int64, string) (Profile, error)
}

type LibraryReads interface {
	ListDrafts(context.Context, string) ([]Draft, error)
	GetDraft(context.Context, string, string) (Draft, error)
	ListVoices(context.Context, string, bool) ([]Voice, error)
	GetVoice(context.Context, string, string) (Voice, error)
	GetAsset(context.Context, string, string) (Asset, error)
}

// Mutate checks owner/idempotency/revision in one writer transaction. Its closure
// performs metadata writes only; uploads and catalog requests happen outside it.
type LibraryWrites interface {
	Mutate(context.Context, RequestIdentity, func(Storage) (MutationResult, error)) (MutationResult, error)
	InsertDraft(context.Context, Draft) error
	UpdateDraft(context.Context, Draft, int64) error
	DeleteDraft(context.Context, string, string, int64) error
	RenameVoice(context.Context, string, string, int64, string) error
	RemoveVoice(context.Context, string, string, int64, time.Time) error
	SelectCandidate(context.Context, string, string, int64, string) error
	AcknowledgeCandidate(context.Context, string, string, int64, string, string, time.Time) error
	AttachCandidates(context.Context, string, string, int64, string, []Candidate, []Asset) error
	ConfirmVoice(context.Context, string, string, int64, string, Voice) error
	AcquireVoice(context.Context, string, string, int64, string) (Voice, error)
}

type AudioAccess interface {
	SavePlayback(context.Context, Playback) error
	GetPlayback(context.Context, string, string, time.Time) (Playback, Asset, error)
	MarkPlaybackServed(context.Context, string, string, time.Time) error
	RevokeAsset(context.Context, string, string, time.Time) error
}

type CleanupLedger interface {
	PrepareCleanup(context.Context, Cleanup) error
	PendingCleanup(context.Context, time.Time) ([]Cleanup, error)
	CompleteCleanup(context.Context, string) error
	DiscardRetainedCleanup(context.Context, string) error
	AssetRetained(context.Context, string) (bool, error)
	DeleteOwner(context.Context, string) error
}

type Storage interface {
	LibraryReads
	LibraryWrites
	AudioAccess
	CleanupLedger
}

type AudioObjects interface {
	PutSpokenAudio(context.Context, string, []byte) error
	ReadSpokenAudio(context.Context, string, int64) ([]byte, error)
	DeleteSpokenAudio(context.Context, string) error
}
