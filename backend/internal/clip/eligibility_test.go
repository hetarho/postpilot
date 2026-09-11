package clip

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

type fakeAdmission struct {
	models  []AnalysisCandidate
	answers map[string]error
	calls   atomic.Int32
	writes  atomic.Int32
}

func (f *fakeAdmission) ObserveModels() []AnalysisCandidate { return f.models }
func (f *fakeAdmission) QualifyObserve(ctx context.Context, ref llm.ModelRef) error {
	f.calls.Add(1)
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.answers[ref.ModelID]
}

func ref(id string) llm.ModelRef { return llm.ModelRef{ProviderID: "openrouter", ModelID: id} }

// Every registered observe model gets exactly one status, in registry order; a
// model without video input costs no qualification; anything that is not an
// admission answer has no proof and reads as no endpoint.
func TestListAnalysisEligibilityAnswersEveryObserveModelInOrder(t *testing.T) {
	admission := &fakeAdmission{
		models: []AnalysisCandidate{{ref("text"), false}, {ref("google"), true}, {ref("qwen"), true}, {ref("amazon"), true}, {ref("dear"), true}, {ref("down"), true}},
		answers: map[string]error{
			"qwen":   nil,
			"google": nil,
			"amazon": llm.ErrRequiredParametersUnsupported,
			"dear":   llm.ErrPriceCeilingUnavailable,
			"down":   errors.New("provider disabled"),
		},
	}
	s := (&GenerationService{}).WithAdmission(admission)
	got, err := s.ListAnalysisEligibility(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []ModelEligibility{
		{ref("text"), EligibilityVideoInputAbsent}, {ref("google"), EligibilityEligible}, {ref("qwen"), EligibilityEligible},
		{ref("amazon"), EligibilityRequiredParametersUnsupported}, {ref("dear"), EligibilityPriceCeilingUnavailable}, {ref("down"), EligibilityInlineEndpointUnavailable},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %v, want %v", i, got[i], want[i])
		}
	}
	if admission.calls.Load() != 5 {
		t.Fatalf("%d qualifications for five video models", admission.calls.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	cancel()
	if _, err := s.ListAnalysisEligibility(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("caller's cancellation did not propagate: %v", err)
	}
	if _, err := (&GenerationService{}).ListAnalysisEligibility(context.Background()); !errors.Is(err, ErrPricingUnavailable) {
		t.Fatal("no qualifier wired yet answered")
	}
}

// The wire's unspecified value is not a status, and eligible is never a default.
func TestEligibilityParsingRejectsUnspecified(t *testing.T) {
	for _, value := range []string{"", "ELIGIBLE", "unspecified", "maybe"} {
		if _, err := ParseEligibility(value); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%q parsed", value)
		}
	}
	for _, s := range []EligibilityStatus{EligibilityEligible, EligibilityVideoInputAbsent, EligibilityInlineEndpointUnavailable, EligibilityRequiredParametersUnsupported, EligibilityPriceCeilingUnavailable} {
		if got, err := ParseEligibility(string(s)); err != nil || got != s {
			t.Fatal(s, err)
		}
	}
	if EligibilityEligible.FailureReason() != "" {
		t.Fatal("eligible has a failure reason")
	}
}

// An admission refusal names the model and the check, carries only the
// display-safe model parameter, and is what a durable clip failure and a
// synchronous detail both read; every other error passes through untouched.
func TestAdmissionRefusalsNameTheModelAndOnlyTheModel(t *testing.T) {
	model := ref("someone/video")
	for err, reason := range map[error]string{
		llm.ErrVideoInputAbsent:              "CLIP_MODEL_VIDEO_INPUT_ABSENT",
		llm.ErrInlineEndpointUnavailable:     "CLIP_MODEL_INLINE_ENDPOINT_UNAVAILABLE",
		llm.ErrRequiredParametersUnsupported: "CLIP_MODEL_REQUIRED_PARAMETERS_UNSUPPORTED",
		llm.ErrPriceCeilingUnavailable:       "CLIP_MODEL_PRICE_CEILING_UNAVAILABLE",
	} {
		wrapped := admissionRefusal(model, err)
		var named *ModelAdmissionError
		if !errors.As(wrapped, &named) || !errors.Is(wrapped, err) || !errors.Is(wrapped, llm.ErrUnsupported) {
			t.Fatalf("%v not named", err)
		}
		f := (&StageFailure{Stage: "analyze", Cause: wrapped}).Failure()
		if f.Reason != reason || len(f.Params) != 1 || f.Params["model"] != "openrouter/someone/video" || f.TechnicalDetail != "" {
			t.Fatalf("failure = %+v", f)
		}
		// Naming twice keeps the first name.
		if again := admissionRefusal(ref("other"), wrapped); again != wrapped {
			t.Fatal("re-wrapped")
		}
	}
	plain := errors.New("disk full")
	if admissionRefusal(model, plain) != plain || admissionRefusal(model, ErrPricingUnavailable) != ErrPricingUnavailable {
		t.Fatal("a non-admission error was named as one")
	}
	if status, ok := EligibilityOf(&llm.AdmissionError{}); ok || status != "" {
		t.Fatal("an unknown admission kind mapped to a status")
	}
}
