package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

type inspectionSource struct{ model llm.SourceModel }

func (s *inspectionSource) Models() []llm.SourceModel { return []llm.SourceModel{s.model} }
func (s *inspectionSource) Lookup(id string) (llm.SourceModel, bool) {
	return s.model, s.model.ModelID == id
}

type inspectionAdapter struct {
	calls int
	last  llm.Request
	err   error
}

func (*inspectionAdapter) Name() string { return "inspection-fixture" }
func (p *inspectionAdapter) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	p.calls++
	p.last = request
	return llm.Response{Text: "usable", Usage: llm.Usage{PromptTokens: 11, CompletionTokens: 7, ReasoningTokens: 2, CostReported: true, CostMicrousd: 999999}}, p.err
}

func TestModelPortForwardsEffectiveCapturedEvidenceWithoutInspectionAdmissionOrHistoricalGuessing(t *testing.T) {
	adapter := &inspectionAdapter{}
	source := &inspectionSource{model: llm.SourceModel{ModelID: "explicit-writer", Stages: []string{llm.StageNameWrite}, Reasoning: map[string]llm.ReasoningEffort{llm.StageNameWrite: llm.ReasoningNone}, ReasoningEfforts: []llm.ReasoningEffort{llm.ReasoningLow, llm.ReasoningHigh}}}
	registry, err := llm.Parse([]byte("providers:\n  - id: fixture\n    adapter: fake\n    base_url: https://private-endpoint.invalid\n    api_key_env: FIXTURE_KEY\n"), func(string) string { return "private-key" }, map[string]llm.AdapterFactory{"fake": func(llm.AdapterConfig) (llm.Provider, error) { return adapter, nil }}, source, llm.Options{Timeout: time.Second, MaxTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	ref := llm.ModelRef{ProviderID: "fixture", ModelID: "explicit-writer"}
	request := llm.Request{System: "고정 지시", Stage: llm.StageNameWrite, Reasoning: llm.ReasoningLow, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("재료🙂")}}}, Composition: &llm.RequestComposition{Stage: "post-writing", Mode: "synthetic-fixture", PromptVersion: "fixture-v1", SchemaVersion: "plain-v1", Composer: "fixture.assemble", Parser: "plain", Consumer: "private-caller-result", SourceFiles: []string{"cmd/api/request_inspection_test.go"}, Activation: "synthetic request", Output: llm.OutputContractInspection{Name: "plain", Version: "plain-v1"}, Fragments: []llm.RequestFragment{{ID: "contract", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "code-contract", Text: "고정 지시"}, {ID: "material", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "synthetic-owner-material", Text: "재료🙂"}}}}
	prepared, err := registry.Prepare(t.Context(), ref, request)
	if err != nil || adapter.calls != 0 || prepared.Status != llm.InspectionPrepared || prepared.IssuedAt != nil || !*prepared.Conditions.DefaultBudget || !*prepared.Conditions.DisableReasoning {
		t.Fatalf("pure effective preparation is wrong: %+v %v", prepared, err)
	}
	// A changed registration must be visible only in the later actual witness;
	// retaining the earlier preview as the issued request would be false history.
	source.model.Reasoning[llm.StageNameWrite] = llm.ReasoningHigh
	port := generationModels{registry: meteredRegistry{Registry: registry}}
	response, err := port.Complete(t.Context(), ref, request)
	if err != nil || adapter.calls != 1 || response.Inspection == nil || response.Inspection.Status != llm.InspectionCaptured {
		t.Fatalf("model adapter discarded the actual witness: %+v %v", response.Inspection, err)
	}
	if adapter.last.MaxTokens != 128 || adapter.last.Reasoning != llm.ReasoningHigh || adapter.last.DisableReasoning || *response.Inspection.Conditions.ReasoningEffort != llm.ReasoningHigh || *response.Inspection.Conditions.DisableReasoning || *prepared.Conditions.ReasoningEffort != llm.ReasoningNone {
		t.Fatal("preview conditions were guessed or mutated after dispatch")
	}
	if *response.Inspection.Measures.ProviderPromptTokens != 11 || *response.Inspection.Measures.ProviderCompletionTokens != 7 || *response.Inspection.Measures.ProviderReasoningTokens != 2 {
		t.Fatal("actual usage was lost at the metered model boundary")
	}
	failed := errors.New("adapter invocation failed")
	adapter.err = failed
	response, err = port.Complete(t.Context(), ref, request)
	inspection, found := llm.RequestInspectionFromError(err)
	if !errors.Is(err, failed) || !found || inspection.Status != llm.InspectionCaptured || response.Inspection == nil || adapter.calls != 2 {
		t.Fatal("billable invocation failure lost its cause or witness", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if response, err := port.Complete(cancelled, ref, request); !errors.Is(err, context.Canceled) || response.Inspection != nil || adapter.calls != 2 {
		t.Fatal("cancelled preflight claimed an adapter invocation", err)
	}
	// The authoring adapter's existing durable authorization precedes the LLM
	// registry. A refused caller never gets an actual request witness.
	response, err = (authoringModels{}).Complete(t.Context(), ref, request)
	if !errors.Is(err, job.ErrDispatchRefused) || response.Inspection != nil || adapter.calls != 2 {
		t.Fatal("unadmitted authoring work claimed a request", err)
	}
}
