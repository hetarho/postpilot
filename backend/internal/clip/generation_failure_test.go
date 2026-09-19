package clip

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

func TestDurableCompositionFailureKeepsElementIdentityWithoutDraftText(t *testing.T) {
	err := &StageFailure{Stage: "render", Cause: fmt.Errorf("private draft/path: %w", &composition.Problem{ElementID: "price", Line: 7, Reason: "copy_limit"})}
	failure := err.Failure()
	if failure.Reason != "CLIP_COMPOSITION_INVALID" || !reflect.DeepEqual(failure.Params, map[string]string{"element_id": "price", "line": "7", "reason": "copy_limit"}) || failure.TechnicalDetail != "" || strings.Contains(err.Error(), "private") {
		t.Fatal("durable failure lost its safe element identity", failure)
	}
}

// A BoundedText answer is the one composition refusal an owner can hit without ever
// seeing the template body, so it names the field instead of a line (CLIP-102).
func TestBoundedAnswerFailureNamesTheFieldAndItsCounts(t *testing.T) {
	problem := &composition.Problem{ElementID: "place", Line: 3, Reason: "answer_limit", Label: "상호", Max: 6, Actual: 9}
	failure := (&StageFailure{Stage: "prepare", Cause: fmt.Errorf("private draft: %w", problem)}).Failure()
	want := map[string]string{"element_id": "place", "line": "3", "reason": "answer_limit", "label": "상호", "max": "6", "actual": "9"}
	if failure.Reason != "CLIP_COMPOSITION_INVALID" || !reflect.DeepEqual(failure.Params, want) {
		t.Fatal("BoundedText answer lost its label or counts", failure)
	}
	// Every other reason keeps the three parameters it always had.
	plain := (&StageFailure{Stage: "render", Cause: &composition.Problem{ElementID: "price", Line: 7, Reason: "copy_limit"}}).Failure()
	if !reflect.DeepEqual(plain.Params, map[string]string{"element_id": "price", "line": "7", "reason": "copy_limit"}) {
		t.Fatal("unbounded reason gained parameters", plain)
	}
}

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

func TestClipDiagnosticMetadataDoesNotChangePublicFailure(t *testing.T) {
	cause := errors.New("BoundedText provider request failed")
	want := (&StageFailure{Stage: "analyze", Cause: cause}).Failure()
	err := &StageFailure{Stage: "analyze", Cause: llm.WithCallDiagnostic(cause, llm.CallDiagnostic{Operation: "response", Class: "http_error", HTTPStatus: 403, RequestID: "req-0123456789abcdef"})}
	if !reflect.DeepEqual(err.Failure(), want) || err.FailureStage() != "analyze" || !errors.Is(err, cause) {
		t.Fatal("public failure changed", err.Failure())
	}
	if _, ok := llm.DiagnosticOf(err); !ok {
		t.Fatal("stage wrapper lost server diagnostics")
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
	for err, reason := range map[error]string{ErrInputTooLarge: "CLIP_INPUT_TOO_LARGE", ErrAnalysisTooLarge: "CLIP_ANALYSIS_TOO_LARGE", ErrWorkspaceLimit: "CLIP_WORKSPACE_LIMIT", ErrModelInputUnsupported: "CLIP_MODEL_INPUT_UNSUPPORTED"} {
		f := (&StageFailure{Stage: "prepare", Cause: err}).Failure()
		if f.Reason != reason || len(f.Params) != 0 || f.TechnicalDetail != "" {
			t.Fatal(f)
		}
	}
}

// The floor's shortfall reaches the owner as its own cause, distinct from the
// unreadable-response cause a failed decode carries (CLIP-120).
func TestInsufficientFootageFailsUnderItsOwnCause(t *testing.T) {
	f := (&StageFailure{Stage: "plan", Cause: fmt.Errorf("clip output: %w", ErrInsufficientFootage)}).Failure()
	if f.Reason != reasonInsufficientFootage || len(f.Params) != 0 || f.TechnicalDetail != "" {
		t.Fatal("the footage shortfall lost its own cause", f)
	}
	unreadable := (&StageFailure{Stage: "plan", Cause: fmt.Errorf("clip output: %w", llm.ErrBadOutput)}).Failure()
	if unreadable.Reason == f.Reason {
		t.Fatal("a footage shortfall and an unreadable response share one cause", unreadable)
	}
}
