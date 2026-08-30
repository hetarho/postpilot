package generation

import "errors"

var (
	ErrNotFound                    = errors.New("post or job not found")
	ErrForbidden                   = errors.New("post or job belongs to another user")
	ErrWriteModelRequired          = errors.New("an enabled write model is required")
	ErrObserveModelRequired        = errors.New("an enabled vision observe model is required")
	ErrInvalidTargetLength         = errors.New("target length must be positive")
	ErrRevisionInstructionRequired = errors.New("a revision instruction is required")
	ErrRevisionInstructionTooLong  = errors.New("the revision instruction is too long")
	ErrRevisionContentRequired     = errors.New("generated content is required before revision")
	ErrVoiceRequired               = errors.New("the post has no voice")
	// ErrVoiceDeleted refuses AI work for a post whose voice is a tombstone; the post stays
	// readable and exportable, and restoring the voice or reassigning the post lifts it.
	ErrVoiceDeleted = errors.New("the post's voice is deleted; restore it or assign another voice first")
	// ErrVoiceMismatch: the post was reassigned after this job was queued, so its frozen
	// voice no longer matches — the result would land in the wrong profile.
	ErrVoiceMismatch = errors.New("the post was assigned to another voice after this job was queued")
)
