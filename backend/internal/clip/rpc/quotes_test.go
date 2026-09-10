package rpc

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestQuoteFailuresHaveStableReasons(t *testing.T) {
	for err, reason := range map[error]string{clip.ErrQuoteRequired: "CLIP_QUOTE_REQUIRED", clip.ErrQuoteExpired: "CLIP_QUOTE_EXPIRED", clip.ErrQuoteChanged: "CLIP_QUOTE_CHANGED", clip.ErrPricingUnavailable: "CLIP_MODEL_PRICING_UNAVAILABLE"} {
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
