package llm_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestInputAllowancesAreFrozenPricedAndLegacyCompatible(t *testing.T) {
	source := twoModels()
	source.models[0].VideoInput = true
	p := &pricedTestProvider{}
	r, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "test"}), map[string]llm.AdapterFactory{"fake": func(llm.AdapterConfig) (llm.Provider, error) { return p, nil }}, source, opts)
	if err != nil {
		t.Fatal(err)
	}
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}
	for _, tc := range []struct {
		stage             string
		delivery          llm.ExecutionDelivery
		limit, completion int
	}{
		{"observe", llm.ExecutionInlineStatic, 30000, 8192},
		{"write", llm.ExecutionTextOnly, 64000, 32768},
	} {
		frozen, err := r.FreezeExecution(context.Background(), ref, tc.stage, tc.completion, llm.ReasoningLow, tc.delivery)
		if err != nil || frozen.InputTokens != tc.limit || frozen.InputTokenLimit() != tc.limit {
			t.Fatalf("allowance=%d err=%v", frozen.InputTokens, err)
		}
		cost, ok := frozen.QuoteMicrousd()
		if !ok || cost != int64(tc.limit+tc.completion*2) {
			t.Fatalf("quote=%d valid=%v", cost, ok)
		}
		data, _ := json.Marshal(frozen)
		var restored llm.CallPolicy
		if json.Unmarshal(data, &restored) != nil || restored != frozen {
			t.Fatal("immutable allowance lost on persistence")
		}
		legacy := frozen
		legacy.InputTokens = 0
		oldCost, ok := legacy.QuoteMicrousd()
		if !ok || legacy.InputTokenLimit() != 30000 || oldCost != int64(30000+tc.completion*2) {
			t.Fatal("legacy approval enlarged")
		}
		for _, invalid := range []int{-1, 1, 30001, 64001} {
			legacy.InputTokens = invalid
			if legacy.Valid() {
				t.Fatalf("invalid allowance %d accepted", invalid)
			}
		}
		if tc.stage == "observe" {
			legacy.InputTokens = 64000
			if legacy.Valid() {
				t.Fatal("writer budget admitted for observation")
			}
		}
	}
	ordinary, err := r.FreezeCall(ref, "write", 8192, llm.ReasoningLow)
	if err != nil || ordinary.InputTokens != 0 || ordinary.InputTokenLimit() != 30000 || p.calls != 0 {
		t.Fatal("non-clip allowance changed or provider called", err)
	}
}
