package llm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestRegistryExecutesOnlyDurablyAdmittedRefAndKeepsFrozenGrade(t *testing.T) {
	provider := &fakeProvider{}
	source := fakeSource{models: []llm.SourceModel{{ModelID: "free-model", Stages: []string{"write"}, Levels: map[string]string{"write": "free"}}}}
	registry, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "key"}), adaptersWith(provider), source, opts)
	if err != nil {
		t.Fatal(err)
	}
	registry.WithModelGrades()
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "free-model"}
	req := llm.Request{Stage: "write", Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("hello")}}}}
	if _, err := registry.Complete(context.Background(), ref, req); !errors.Is(err, llm.ErrModelUnavailable) {
		t.Fatalf("unadmitted call = %v", err)
	}
	ctx := llm.WithAdmittedCalls(context.Background(), []llm.AdmittedCall{{Ref: ref, Stage: "write", Grade: "free"}})
	// Curation can change after admission; the recorded grade still controls routing.
	source.models[0].Levels["write"] = "top"
	if _, err := registry.Complete(ctx, ref, req); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || !provider.last.FreeCall {
		t.Fatalf("provider calls=%d free=%v", provider.calls, provider.last.FreeCall)
	}
	if _, err := registry.Complete(ctx, ref, llm.Request{Stage: "observe"}); !errors.Is(err, llm.ErrModelUnavailable) {
		t.Fatalf("forged stage = %v", err)
	}
}
