package rpc

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// ARCH-3: every wire basis but UNSPECIFIED is produced by exactly one product basis, so a value
// added to the enum without a mapping fails here rather than reaching a client as nothing.
func TestPostCreditsBasisMirrorsTheWireEnum(t *testing.T) {
	values := postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_UNSPECIFIED.Descriptor().Values()
	mapped := map[postpilotv1.PostCreditsBasis]int{}
	for _, wire := range postCreditsBasis {
		mapped[wire]++
	}
	for i := 0; i < values.Len(); i++ {
		wire := postpilotv1.PostCreditsBasis(values.Get(i).Number())
		if wire == postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_UNSPECIFIED {
			if mapped[wire] != 0 {
				t.Fatal("a product basis maps to UNSPECIFIED")
			}
			continue
		}
		if mapped[wire] != 1 {
			t.Errorf("%s is produced by %d product bases, want 1", wire, mapped[wire])
		}
	}
	if got := PostCreditsBasisToProto("unknown"); got != postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_UNSPECIFIED {
		t.Fatalf("unknown basis = %s", got)
	}
}
