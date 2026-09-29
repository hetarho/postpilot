package usage

import (
	"errors"
	"fmt"

	"github.com/postpilot/backend/internal/plan"
)

// ModelAccessFailure extracts the two admission failures for transport edges.
func ModelAccessFailure(err error) (interface {
	error
	Reason() string
	Params() map[string]string
}, bool) {
	var grade *ModelGradeError
	if errors.As(err, &grade) {
		return grade, true
	}
	var free *FreePathError
	if errors.As(err, &free) {
		return free, true
	}
	return nil, false
}

// ModelGradeError is a plan refusal before an AI job or hold is created.
type ModelGradeError struct {
	Ref, Stage, Grade string
	Required          plan.Plan
	Unavailable       bool
}

func (e *ModelGradeError) Error() string {
	return fmt.Sprintf("model %s is not entitled for %s on this plan", e.Ref, e.Stage)
}
func (e *ModelGradeError) Reason() string {
	if e.Unavailable {
		return "MODEL_UNSUITABLE"
	}
	if e.Required == "" {
		return "MODEL_UNCLASSIFIED"
	}
	return "MODEL_PLAN_REQUIRED"
}
func (e *ModelGradeError) Params() map[string]string {
	return map[string]string{"model": e.Ref, "stage": e.Stage, "grade": e.Grade, "required_plan": string(e.Required)}
}

// FreePathError means a purported free call has unknown or positive pricing.
// It cannot be converted into a paid debit or rerouted to a paid model.
type FreePathError struct{ Ref, Stage string }

func (e *FreePathError) Error() string {
	return fmt.Sprintf("free model %s has no verified zero-cost %s path", e.Ref, e.Stage)
}
func (e *FreePathError) Reason() string { return "MODEL_FREE_PATH_UNAVAILABLE" }
func (e *FreePathError) Params() map[string]string {
	return map[string]string{"model": e.Ref, "stage": e.Stage}
}
