package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

type eligibilityAdmission struct {
	models  []clip.AnalysisCandidate
	answers map[string]error
	calls   atomic.Int32
	writes  atomic.Int32
}

func (f *eligibilityAdmission) ObserveModels() []clip.AnalysisCandidate { return f.models }

func (f *eligibilityAdmission) QualifyObserve(ctx context.Context, ref llm.ModelRef) error {
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
	admission := &eligibilityAdmission{
		models: []clip.AnalysisCandidate{{Ref: ref("text"), VideoInput: false}, {Ref: ref("google"), VideoInput: true}, {Ref: ref("qwen"), VideoInput: true}, {Ref: ref("amazon"), VideoInput: true}, {Ref: ref("dear"), VideoInput: true}, {Ref: ref("down"), VideoInput: true}},
		answers: map[string]error{
			"qwen":   nil,
			"google": nil,
			"amazon": llm.ErrRequiredParametersUnsupported,
			"dear":   llm.ErrPriceCeilingUnavailable,
			"down":   errors.New("provider disabled"),
		},
	}
	s := &GenerationService{admission: admission}
	got, err := s.ListAnalysisEligibility(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []clip.ModelEligibility{
		{Model: ref("text"), Status: clip.EligibilityVideoInputAbsent}, {Model: ref("google"), Status: clip.EligibilityEligible}, {Model: ref("qwen"), Status: clip.EligibilityEligible},
		{Model: ref("amazon"), Status: clip.EligibilityRequiredParametersUnsupported}, {Model: ref("dear"), Status: clip.EligibilityPriceCeilingUnavailable}, {Model: ref("down"), Status: clip.EligibilityInlineEndpointUnavailable},
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
	if _, err := (&GenerationService{}).ListAnalysisEligibility(context.Background()); !errors.Is(err, clip.ErrPricingUnavailable) {
		t.Fatal("no qualifier wired yet answered")
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
		var named *clip.ModelAdmissionError
		if !errors.As(wrapped, &named) || !errors.Is(wrapped, err) || !errors.Is(wrapped, llm.ErrUnsupported) {
			t.Fatalf("%v not named", err)
		}
		f := (&clip.StageFailure{Stage: "analyze", Cause: wrapped}).Failure()
		if f.Reason != reason || len(f.Params) != 1 || f.Params["model"] != "openrouter/someone/video" || f.TechnicalDetail != "" {
			t.Fatalf("failure = %+v", f)
		}
		// Naming twice keeps the first name.
		if again := admissionRefusal(ref("other"), wrapped); again != wrapped {
			t.Fatal("re-wrapped")
		}
	}
	plain := errors.New("disk full")
	if admissionRefusal(model, plain) != plain || admissionRefusal(model, clip.ErrPricingUnavailable) != clip.ErrPricingUnavailable {
		t.Fatal("a non-admission error was named as one")
	}
	if status, ok := clip.EligibilityOf(&llm.AdmissionError{}); ok || status != "" {
		t.Fatal("an unknown admission kind mapped to a status")
	}
}
