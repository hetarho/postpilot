package usage

import (
	"github.com/postpilot/backend/internal/llm"
	"testing"
)

func TestUnitSchedulePreservesDifferentInputsWithoutUnreservedCalls(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "speech", ModelID: "tts"}
	a := UnitBudget{Ref: ref, Operation: "speech", InputDigest: "first", Count: 1}
	b := a
	b.InputDigest = "changed"
	schedule := []PlannedCall{{Ref: ref, Stage: "speech", Count: 2}}
	out, err := UnitCallsForSchedule(schedule, []UnitBudget{a, b})
	if err != nil || len(out) != 2 || out[0].Units.InputDigest == out[1].Units.InputDigest {
		t.Fatal(out, err)
	}
	for _, bad := range []PlannedCall{{Ref: ref, Stage: "speech", Count: 1}, {Ref: ref, Stage: "speech", Count: 3}, {Ref: ref, Stage: "write", Count: 2}, {Ref: ref, Stage: "speech", Count: 2, CompletionTokens: 1}} {
		if _, err := UnitCallsForSchedule([]PlannedCall{bad}, []UnitBudget{a, b}); err != ErrUnitApproval {
			t.Fatal(bad, err)
		}
	}
}
