package main

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// emptyCatalog is what a fresh database offers. The shipped file must load against it: the
// connection is configuration, the models are not.
type emptyCatalog struct{}

func (emptyCatalog) Models() []llm.SourceModel { return nil }

func (emptyCatalog) Lookup(string) (llm.SourceModel, bool) { return llm.SourceModel{}, false }

// The connection that ships in the image must load with the adapters this binary wires —
// a typo in config/providers.yaml would otherwise be found by the deploy's health gate.
func TestShippedProvidersConfigLoads(t *testing.T) {
	noKeys := func(string) string { return "" }
	reg, err := llm.Load("../../config/providers.yaml", noKeys, adapters, emptyCatalog{}, llm.Options{Timeout: time.Minute, MaxTokens: 1})
	if err != nil {
		t.Fatal(err)
	}
	if reg.ProviderID() == "" || reg.BaseURL() == "" {
		t.Fatalf("provider id %q / base url %q", reg.ProviderID(), reg.BaseURL())
	}
}
