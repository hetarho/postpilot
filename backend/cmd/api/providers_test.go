package main

import (
	"os"
	"strings"
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
	if len(reg.RecommendationSets()) == 0 {
		t.Error("the shipped file declares no recommendation set")
	}
}

// The registry no longer checks at boot that a recommendation set names models that exist —
// the catalog is curated data by then. This is the replacement, and it runs where staleness
// is actually a mistake: every shipped ref must name a model the seed migration inserts, so
// a set that has drifted is caught in CI rather than by the first operator who applies it.
// Note the seed carries NO purpose registrations (change 20): after a fresh cutover, apply
// refuses these very refs until the operator registers them per purpose — that gate lives
// in provider.ApplyRecommendationSet and is tested there.
func TestShippedRecommendationRefsAreSeeded(t *testing.T) {
	seed, err := os.ReadFile("../../internal/platform/db/migrations/0018_catalog_models.sql")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := llm.Load("../../config/providers.yaml", func(string) string { return "" }, adapters, emptyCatalog{},
		llm.Options{Timeout: time.Minute, MaxTokens: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range reg.RecommendationSets() {
		for _, selection := range set.Selections {
			for _, ref := range []llm.ModelRef{selection.Active, selection.CandidateA, selection.CandidateB} {
				if ref.ProviderID != reg.ProviderID() {
					t.Errorf("set %s stage %s: ref %s names an unregistered provider", set.ID, selection.Stage, ref)
				}
				if !strings.Contains(string(seed), "('"+ref.ModelID+"'") {
					t.Errorf("set %s stage %s: %s is not seeded by the catalog migration", set.ID, selection.Stage, ref.ModelID)
				}
			}
		}
	}
}
