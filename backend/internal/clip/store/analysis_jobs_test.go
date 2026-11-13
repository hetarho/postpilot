package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// analyzed runs 관찰하기 on the harness's project with the sources named for 다시 관찰하기.
func analyzed(t *testing.T, h *generationHarness, reanalyze ...string) (clip.GenerationQuote, error) {
	t.Helper()
	q, err := h.service.QuoteAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", reanalyze)
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", reanalyze, approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	return q, h.run(t)
}

// CLIP-234: 관찰하기 prices the observation chunks still missing and no writing call, observes
// them, and stops at the stored observations — no storyline, plan, region or upload text, no
// request recorded — after which 스토리라인 먼저 and 바로 만들기 count no analysis.
func TestTheAnalysisJobObservesWhatIsMissingAndStopsAtTheObservations(t *testing.T) {
	h := generationSetup(t)
	generation, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	q, err := analyzed(t, h)
	if err != nil {
		t.Fatal(err)
	}
	if !q.Pricing.Analysis || q.Pricing.Storyline || q.Pricing.ObservationCalls != generation.Pricing.ObservationCalls || q.Pricing.PlanCalls() != 0 || q.Pricing.RenderOnly() || q.Pricing.Reanalyze != "" {
		t.Fatalf("the analysis quote is not the missing observations alone: %+v", q.Pricing)
	}
	if h.planner.observe != q.Pricing.ObservationCalls || h.planner.flows != 0 || h.planner.narrations != 0 || len(h.planner.storylines) != 0 {
		t.Fatalf("the analysis job made observe %d, flow %d, narration %d, storyline %d calls", h.planner.observe, h.planner.flows, h.planner.narrations, len(h.planner.storylines))
	}
	before := h.project
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Analysis == "" || len(analysedSources(t, p)) != len(h.batch.Sources) {
		t.Fatal("the analysis was not saved as the stored observations", p.Analysis)
	}
	if p.EditPlan != "" || p.Storyline != nil || p.Result != nil || p.EditPlanRevision != before.EditPlanRevision {
		t.Fatal("the analysis job wrote more than the observations", p.EditPlan != "", p.Storyline != nil, p.Result != nil)
	}
	if kinds := requestKinds(t, h); len(kinds) != 0 {
		t.Fatal("the analysis job recorded a request", kinds)
	}
	// What was observed is reused: neither writing route nor 관찰하기 again pays for it.
	storyline, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || storyline.Pricing.ObservationCalls != 0 || storyline.Pricing.ReusedChunks != q.Pricing.ObservationCalls {
		t.Fatalf("스토리라인 먼저 analyzes again after 관찰하기: %+v %v", storyline.Pricing, err)
	}
	direct, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || direct.Pricing.ObservationCalls != 0 || direct.Pricing.PlanCalls() == 0 {
		t.Fatalf("바로 만들기 analyzes again after 관찰하기: %+v %v", direct.Pricing, err)
	}
	again, err := h.service.QuoteAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", nil)
	if err != nil || again.Pricing.ObservationCalls != 0 || again.Pricing.MaxCredits != 0 {
		t.Fatalf("nothing left to observe still quoted calls: %+v %v", again.Pricing, err)
	}
	// The storyline built afterwards observes nothing.
	if storyline, err = h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w"); err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(storyline))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	observed := h.planner.observe
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.observe != observed || len(h.planner.storylines) != 1 {
		t.Fatal("the storyline after 관찰하기 observed again", h.planner.observe-observed)
	}
	// An analysis quote cannot start a storyline job, nor the other way round.
	if _, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(again)); err == nil {
		t.Fatal("an analysis approval started a storyline job")
	}
}

// CLIP-234, QUOTA-45: 다시 관찰하기 observes every chunk of the sources it names although the
// recovery holds them, binds that list to the approval, and refuses a source the batch lacks.
func TestReanalysisObservesTheNamedSourcesAgain(t *testing.T) {
	h := generationSetup(t)
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	first, second := h.batch.Sources[0].ID, h.batch.Sources[1].ID
	if _, err := h.service.QuoteAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", []string{"nowhere"}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("a source the batch lacks was quoted for reanalysis", err)
	}
	q, err := h.service.QuoteAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", []string{first})
	if err != nil {
		t.Fatal(err)
	}
	if q.Pricing.ObservationCalls != 2 || q.Pricing.ReusedChunks != 1 || q.Pricing.Reanalyze != first {
		t.Fatalf("reanalysis did not count the named source's chunks as missing: %+v", q.Pricing)
	}
	// The start must carry the list the quote was taken with.
	if _, err := h.service.StartAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", []string{second}, approve(q)); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("another reanalysis list started the approved job", err)
	}
	if _, err := h.service.StartAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", nil, approve(q)); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("an empty reanalysis list started the approved job", err)
	}
	observed := h.planner.observe
	id, err := h.service.StartAnalysis(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", []string{first}, approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.observe-observed != 2 {
		t.Fatal("reanalysis observed", h.planner.observe-observed, "chunks instead of the named source's two")
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || p.Analysis == "" || p.EditPlan != "" || p.Storyline != nil {
		t.Fatal("reanalysis did not stop at the observations", err)
	}
	// Generation routes take no reanalysis list.
	follow, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || follow.Pricing.ObservationCalls != 0 || follow.Pricing.ReusedChunks != 3 {
		t.Fatalf("the reanalysed chunks are not reused afterwards: %+v %v", follow.Pricing, err)
	}
}

// CLIP-87, QUOTA-46: a failed or cancelled analysis keeps the chunks it completed for the next
// quote and stores no observations.
func TestAFailedOrCancelledAnalysisKeepsItsCompletedChunks(t *testing.T) {
	for name, cause := range map[string]error{"failure": llm.ErrBadOutput, "cancellation": context.Canceled} {
		t.Run(name, func(t *testing.T) {
			h := generationSetup(t)
			h.planner.observeErr, h.planner.failObserveAt = cause, 2
			if _, err := analyzed(t, h); err == nil {
				t.Fatal("the second chunk's", name, "did not end the analysis")
			}
			state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
			if err != nil || state == nil || len(state.Chunks) != 1 || state.Plan != "" {
				t.Fatal("the completed chunk was not kept", err)
			}
			p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
			if err != nil || p.Analysis != "" || p.EditPlan != "" || p.Storyline != nil {
				t.Fatal("an unfinished analysis wrote the project", err)
			}
			h.planner.observeErr = nil
			q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
			if err != nil || q.Pricing.ReusedChunks != 1 || q.Pricing.ObservationCalls != 2 {
				t.Fatalf("the next quote does not reuse the kept chunk: %+v %v", q.Pricing, err)
			}
		})
	}
}
