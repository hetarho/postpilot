package clip

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/plan"
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

func TestClipWorkerErrorStringCannotLeakMediaPaths(t *testing.T) {
	cause := fmt.Errorf("%w: /private/clip-work/source-secret.mp4: stderr=private contents", ErrInvalidMedia)
	err := &StageFailure{Stage: "prepare", Cause: cause}
	if !errors.Is(err, ErrInvalidMedia) || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "source-secret") || err.Error() != "clip prepare: CLIP_INVALID_MEDIA" {
		t.Fatal(err.Error())
	}
}
func TestPreparationResourceFailuresStayTypedAndRedacted(t *testing.T) {
	for err, reason := range map[error]string{ErrAnalysisTooLarge: "CLIP_ANALYSIS_TOO_LARGE", ErrWorkspaceLimit: "CLIP_WORKSPACE_LIMIT", ErrModelInputUnsupported: "CLIP_MODEL_INPUT_UNSUPPORTED"} {
		f := (&StageFailure{Stage: "prepare", Cause: err}).Failure()
		if f.Reason != reason || len(f.Params) != 0 || f.TechnicalDetail != "" {
			t.Fatal(f)
		}
	}
}
