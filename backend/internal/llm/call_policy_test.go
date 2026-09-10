package llm_test

import (
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestFreezeCallCapturesEffectiveReasoningAndExactRatesWithoutInvocation(t *testing.T) {
	p := &fakeProvider{}
	source := fakeSource{models: []llm.SourceModel{{ModelID: "clip", Stages: []string{"observe", "write"}, InputUSDPerMillion: "0.1234500", OutputUSDPerMillion: "0", Reasoning: map[string]llm.ReasoningEffort{"observe": llm.ReasoningNone}, ReasoningEfforts: []llm.ReasoningEffort{llm.ReasoningHigh}}}}
	r, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(p), source, opts)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := r.FreezeCall(llm.ModelRef{ProviderID: "openrouter", ModelID: "clip"}, "observe", 8192, llm.ReasoningLow)
	if err != nil || !policy.Valid() || policy.Reasoning != llm.ReasoningNone || !policy.DisableReasoning || policy.InputUSDPerMillion != "0.1234500" || policy.OutputUSDPerMillion != "0" || p.calls != 0 {
		t.Fatal(policy, err, p.calls)
	}
	if _, err = r.FreezeCall(policy.Ref, "analyze", 8192, llm.ReasoningLow); err == nil {
		t.Fatal("unregistered stage")
	}
	if _, err = r.FreezeCall(policy.Ref, "write", 0, llm.ReasoningLow); err == nil {
		t.Fatal("missing budget")
	}
}
