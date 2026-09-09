package clip

import (
	"github.com/postpilot/backend/internal/plan"
	"testing"
)

func TestClipFailureUsesOnlyReasonSpecificParams(t *testing.T) {
	f := (&StageFailure{Stage: "prepare", Cause: &plan.InsufficientCreditsError{Required: 79, Balance: 12}}).Failure()
	if f.Reason != "INSUFFICIENT_CREDITS" || len(f.Params) != 3 || f.Params["required"] != "79" || f.Params["stage"] != "" {
		t.Fatal(f)
	}
	f = (&StageFailure{Stage: "prepare", Cause: ErrInvalidMedia}).Failure()
	if f.Reason != "CLIP_INVALID_MEDIA" || len(f.Params) != 0 {
		t.Fatal(f)
	}
}
