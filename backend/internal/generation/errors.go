package generation

import "errors"

var (
	ErrNotFound             = errors.New("post or job not found")
	ErrForbidden            = errors.New("post or job belongs to another user")
	ErrWriteModelRequired   = errors.New("an enabled write model is required")
	ErrObserveModelRequired = errors.New("an enabled vision observe model is required")
	// ErrVideoUnsupported: the post has a video and the chosen observe model cannot watch
	// one. It is checked BEFORE enqueue rather than at the call, so the run is refused while
	// the user is still looking at the picker they can fix it in (VIDEO-11).
	ErrVideoUnsupported             = errors.New("the selected observe model cannot watch video")
	ErrLanguageRequired             = errors.New("a supported content language is required")
	ErrContentLanguageRequired      = errors.New("content language is required for revision")
	ErrVoiceContentLanguageMismatch = errors.New("post content language does not match voice source language")
	ErrInvalidTargetLength          = errors.New("target length must be positive")
	ErrRevisionInstructionRequired  = errors.New("a revision instruction is required")
	ErrRevisionInstructionTooLong   = errors.New("the revision instruction is too long")
	ErrRevisionContentRequired      = errors.New("generated content is required before revision")
	ErrVoiceRequired                = errors.New("the post has no voice")
	// ErrVoiceDeleted refuses AI work for a post whose voice is a tombstone; the post stays
	// readable and exportable, and restoring the voice or reassigning the post lifts it.
	ErrVoiceDeleted = errors.New("the post's voice is deleted; restore it or assign another voice first")
	// ErrVoiceMismatch: the post was reassigned after this job was queued, so its frozen
	// voice no longer matches — the result would land in the wrong profile.
	ErrVoiceMismatch = errors.New("the post was assigned to another voice after this job was queued")
)

// VideoUnsupportedError names the observe model that cannot watch a clip, so the transport can
// put the ref in the refusal's params — the fix is to pick another model, and the user is
// looking at the picker they would pick it in.
type VideoUnsupportedError struct{ Model string }

func (e *VideoUnsupportedError) Error() string {
	return ErrVideoUnsupported.Error() + ": " + e.Model
}

func (e *VideoUnsupportedError) Unwrap() error { return ErrVideoUnsupported }
