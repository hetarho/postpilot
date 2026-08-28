package generation

import "errors"

var (
	ErrNotFound                    = errors.New("post or job not found")
	ErrForbidden                   = errors.New("post or job belongs to another user")
	ErrWriteModelRequired          = errors.New("an enabled write model is required")
	ErrObserveModelRequired        = errors.New("an enabled vision observe model is required")
	ErrRevisionInstructionRequired = errors.New("a revision instruction is required")
	ErrRevisionInstructionTooLong  = errors.New("the revision instruction is too long")
	ErrRevisionContentRequired     = errors.New("generated content is required before revision")
)
