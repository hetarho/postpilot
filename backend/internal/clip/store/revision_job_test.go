package store_test

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// revisionReady runs one generation so the project holds a plan and the
// observations a revision is bound to, then renders it, since a generation
// stops at the plan and a revision is about leaving a RENDERED result stale
// (CLIP-151, CLIP-132).
func revisionReady(t *testing.T) *generationHarness {
	t.Helper()
	h := generationSetup(t)
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	h.render(t)
	return h
}

func startRevision(t *testing.T, h *generationHarness, request, target string) string {
	t.Helper()
	q, err := h.service.QuoteRevision(t.Context(), "alice", h.project.ID, request, target, 0, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	// One writing call for a narration target, two where the footage moves or may move.
	want := 2
	if target == clip.RevisionNarration {
		want = 1
	}
	if q.Pricing.ObservationCalls != 0 || q.Pricing.PlanCalls() != want {
		t.Fatalf("a %s revision priced %d writing calls and %d observations", target, q.Pricing.PlanCalls(), q.Pricing.ObservationCalls)
	}
	id, err := h.service.StartRevision(t.Context(), "alice", h.project.ID, request, target, 0, "p/o", "p/w",
		clip.QuoteApproval{CancellationPolicyVersion: clip.CancellationPolicyVersion, QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits})
	if err != nil {
		t.Fatal(err)
	}
	// The frozen policy the fake consumes belongs to THIS job.
	h.planner.id = id
	return id
}

func TestARevisionRewritesTheSavedPlanAndLeavesTheResultStale(t *testing.T) {
	h := revisionReady(t)
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || before.Result == nil || before.EditPlanRevision != before.RenderedPlanRevision {
		t.Fatal("the fixture has no rendered result to revise", err)
	}
	writes := h.planner.flows + h.planner.narrations
	startRevision(t, h, "고기 장면을 먼저 보여줘", clip.RevisionFlow)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.planner.revisions) != 1 || h.planner.revisions[0].Request != "고기 장면을 먼저 보여줘" {
		t.Fatal("the owner's words did not reach the writer", h.planner.revisions)
	}
	// The plan the request was answered from is the plan the owner had.
	if h.planner.revisions[0].Current.DurationMS != before.EditPlanRevision*0+h.renderer.plan.DurationMS {
		t.Fatal("the revision answered from another plan")
	}
	if after.EditPlanRevision != before.EditPlanRevision+1 || after.RenderedPlanRevision != before.RenderedPlanRevision {
		t.Fatal("the result is not stale", after.EditPlanRevision, after.RenderedPlanRevision)
	}
	if after.EditPlan == before.EditPlan {
		t.Fatal("the saved plan was not rewritten")
	}
	if after.Result == nil || after.Result.Key != before.Result.Key {
		t.Fatal("the existing result was touched")
	}
	if after.Analysis != before.Analysis {
		t.Fatal("a revision rewrote the recorded observations")
	}
	if h.planner.flows+h.planner.narrations != writes+2 {
		t.Fatal("a flow revision did not make the two writing calls it charged for")
	}
	// A quote taken against the old plan is refused now that the plan moved.
	q, err := h.service.QuoteRevision(t.Context(), "alice", h.project.ID, "다시", clip.RevisionNarration, 0, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	stale := clip.RevisionInputDigest(before, "다시", clip.RevisionNarration, 0, "p/w", q.Pricing)
	if q.InputDigest == stale {
		t.Fatal("the quote did not bind the plan revision it was taken against")
	}
}

func TestANarrationRevisionChargesOneWritingCallAndNoObservation(t *testing.T) {
	h := revisionReady(t)
	observed := h.planner.observe
	startRevision(t, h, "가격을 말해줘", clip.RevisionNarration)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.observe != observed {
		t.Fatal("a revision re-observed the footage")
	}
	if len(h.planner.revisions) != 1 || h.planner.revisions[0].Target != clip.RevisionNarration {
		t.Fatal("the target did not reach the writer", h.planner.revisions)
	}
}

func TestARefusedRevisionLeavesThePlanAndTheResultAlone(t *testing.T) {
	h := revisionReady(t)
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A request nobody wrote, a target nobody declared, and a finalized project
	// are refused before anything is charged.
	for name, run := range map[string]func() error{
		"no request": func() error {
			_, err := h.service.QuoteRevision(t.Context(), "alice", h.project.ID, "  ", clip.RevisionFlow, 0, "p/o", "p/w")
			return err
		},
		"an unknown target": func() error {
			_, err := h.service.QuoteRevision(t.Context(), "alice", h.project.ID, "고쳐줘", "everything", 0, "p/o", "p/w")
			return err
		},
		"another owner": func() error {
			_, err := h.service.QuoteRevision(t.Context(), "bob", h.project.ID, "고쳐줘", clip.RevisionFlow, 0, "p/o", "p/w")
			return err
		},
	} {
		if err := run(); err == nil {
			t.Fatal("a revision was quoted with " + name)
		}
	}
	// A failing writer leaves everything as it was.
	h.planner.errorPlan = errors.New("writer refused")
	startRevision(t, h, "고기 장면을 먼저 보여줘", clip.RevisionFlow)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected writer failure")
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EditPlan != before.EditPlan || after.EditPlanRevision != before.EditPlanRevision {
		t.Fatal("a failed revision changed the saved plan")
	}
	if after.Result == nil || after.Result.Key != before.Result.Key {
		t.Fatal("a failed revision touched the result")
	}
}

func TestARevisionIsOneJobAtATimeLikeEveryOtherClipJob(t *testing.T) {
	h := revisionReady(t)
	startRevision(t, h, "고기 장면을 먼저 보여줘", clip.RevisionFlow)
	if _, err := h.service.QuoteRevision(t.Context(), "alice", h.project.ID, "또", clip.RevisionFlow, 0, "p/o", "p/w"); !errors.Is(err, clip.ErrBusy) {
		t.Fatal("a second revision was quoted while one was running", err)
	}
	if _, err := h.service.StartRender(t.Context(), "alice", h.project.ID, h.batch.ID, 1, clip.RenderServer); err == nil {
		t.Fatal("a render started beside a running revision")
	}
}

// An auto request is approved at two writing calls and, answered as narration-only,
// runs and is charged for one; the record names it auto (CLIP-131, CLIP-132, CLIP-133).
func TestAnAutoRevisionRunsOnlyTheCallItsRoutingChose(t *testing.T) {
	h := revisionReady(t)
	writes := h.planner.flows + h.planner.narrations
	stages := len(h.planner.stages)
	startRevision(t, h, "가격을 말해줘", clip.RevisionAuto)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.flows+h.planner.narrations != writes+1 || len(h.planner.revisions) != 1 || h.planner.revisions[0].Target != clip.RevisionAuto {
		t.Fatal("a narration-only auto revision did not make exactly one writing call", h.planner.flows+h.planner.narrations-writes)
	}
	if got := h.planner.stages[stages:]; len(got) != 1 || got[0] != "narrate" {
		t.Fatal("the auto revision ran other stages", got)
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || len(p.Requests) == 0 || p.Requests[0].Kind != "revision:auto" || p.Requests[0].Body != "가격을 말해줘" {
		t.Fatal("the auto request was not recorded under its own kind", p.Requests, err)
	}
	// A scene the plan does not hold is refused before any quote is saved.
	if _, err := h.service.QuoteRevision(t.Context(), "alice", h.project.ID, "이 장면만", clip.RevisionAuto, 99, "p/o", "p/w"); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("a scene outside the plan was quoted", err)
	}
}
