package rpc

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestQuoteFailuresHaveStableReasons(t *testing.T) {
	for err, reason := range map[error]string{clip.ErrCancellationPolicy: "CLIP_CANCELLATION_POLICY_REQUIRED", clip.ErrQuoteRequired: "CLIP_QUOTE_REQUIRED", clip.ErrQuoteExpired: "CLIP_QUOTE_EXPIRED", clip.ErrQuoteChanged: "CLIP_QUOTE_CHANGED", clip.ErrPricingUnavailable: "CLIP_MODEL_PRICING_UNAVAILABLE", clip.ErrRateUnavailable: "AI_FX_RATE_UNAVAILABLE"} {
		var ce *connect.Error
		if e := toConnectError(err); !errors.As(e, &ce) || ce.Code() != connect.CodeFailedPrecondition {
			t.Fatal(e)
		}
		detail, e := ce.Details()[0].Value()
		if e != nil || detail.(*v1.AppErrorDetail).Reason != reason {
			t.Fatal(detail, e)
		}
	}
}

func TestApprovalAndAccountingWirePreservesPresence(t *testing.T) {
	f := (&v1.StartClipGenerationRequest{}).ProtoReflect().Descriptor().Fields().ByName("approved_max_credits")
	if f == nil || !f.HasPresence() || f.Number() != 6 {
		t.Fatal("approval presence/field number")
	}
	zero := 0
	maximum := 18
	pending := accountingProto(&clip.Accounting{JobID: "job", Status: "reserved", ApprovedMax: &maximum, Reserved: &zero})
	if pending.ReservedCredits == nil || *pending.ReservedCredits != 0 || pending.FinalChargeCredits != nil || pending.Settled {
		t.Fatal(pending)
	}
	settled := accountingProto(&clip.Accounting{JobID: "job", Status: "settled", FinalCharge: &zero, Refund: &maximum, Settled: true})
	if settled.FinalChargeCredits == nil || *settled.FinalChargeCredits != 0 || !settled.Settled {
		t.Fatal(settled)
	}
}

// QUOTA-65, QUOTA-68: a quote and its accounting carry credits and no rate, master included;
// the frozen snapshot stays on the job for audit.
func TestFXRateReachesNoCaller(t *testing.T) {
	for _, message := range []interface {
		ProtoReflect() protoreflect.Message
	}{&v1.QuoteClipGenerationResponse{}, &v1.QuoteClipRevisionResponse{}, &v1.ClipAccounting{}} {
		if message.ProtoReflect().Descriptor().Fields().ByName("rate") != nil {
			t.Fatalf("%s still carries the exchange rate", message.ProtoReflect().Descriptor().FullName())
		}
	}
	rate := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29", ReferenceE4: 13_925_000, AppliedE4: 14_000_000}
	charge := 7
	accounting := &clip.Accounting{JobID: "job", Status: "settled", FinalCharge: &charge, Settled: true, Rate: rate}
	if got := accountingProto(accounting); got.GetFinalChargeCredits() != 7 {
		t.Fatalf("accounting = %v", got)
	}
}

func TestPreparationFailuresHaveStableReasons(t *testing.T) {
	for err, reason := range map[error]string{clip.ErrAnalysisTooLarge: "CLIP_ANALYSIS_TOO_LARGE", clip.ErrWorkspaceLimit: "CLIP_WORKSPACE_LIMIT", clip.ErrModelInputUnsupported: "CLIP_MODEL_INPUT_UNSUPPORTED"} {
		var ce *connect.Error
		if e := toConnectError(err); !errors.As(e, &ce) || len(ce.Details()) != 1 {
			t.Fatal(e)
		}
		detail, e := ce.Details()[0].Value()
		if e != nil || detail.(*v1.AppErrorDetail).Reason != reason {
			t.Fatal(detail, e)
		}
	}
}

// The four admission reasons ride the same detail contract with the model as
// their one display-safe parameter; the wire enum has one value per status and
// treats anything else as unspecified, never eligible.
func TestAdmissionRefusalsAndEligibilityWire(t *testing.T) {
	model := llm.ModelRef{ProviderID: "openrouter", ModelID: "someone/video"}
	for cause, reason := range map[error]string{llm.ErrVideoInputAbsent: "CLIP_MODEL_VIDEO_INPUT_ABSENT", llm.ErrInlineEndpointUnavailable: "CLIP_MODEL_INLINE_ENDPOINT_UNAVAILABLE", llm.ErrRequiredParametersUnsupported: "CLIP_MODEL_REQUIRED_PARAMETERS_UNSUPPORTED", llm.ErrPriceCeilingUnavailable: "CLIP_MODEL_PRICE_CEILING_UNAVAILABLE"} {
		var ce *connect.Error
		if e := toConnectError(&clip.ModelAdmissionError{Model: model, Err: cause}); !errors.As(e, &ce) || ce.Code() != connect.CodeFailedPrecondition {
			t.Fatal(e)
		}
		detail, e := ce.Details()[0].Value()
		d := detail.(*v1.AppErrorDetail)
		if e != nil || d.Reason != reason || len(d.Params) != 1 || d.Params["model"] != "openrouter/someone/video" {
			t.Fatal(d, e)
		}
	}
	for status, want := range map[clip.EligibilityStatus]v1.ClipAnalysisEligibility{
		clip.EligibilityEligible:                      v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_ELIGIBLE,
		clip.EligibilityVideoInputAbsent:              v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_VIDEO_INPUT_ABSENT,
		clip.EligibilityInlineEndpointUnavailable:     v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_INLINE_ENDPOINT_UNAVAILABLE,
		clip.EligibilityRequiredParametersUnsupported: v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_REQUIRED_PARAMETERS_UNSUPPORTED,
		clip.EligibilityPriceCeilingUnavailable:       v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_PRICE_CEILING_UNAVAILABLE,
		"":                                            v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_UNSPECIFIED,
	} {
		if got := eligibilityProto(status); got != want {
			t.Fatalf("%q → %v", status, got)
		}
	}
}
