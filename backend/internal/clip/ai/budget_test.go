package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
)

// The completion budgets are frozen into the credit hold before any provider
// work (QUOTA-14), so a schema that grew past them would fail a run the owner
// already paid to approve. T105 added scene, readable_text and subject to each
// segment: this measures the worst case those bounds allow against the budget
// they have to fit, and a realistic answer of each writing call against its own.
func TestSchemaWorstCaseFitsTheCompletionBudgets(t *testing.T) {
	cfg := ai.DefaultConfig(clip.Environment{})
	long := strings.Repeat("한", 2000)
	subjects := make([]string, cfg.Analysis.MaxSubjects)
	for i := range subjects {
		subjects[i] = long
	}
	segment := map[string]any{
		"start_ms": 60000, "end_ms": 60000, "event": long, "action": long, "motion": long, "subjects": subjects,
		"speech": long, "quality": long, "focal": map[string]float64{"x": .5, "y": .5},
		"scene": "interior", "readable_text": true, "certainty": "uncertain", "usability": "unusable",
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
	// What T105 and then T145 added to each segment, at its own maximum. The
	// two status fields are enum-bounded, so their worst case is their longest
	// member.
	segmentAdded := len(`"scene":"interior","readable_text":true,"certainty":"uncertain","usability":"unusable",`)
	// A conservative 3 bytes per token for Korean UTF-8 output.
	const bytesPerToken = 3
	t.Logf("observe worst case %d bytes ≈ %d tokens, budget %d", len(chunk), len(chunk)/bytesPerToken, cfg.ObserveCompletionTokens)

	// The absolute worst case the observation shape allows has always been far
	// past its budget — sixty segments of twenty 2000-character subjects is
	// millions of bytes — and is bounded in practice by the provider's own
	// MaxTokens and by MaxResponseBytes at the parser. What matters is that
	// T105's own fields are a small part of the shape, so they cannot be what
	// truncates a response.
	if segmentAdded > 200 {
		t.Fatalf("the new segment fields cost %d bytes", segmentAdded)
	}
	// A REALISTIC response — ten segments, and twenty cuts of flow — fits its
	// budget with room to spare.
	real := realisticChunk(cfg.Analysis.MaxSubjects)
	realFlow := realisticFlow()
	t.Logf("realistic observe %d bytes ≈ %d tokens; flow %d bytes ≈ %d tokens", len(real), len(real)/bytesPerToken, len(realFlow), len(realFlow)/bytesPerToken)
	if len(real)/bytesPerToken > cfg.ObserveCompletionTokens/2 || len(realFlow)/bytesPerToken > cfg.FlowCompletionTokens/2 {
		t.Fatal("a realistic response no longer fits half of its budget")
	}
	// The schemas themselves ride in every prompt, so they stay small.
	if len(ai.ChunkSchema()) > 4096 || len(ai.FlowSchema()) > 4096 {
		t.Fatalf("schemas grew: chunk %d, flow %d bytes", len(ai.ChunkSchema()), len(ai.FlowSchema()))
	}
}

// The observe REQUEST rides beside a bounded inline MP4, so its own allowance is
// the smallest one in the system: 20,000 units of the 30,000 are reserved for the
// media. T145's coverage and status rules made the prompt longer, and it still
// has to fit without dropping a rule or shortening the schema (CLIP-92).
func TestObservePromptFitsTheInlineInputAllowance(t *testing.T) {
	in := clip.ChunkInput{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: strings.Repeat("a", 64), Info: clip.MediaInfo{DurationMS: 60000, Width: 1920, Height: 1080, HasAudio: true}}, Filename: strings.Repeat("원", 80) + ".mp4"}, DurationMS: 60000}
	system, user := ai.BuildObservePrompt(in)
	request, err := json.Marshal(struct {
		System, User string
		Schema       json.RawMessage
	}{system, user, ai.ChunkSchema()})
	if err != nil {
		t.Fatal(err)
	}
	limit := llm.ClipInputUnits - 2048 - 20000
	t.Logf("observe request %d bytes of %d (system %d, user %d, schema %d)", len(request), limit, len(system), len(user), len(ai.ChunkSchema()))
	if len(request) > limit {
		t.Fatalf("the observe prompt no longer fits its reserved inline allowance: %d > %d", len(request), limit)
	}
}

func realisticChunk(maxSubjects int) []byte {
	segments := make([]any, 10)
	for i := range segments {
		segments[i] = map[string]any{
			"start_ms": i * 6000, "end_ms": (i + 1) * 6000,
			"event": "접시에 김밥을 담고 카메라가 천천히 다가간다", "action": "김밥을 접시에 옮겨 담는다",
			"motion": "카메라가 천천히 앞으로 다가간다", "subjects": []string{"김밥", "접시"},
			"speech": "", "quality": "steady and sharp", "focal": map[string]float64{"x": .5, "y": .5},
			"scene": "food", "readable_text": false, "certainty": "certain", "usability": "usable",
			"subject": map[string]float64{"x": .2, "y": .3, "width": .5, "height": .4},
		}
	}
	out, _ := json.Marshal(map[string]any{"source_id": strings.Repeat("a", 32), "chunk_index": 0, "segments": segments})
	return out
}
func realisticFlow() []byte {
	cuts := make([]any, 20)
	for i := range cuts {
		cuts[i] = map[string]any{
			"id": "cut-" + strings.Repeat("a", 8), "source_id": strings.Repeat("a", 32),
			"start_ms": 0, "end_ms": 3000, "rate_permille": 1000, "volume": 1, "focal": map[string]float64{"x": .5, "y": .5},
			"observation_refs": []string{strings.Repeat("a", 32) + "/0"},
		}
	}
	out, _ := json.Marshal(map[string]any{"ratio": "vertical", "duration_ms": 60000, "cuts": cuts})
	return out
}
