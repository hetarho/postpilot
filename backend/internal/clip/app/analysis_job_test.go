package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// CLIP-234: 관찰하기 prices its observations and no writing call; it is not a render-only
// resume, and the admission's refreeze prices it in the same shape and binds the same list.
func TestAnAnalysisIsPricedAsItsObservationsAlone(t *testing.T) {
	s := &GenerationService{pricing: NewPricing(&fakeFreezer{}, Budgets{ObserveCompletionTokens: 8192, FlowCompletionTokens: 4096, NarrationCompletionTokens: 2048}, &changingRates{rate: testQuoteRate})}
	observe := llm.ModelRef{ProviderID: "p", ModelID: "o"}
	write := llm.ModelRef{ProviderID: "p", ModelID: "w"}
	observations, err := s.freezeWork(context.Background(), observe, write, 2, true, true, DefaultQuoteRetries)
	if err != nil {
		t.Fatal(err)
	}
	analysis := observations
	analysis.SkipFlow, analysis.SkipNarration, analysis.Analysis, analysis.Reanalyze = false, false, true, "a,b"
	if analysis.PlanCalls() != 0 || analysis.FlowCalls() != 0 || analysis.NarrationCalls() != 0 || analysis.RenderOnly() {
		t.Fatalf("an analysis counts writing calls or reads as render-only: %+v", analysis)
	}
	credits, err := QuoteCredits(analysis, 2, DefaultQuoteRetries)
	if err != nil || credits != analysis.MaxCredits {
		t.Fatal("an analysis is priced beyond its observations", credits, analysis.MaxCredits, err)
	}
	current, err := s.refreeze(context.Background(), analysis, 2)
	if err != nil || !reflect.DeepEqual(current, analysis) {
		t.Fatalf("the admission priced the analysis in another shape: %+v %v", current, err)
	}
	if !reflect.DeepEqual(analysis.ReanalyzedSources(), []string{"a", "b"}) || (clip.GenerationPricing{}).ReanalyzedSources() != nil {
		t.Fatal("the reanalysis list does not read back", analysis.ReanalyzedSources())
	}
	storyline := analysis
	storyline.Storyline = true
	generation := observations
	generation.SkipFlow, generation.SkipNarration, generation.Reanalyze = false, false, "a"
	for name, invalid := range map[string]clip.GenerationPricing{"analysis and storyline": storyline, "a reanalysis list without an analysis": generation} {
		invalid.CancellationPolicyVersion = clip.CancellationPolicyVersion
		if invalid.Valid() {
			t.Fatal(name, "is a valid pricing")
		}
	}
}

// CLIP-234: 다시 관찰하기 drops the named sources' kept chunks, keeps the batch's order, and
// refuses a source the batch does not hold.
func TestReanalysisDropsOnlyTheNamedSourcesChunks(t *testing.T) {
	b := clip.SourceBatch{Sources: []clip.SourceLease{{ID: "a"}, {ID: "b"}, {ID: "c"}}}
	list, err := reanalysisList(b, []string{"c", "a", "a"})
	if err != nil || !reflect.DeepEqual(list, []string{"a", "c"}) {
		t.Fatal("the reanalysis list is not the batch's order, each once", list, err)
	}
	if _, err := reanalysisList(b, []string{"z"}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("a source outside the batch was accepted", err)
	}
	if list, err := reanalysisList(b, nil); err != nil || list != nil {
		t.Fatal("관찰하기 named sources", list, err)
	}
	r := clip.RecoveryState{Chunks: []clip.ChunkAnalysis{{SourceID: "a"}, {SourceID: "b", Index: 0}, {SourceID: "b", Index: 1}, {SourceID: "c"}}}
	kept := withoutReanalyzed(r, []string{"b"})
	if len(kept.Chunks) != 2 || kept.Chunks[0].SourceID != "a" || kept.Chunks[1].SourceID != "c" || len(r.Chunks) != 4 {
		t.Fatal("reanalysis dropped the wrong chunks", kept.Chunks)
	}
}
