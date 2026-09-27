package rpc_test

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
)

// ARCH-3, F111: the ladder's one wire mapping is walked against the generated enum and the
// domain ladder, so a tier added on either side fails here instead of travelling as UNSPECIFIED.
func TestPlanMappingCoversGeneratedEnum(t *testing.T) {
	if got, want := len(postpilotv1.Plan_name), len(plan.Ladder())+1; got != want {
		t.Fatalf("generated plans = %d, want %d (the ladder plus UNSPECIFIED); update the mapping", got, want)
	}
	for number, name := range postpilotv1.Plan_name {
		wire := postpilotv1.Plan(number)
		t.Run(name, func(t *testing.T) {
			domain, ok := planrpc.FromProto(wire)
			if wire == postpilotv1.Plan_PLAN_UNSPECIFIED {
				if ok {
					t.Fatalf("UNSPECIFIED mapped to %q", domain)
				}
				return
			}
			if !ok {
				t.Fatalf("generated plan %s has no domain mapping", name)
			}
			if got := planrpc.ToProto(domain); got != wire {
				t.Fatalf("round trip = %s, want %s", got, wire)
			}
		})
	}
	for _, rung := range plan.Ladder() {
		if planrpc.ToProto(rung) == postpilotv1.Plan_PLAN_UNSPECIFIED {
			t.Errorf("ladder rung %q has no wire value", rung)
		}
	}
}

func TestPlanMappingRefusesUnknownValues(t *testing.T) {
	if got, ok := planrpc.FromProto(postpilotv1.Plan(999)); ok {
		t.Fatalf("an unknown wire value mapped to %q", got)
	}
	for _, value := range []plan.Plan{"", "enterprise"} {
		if got := planrpc.ToProto(value); got != postpilotv1.Plan_PLAN_UNSPECIFIED {
			t.Errorf("domain value %q mapped to %s", value, got)
		}
	}
}
