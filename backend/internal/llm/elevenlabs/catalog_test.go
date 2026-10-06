package elevenlabs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestAuthenticatedSpeechCatalogCachesWithoutInventingAnAccountPair(t *testing.T) {
	var calls atomic.Int64
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/v1/models" || r.Header.Get("xi-api-key") != "private-test-key" {
			t.Errorf("request: %s %s %v", r.Method, r.URL, r.Header)
		}
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"model_id":"synth-v1","name":"Korean synth","can_do_text_to_speech":true,"can_use_style":true,"can_use_speaker_boost":true,"maximum_text_length_per_request":5000,"max_characters_request_free_user":1,"languages":[{"language_id":"ko","name":"Korean"}],"token_cost_factor":0.500001,"model_rates":{"character_cost_multiplier":1.25,"cost_discount_multiplier":0.75}}]`))
	}))
	defer server.Close()
	p, err := New(llm.SpeechAdapterConfig{ProviderID: "custom-connection", BaseURL: server.URL, APIKey: "private-test-key"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	c, err := p.ReadSpeechCatalog(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Models) != 3 || c.CheckedAt.IsZero() {
		t.Fatalf("catalog: %+v", c)
	}
	if len(c.ConnectionScope) != 64 || strings.Contains(c.ConnectionScope, "private-test-key") {
		t.Fatal("private scope missing")
	}
	rotated, err := New(llm.SpeechAdapterConfig{ProviderID: "custom-connection", BaseURL: server.URL, APIKey: "rotated-key"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if p.connectionScope() == rotated.connectionScope() {
		t.Fatal("rotated supplier key reused old scope")
	}
	m := c.Models[0]
	if m.Ref.ProviderID != "custom-connection" || !m.Korean || !m.Synthesis || m.Design || m.MaxText != 5000 || m.TokenCostFactor != "0.500001" || m.CharacterCostMultiplier != "1.25" || m.CostDiscountMultiplier != "0.75" {
		t.Fatalf("metadata: %+v", m)
	}
	for _, model := range c.Models[1:] {
		if !model.Design || model.Synthesis || !slices.Contains(designModels, model.Ref.ModelID) {
			t.Fatalf("invented candidate: %+v", model)
		}
	}
	c.Models[0].Korean = false
	copy, err := p.ReadSpeechCatalog(context.Background(), false)
	if err != nil || !copy.Models[0].Korean || calls.Load() != 1 {
		t.Fatalf("cache mutated: %+v %v %d", copy, err, calls.Load())
	}
	fail.Store(true)
	if _, err := p.ReadSpeechCatalog(context.Background(), true); err == nil {
		t.Fatal("refresh failure swallowed")
	}
	copy, err = p.ReadSpeechCatalog(context.Background(), false)
	if err != nil || !copy.Models[0].Korean {
		t.Fatal("outage replaced prior successful metadata")
	}
}

func TestSpeechCatalogMissingKeyNeverCallsSupplier(t *testing.T) {
	p, err := New(llm.SpeechAdapterConfig{ProviderID: "speech"}, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadSpeechCatalog(context.Background(), true); err != llm.ErrProviderDisabled {
		t.Fatalf("missing-key catalog: %v", err)
	}
}

func TestSpeechBillingRulesRefuseAmbiguousFactorsAndPreserveCachedBounds(t *testing.T) {
	for _, tc := range []struct {
		factor, character, discount string
		qualified                   bool
	}{
		{"1", "1", "1", true}, {"1.00", "1e0", "1.0", true}, {"0.5", "1", "1", false}, {"1", "1.25", "1", false}, {"1", "1", "0.75", false}, {"null", "1", "1", false},
	} {
		t.Run(tc.factor+"-"+tc.character+"-"+tc.discount, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `[{"model_id":"synth","name":"Synth","can_do_text_to_speech":true,"maximum_text_length_per_request":5000,"token_cost_factor":%s,"model_rates":{"character_cost_multiplier":%s,"cost_discount_multiplier":%s}}]`, tc.factor, tc.character, tc.discount)
			}))
			defer server.Close()
			p, err := New(llm.SpeechAdapterConfig{ProviderID: "speech", BaseURL: server.URL, APIKey: "fixture"}, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			c, err := p.ReadSpeechCatalog(t.Context(), false)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, r := range c.BillingRules {
				if r.Operation == "speech" {
					found = true
				}
			}
			if found != tc.qualified {
				t.Fatal("unqualified rate priced", c.BillingRules)
			}
			c.BillingRules[0].UnitsPerInputCharacter = "999"
			copy, err := p.ReadSpeechCatalog(t.Context(), false)
			if err != nil || copy.BillingRules[0].UnitsPerInputCharacter == "999" {
				t.Fatal("cached bounds mutated", err)
			}
		})
	}
}
