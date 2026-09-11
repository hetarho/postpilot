package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/platform/config"
)

// The completion budgets are frozen into the credit hold before any provider
// work (QUOTA-14), so a schema that grew past them would fail a run the owner
// already paid to approve. T105 added scene, readable_text and subject to each
// segment and short_text, keyword, chips and hook to the plan: this measures the
// worst case those bounds allow against the budgets they have to fit.
func TestSchemaWorstCaseFitsTheCompletionBudgets(t *testing.T) {
	cfg := config.ClipAI(&config.Config{})
	long := strings.Repeat("한", 2000)
	subjects := make([]string, cfg.Analysis.MaxSubjects)
	for i := range subjects {
		subjects[i] = long
	}
	segment := map[string]any{
		"start_ms": 60000, "end_ms": 60000, "event": long, "subjects": subjects,
		"speech": long, "quality": long, "focal": map[string]float64{"x": .5, "y": .5},
		"scene": "interior", "readable_text": true,
		"subject": map[string]float64{"x": .1, "y": .1, "width": .1, "height": .1},
	}
	segments := make([]any, cfg.Analysis.MaxSegments)
	for i := range segments {
		segments[i] = segment
	}
	chunk, err := json.Marshal(map[string]any{"source_id": strings.Repeat("a", 64), "chunk_index": 9, "segments": segments})
	if err != nil {
		t.Fatal(err)
	}
	// short_text, the keyword and the hook carry rules of their own (14
	// characters, one word, two lines of nine), so the contract bounds them at
	// 40 rather than at the caption's 500: a plan may not double its own output.
	short := strings.Repeat("한", 40)
	cut := map[string]any{
		"id": strings.Repeat("a", cfg.MaxCutIDRunes), "source_id": strings.Repeat("a", 64),
		"start_ms": 1, "end_ms": 2, "volume": 1, "focal": map[string]float64{"x": .5, "y": .5},
		"chips":   []string{"위치", "가격"},
		"caption": map[string]any{"text": strings.Repeat("한", cfg.Render.MaxCopyRunes), "start_ms": 1, "end_ms": 2, "short_text": short, "keyword": short},
	}
	cuts := make([]any, cfg.Render.MaxCuts)
	for i := range cuts {
		cuts[i] = cut
	}
	plan, err := json.Marshal(map[string]any{"ratio": "vertical", "duration_ms": 90000, "hook": short, "cuts": cuts})
	if err != nil {
		t.Fatal(err)
	}
	// What T105 added to each shape, at its own maximum.
	segmentAdded := len(`"scene":"interior","readable_text":true,`)
	cutAdded := len(`"chips":["위치","가격"],"short_text":"","keyword":"",`) + 2*len(short)
	// A conservative 3 bytes per token for Korean UTF-8 output.
	const bytesPerToken = 3
	t.Logf("observe worst case %d bytes ≈ %d tokens, budget %d", len(chunk), len(chunk)/bytesPerToken, cfg.ObserveCompletionTokens)
	t.Logf("plan worst case %d bytes ≈ %d tokens, budget %d", len(plan), len(plan)/bytesPerToken, cfg.PlanCompletionTokens)

	// The absolute worst case both shapes allow has always been far past every
	// budget — sixty segments of twenty 2000-character subjects is millions of
	// bytes — and is bounded in practice by the provider's own MaxTokens and by
	// MaxResponseBytes at the parser. What matters is that T105's own fields are
	// a small part of each shape, so they cannot be what truncates a response.
	if segmentAdded > 100 {
		t.Fatalf("the new segment fields cost %d bytes", segmentAdded)
	}
	// Against the most cuts CDS-37 actually allows — the longest clip divided by
	// the shortest cut — not the contract's 100, which would mean 0.9-second
	// cuts the design system refuses.
	cutCeiling := cfg.Render.MaxDurationMS / int(design.Timing.CutMinS*1000)
	if cutAdded*cutCeiling > cfg.PlanCompletionTokens*bytesPerToken/4 {
		t.Fatalf("the new cut fields cost %d bytes across %d cuts, a quarter of the plan budget", cutAdded*cutCeiling, cutCeiling)
	}
	// A REALISTIC response — ten segments and twenty cuts written to CDS's own
	// character limits — fits both budgets with room to spare.
	real := realisticChunk(cfg.Analysis.MaxSubjects)
	realPlan := realisticPlan()
	t.Logf("realistic observe %d bytes ≈ %d tokens; plan %d bytes ≈ %d tokens", len(real), len(real)/bytesPerToken, len(realPlan), len(realPlan)/bytesPerToken)
	if len(real)/bytesPerToken > cfg.ObserveCompletionTokens/2 || len(realPlan)/bytesPerToken > cfg.PlanCompletionTokens/2 {
		t.Fatal("a realistic response no longer fits half of its budget")
	}
	// The schemas themselves ride in every prompt, so they stay small.
	if len(ai.ChunkSchema()) > 4096 || len(ai.PlanSchema()) > 4096 {
		t.Fatalf("schemas grew: chunk %d, plan %d bytes", len(ai.ChunkSchema()), len(ai.PlanSchema()))
	}
}

func realisticChunk(maxSubjects int) []byte {
	segments := make([]any, 10)
	for i := range segments {
		segments[i] = map[string]any{
			"start_ms": i * 6000, "end_ms": (i + 1) * 6000,
			"event": "접시에 김밥을 담고 카메라가 천천히 다가간다", "subjects": []string{"김밥", "접시"},
			"speech": "", "quality": "steady and sharp", "focal": map[string]float64{"x": .5, "y": .5},
			"scene": "food", "readable_text": false,
			"subject": map[string]float64{"x": .2, "y": .3, "width": .5, "height": .4},
		}
	}
	out, _ := json.Marshal(map[string]any{"source_id": strings.Repeat("a", 32), "chunk_index": 0, "segments": segments})
	return out
}
func realisticPlan() []byte {
	cuts := make([]any, 20)
	for i := range cuts {
		cuts[i] = map[string]any{
			"id": "cut-" + strings.Repeat("a", 8), "source_id": strings.Repeat("a", 32),
			"start_ms": 0, "end_ms": 3000, "volume": 1, "focal": map[string]float64{"x": .5, "y": .5},
			"chips":   []string{"위치"},
			"caption": map[string]any{"text": "연남동에서 제일 조용한 자리", "start_ms": 0, "end_ms": 3000, "short_text": "조용한 자리", "keyword": ""},
		}
	}
	out, _ := json.Marshal(map[string]any{"ratio": "vertical", "duration_ms": 60000, "hook": "여기 왜 유명할까", "cuts": cuts})
	return out
}
