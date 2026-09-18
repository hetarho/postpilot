package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func writingPricing(t *testing.T) clip.GenerationPricing {
	t.Helper()
	p := &quotePricing{}
	pricing, err := p.Freeze(context.Background(), llm.ModelRef{ProviderID: "p", ModelID: "o"}, llm.ModelRef{ProviderID: "p", ModelID: "w"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	pricing.CancellationPolicyVersion = clip.CancellationPolicyVersion
	return pricing
}

func TestPricingCountsEachWritingCallAndRefusesAnIncompleteOne(t *testing.T) {
	pricing := writingPricing(t)
	if !pricing.Valid() || pricing.PlanCalls() != 2 || pricing.FlowCalls() != 1 || pricing.NarrationCalls() != 1 || pricing.RenderOnly() {
		t.Fatal("a fresh generation does not price two writing calls", pricing)
	}
	// A narration-only resume pays for the one call it still makes.
	narrationOnly := pricing
	narrationOnly.SkipFlow, narrationOnly.ObservationCalls = true, 0
	if !narrationOnly.Valid() || narrationOnly.PlanCalls() != 1 || narrationOnly.FlowCalls() != 0 || narrationOnly.RenderOnly() {
		t.Fatal("a narration-only resume mispriced", narrationOnly)
	}
	renderOnly := narrationOnly
	renderOnly.SkipNarration = true
	if !renderOnly.Valid() || renderOnly.PlanCalls() != 0 || !renderOnly.RenderOnly() {
		t.Fatal("a render-only resume still pays for writing", renderOnly)
	}
	// Retries are per call, so each writing call carries its own.
	retried := pricing
	retried.Plan.ResponseRetries, retried.Narration.ResponseRetries = 3, 3
	if retried.PlanCalls() != 8 {
		t.Fatal("response corrections are not counted per writing call", retried.PlanCalls())
	}
	for name, broken := range map[string]func(*clip.GenerationPricing){
		"no narration policy at all": func(p *clip.GenerationPricing) { p.Narration = llm.CallPolicy{} },
		"a narration on another model": func(p *clip.GenerationPricing) {
			p.Narration.Ref = llm.ModelRef{ProviderID: "p", ModelID: "other"}
		},
		"a narration budget of its own": func(p *clip.GenerationPricing) { p.Narration.CompletionTokens = 8192 },
		"a kept narration over a rewritten flow": func(p *clip.GenerationPricing) {
			p.SkipNarration, p.SkipFlow = true, false
		},
		"a skipped flow beside new observations": func(p *clip.GenerationPricing) { p.SkipFlow = true },
		"the previous policy version":            func(p *clip.GenerationPricing) { p.Version = 2 },
	} {
		invalid := pricing
		broken(&invalid)
		if invalid.Valid() {
			t.Fatal("pricing admitted " + name)
		}
	}
}

// The generation makes the two calls in order and keeps the flow between them,
// so a failure in the narration resumes on the flow that was already paid for.
func TestGenerationRunsTheFlowThenTheNarration(t *testing.T) {
	h := generationSetup(t)
	h.planner.portableFlow = true
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.flows != 1 || h.planner.narrations != 1 {
		t.Fatal("the writing calls were not the flow and then the narration", h.planner.flows, h.planner.narrations, h.planner.plans)
	}
	if h.planner.stages[0] != "flow" || h.planner.stages[1] != "narrate" {
		t.Fatal("the calls ran in the wrong order", h.planner.stages)
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil || !state.FlowReady || !state.PlanReady {
		t.Fatal("the finished plan was not retained", state, err)
	}
}

func TestANarrationFailureResumesOnTheFlowItAlreadyPaidFor(t *testing.T) {
	h := generationSetup(t)
	h.planner.portableFlow = true
	h.planner.narrateErr = llm.ErrBadOutput
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected narration failure")
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil || !state.FlowReady || state.PlanReady || state.Plan == "" {
		t.Fatal("the written flow was not kept for the resume", state, err)
	}
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	// One writing call, not two: the flow is done and paid for.
	if q.Pricing.RenderOnly() || !q.Pricing.SkipFlow || q.Pricing.PlanCalls() != 1 || q.Pricing.ObservationCalls != 0 || q.Pricing.ReusedChunks != 3 {
		t.Fatalf("the resume repriced the flow: %+v", q.Pricing)
	}
	h.planner.narrateErr = nil
	flows, narrations := h.planner.flows, h.planner.narrations
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal("the resume failed", err)
	}
	if h.planner.flows != flows || h.planner.narrations != narrations+1 {
		t.Fatal("the resume rewrote the flow", h.planner.flows-flows, h.planner.narrations-narrations)
	}
}

// A recovery written before the flow was its own call carries a complete plan
// and no FlowReady at all. It still resumes at rendering.
func TestAPlanWrittenBeforeTheCallsSplitStillResumesAtRender(t *testing.T) {
	h := generationSetup(t)
	// The attempt fails after its plan is durable: the render it used to fail in
	// is its own job now (CLIP-151).
	h.media.cleanupErr = errors.New("workspace cleanup failed")
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected attempt failure")
	}
	if _, err := h.db.Writer.Exec("UPDATE clip_recovery_states SET state_json=json_set(state_json,'$.FlowReady',json('false')) WHERE project_id=?", h.project.ID); err != nil {
		t.Fatal(err)
	}
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || !q.Pricing.RenderOnly() || q.Pricing.PlanCalls() != 0 || q.Pricing.MaxCredits != 0 {
		t.Fatalf("an older complete plan was rewritten: %+v %v", q.Pricing, err)
	}
	h.media.cleanupErr = nil
	flows := h.planner.flows
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal("the render-only resume failed", err)
	}
	if h.planner.flows != flows {
		t.Fatal("the resume paid for writing again")
	}
}
