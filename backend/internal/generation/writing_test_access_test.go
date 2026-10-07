package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestWritingTestRetryAccessUsesOnlyRemainingFrozenCalls(t *testing.T) {
	factory, ports, _, models, _, _ := plannerFixture(t)
	request, _ := plannerRequest(t, ports, "model", "write", 4)
	for ref, info := range ports.models {
		models.infos[ref] = info
	}
	snapshot, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	full, err := factory.PlanWritingTest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	remaining := full[len(full)-1:]
	ports.blocked[full[0].Ref] = ErrWriteModelRequired
	reads := ports.sourceReads
	if err := factory.CheckWritingTestAccess(context.Background(), snapshot, remaining); err != nil {
		t.Fatalf("unissued former model blocks retry: %v", err)
	}
	if ports.sourceReads != reads {
		t.Fatal("access check changes frozen source")
	}
	if len(models.calls) != 0 {
		t.Fatal("access inspection issued provider work")
	}
	ports.blocked[remaining[0].Ref] = ErrWriteModelRequired
	if err := factory.CheckWritingTestAccess(context.Background(), snapshot, remaining); !errors.Is(err, ErrWriteModelRequired) {
		t.Fatalf("withdrawn remaining model admitted: %v", err)
	}
}

func TestWritingTestAccessChecksFixedWriterAndRejectsUnfrozenPlan(t *testing.T) {
	factory, ports, _, models, _, _ := plannerFixture(t)
	request, _ := plannerRequest(t, ports, "guideline", "", 4)
	for ref, info := range ports.models {
		models.infos[ref] = info
	}
	snapshot, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	full, err := factory.PlanWritingTest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := factory.CheckWritingTestAccess(context.Background(), snapshot, full); err != nil {
		t.Fatal(err)
	}
	ports.blocked[llm.ModelRef{ProviderID: "p", ModelID: "fixed-write"}] = ErrWriteModelRequired
	if err := factory.CheckWritingTestAccess(context.Background(), snapshot, full); !errors.Is(err, ErrWriteModelRequired) {
		t.Fatalf("setting retry bypasses fixed writer rights: %v", err)
	}
	delete(ports.blocked, llm.ModelRef{ProviderID: "p", ModelID: "fixed-write"})
	for _, alter := range []func(*WritingTestCall){
		func(c *WritingTestCall) { c.Ref.ModelID = "unfrozen" },
		func(c *WritingTestCall) { c.Count += 100 },
		func(c *WritingTestCall) { c.PromptTokens++ },
		func(c *WritingTestCall) { c.CompletionTokens++ },
		func(c *WritingTestCall) { c.Stage = "analyze" },
	} {
		calls := append([]WritingTestCall(nil), full...)
		alter(&calls[0])
		if err := factory.CheckWritingTestAccess(context.Background(), snapshot, calls); !errors.Is(err, ErrWritingTestMaterial) {
			t.Fatalf("unfrozen work admitted: %v", err)
		}
	}
}
