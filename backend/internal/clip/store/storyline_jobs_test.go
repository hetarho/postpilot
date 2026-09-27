package store_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func approve(q clip.GenerationQuote) clip.QuoteApproval {
	return clip.QuoteApproval{CancellationPolicyVersion: clip.CancellationPolicyVersion, QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits}
}

// storylineWritten runs 스토리라인 먼저 on a fresh project, so it holds an analysis and a
// storyline and no plan.
func storylineWritten(t *testing.T) *generationHarness {
	t.Helper()
	h := generationSetup(t)
	q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	return h
}

func requestKinds(t *testing.T, h *generationHarness) []string {
	t.Helper()
	requests, err := h.store.ListProjectRequests(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, r := range requests {
		out = append(out, r.Kind+":"+r.Body)
	}
	return out
}

// CLIP-177: 스토리라인 먼저 prices the analysis still missing and ONE writing call, and its job
// analyzes, makes that one storyline call and saves the storyline alone — no flow, no
// narration and no plan.
func TestTheStorylineCallAnalyzesWhatIsMissingAndStopsAtTheStoryline(t *testing.T) {
	h := generationSetup(t)
	generation, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if !q.Pricing.Storyline || q.Pricing.ObservationCalls != generation.Pricing.ObservationCalls || q.Pricing.NarrationCalls() != 0 || q.Pricing.PlanCalls() != 1+q.Pricing.Plan.ResponseRetries {
		t.Fatalf("the storyline quote is not the missing analysis and one writing call: %+v", q.Pricing)
	}
	if q.Pricing.MaxCredits >= generation.Pricing.MaxCredits {
		t.Fatal("one writing call cost as much as two", q.Pricing.MaxCredits, generation.Pricing.MaxCredits)
	}
	id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.flows != 0 || h.planner.narrations != 0 || len(h.planner.storylines) != 1 {
		t.Fatalf("the storyline job made flow %d, narration %d, storyline %d calls", h.planner.flows, h.planner.narrations, len(h.planner.storylines))
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.EditPlan != "" || p.Analysis == "" {
		t.Fatal("the storyline job wrote a plan, or left no analysis", p.EditPlan != "", p.Analysis == "")
	}
	if p.Storyline == nil || len(p.Storyline.Paragraphs) != 1 || p.Storyline.EditedByHand || !reflect.DeepEqual(p.Storyline.MadeWithSources, analysedSources(t, p)) {
		t.Fatalf("the storyline was not saved as written: %+v", p.Storyline)
	}
	// A generation's quote cannot start the storyline job, nor the other way round.
	if _, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(generation)); err == nil {
		t.Fatal("a generation's approval started a storyline job")
	}
}

// CLIP-181: the storyline request is one writing call on the stored observations; it rewrites
// the storyline as asked, is recorded as a storyline request, and an edit between the quote and
// the start refuses the approval (QUOTA-45).
func TestAStorylineRequestRewritesTheStorylineInOneWritingCall(t *testing.T) {
	h := storylineWritten(t)
	observed := h.planner.observe
	q, err := h.service.QuoteStorylineRevision(t.Context(), "alice", h.project.ID, "가게 소개를 먼저", "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if !q.Pricing.Storyline || q.Pricing.ObservationCalls != 0 || q.Pricing.PlanCalls() != 1+q.Pricing.Plan.ResponseRetries {
		t.Fatalf("a storyline request is not one writing call: %+v", q.Pricing)
	}
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	// An owner edit after the quote moves the storyline the request was priced against.
	edit := []clip.StorylineParagraph{{Text: "손으로 고친 문단", ObservationIDs: before.Storyline.Paragraphs[0].ObservationIDs}}
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Storyline: &edit}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.StartStorylineRevision(t.Context(), "alice", h.project.ID, "가게 소개를 먼저", "p/o", "p/w", approve(q)); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("a storyline edited after its quote was rewritten", err)
	}
	q, err = h.service.QuoteStorylineRevision(t.Context(), "alice", h.project.ID, "가게 소개를 먼저", "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartStorylineRevision(t.Context(), "alice", h.project.ID, "가게 소개를 먼저", "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	h.planner.storylineAnswer = &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "가게 소개로 시작해요."}}}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.observe != observed {
		t.Fatal("a storyline request observed the footage again")
	}
	last := h.planner.storylines[len(h.planner.storylines)-1]
	if last.Request != "가게 소개를 먼저" || last.Current == nil || last.Current.Paragraphs[0].Text != "손으로 고친 문단" {
		t.Fatalf("the request did not reach the writer with the storyline it rewrites: %+v", last)
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Storyline == nil || p.Storyline.Paragraphs[0].Text != "가게 소개로 시작해요." || p.Storyline.EditedByHand {
		t.Fatalf("the rewritten storyline was not saved: %+v", p.Storyline)
	}
	if !slices.Contains(requestKinds(t, h), clip.RequestStoryline+":가게 소개를 먼저") {
		t.Fatal("the request was not recorded as a storyline request", requestKinds(t, h))
	}
}

// CLIP-178, CDS-37: 이 스토리로 만들기 builds the flow along the stored storyline — the writer is
// handed it — the narration follows it, and the stored storyline is left exactly as it was.
func TestBuildingFromTheStorylineFollowsItAndKeepsIt(t *testing.T) {
	h := storylineWritten(t)
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	q, err := h.service.QuoteFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if q.Pricing.StorylineDigest != before.Storyline.Digest() {
		t.Fatal("the quote did not bind the storyline it builds along")
	}
	id, err := h.service.StartFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.planner.input.FollowStoryline, before.Storyline) {
		t.Fatal("the flow was not built along the storyline", h.planner.input.FollowStoryline)
	}
	if last := h.planner.narrated[len(h.planner.narrated)-1]; !reflect.DeepEqual(last, before.Storyline) {
		t.Fatal("the narration did not follow the storyline", last)
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EditPlan == "" || !reflect.DeepEqual(after.Storyline, before.Storyline) {
		t.Fatal("the build wrote no plan or changed the stored storyline", after.Storyline)
	}
}

// A clip with no storyline has nothing to build from.
func TestBuildingFromAMissingStorylineIsRefused(t *testing.T) {
	h := generationSetup(t)
	if _, err := h.service.QuoteFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w"); !errors.Is(err, clip.ErrStorylineMissing) {
		t.Fatal("a build from no storyline was quoted", err)
	}
	if _, err := h.service.QuoteStorylineRevision(t.Context(), "alice", h.project.ID, "다시", "p/o", "p/w"); !errors.Is(err, clip.ErrStorylineMissing) {
		t.Fatal("a request about no storyline was quoted", err)
	}
}

// CLIP-178: the owner's edit keeps the storyline's shape and is marked edited by hand.
func TestTheOwnerEditsTheStorylineWithinItsShape(t *testing.T) {
	h := storylineWritten(t)
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	scene := p.Storyline.Paragraphs[0].ObservationIDs[0]
	other := clip.ObservationID(p.Storyline.MadeWithSources[len(p.Storyline.MadeWithSources)-1], 0)
	for name, edit := range map[string][]clip.StorylineParagraph{
		"another paragraph count": {{Text: "하나"}, {Text: "둘"}},
		"an unknown scene":        {{Text: "앞", ObservationIDs: []string{"nowhere/0"}}},
		"a scene named twice":     {{Text: "앞", ObservationIDs: []string{scene, scene}}},
		"an empty paragraph":      {{Text: "  "}},
	} {
		if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Storyline: &edit}); !errors.Is(err, clip.ErrStorylineInvalid) {
			t.Fatalf("%s was saved: %v", name, err)
		}
	}
	edit := []clip.StorylineParagraph{{Text: "손으로 고친 문단", ObservationIDs: []string{other}}}
	saved, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Storyline: &edit})
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Storyline.EditedByHand || saved.Storyline.Paragraphs[0].Text != "손으로 고친 문단" || !reflect.DeepEqual(saved.Storyline.MadeWithSources, p.Storyline.MadeWithSources) {
		t.Fatalf("the edit was not saved by hand: %+v", saved.Storyline)
	}
	fresh := generationSetup(t)
	if _, err := fresh.projects.UpdateProject(t.Context(), "alice", fresh.project.ID, clip.ProjectPatch{Storyline: &edit}); !errors.Is(err, clip.ErrStorylineMissing) {
		t.Fatal("an edit of no storyline was saved", err)
	}
}

// CLIP-180: the plan reads as edited by hand once the owner changes it after a writer, and not
// after a generation or a revision.
func TestThePlanReadsAsEditedByHandOnlyAfterTheOwnerChangesIt(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	h.render(t)
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.PlanEditedByHand() {
		t.Fatal("a generated plan reads as edited by hand")
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Cuts[0].Focal = clip.Point{X: .3, Y: .4}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	edited, err := h.store.SaveCorrection(t.Context(), "alice", h.project.ID, p.EditPlanRevision, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !edited.PlanEditedByHand() {
		t.Fatal("the owner's correction does not read as edited by hand")
	}
	startRevision(t, h, "다시", clip.RevisionNarration)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	revised, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if revised.PlanEditedByHand() {
		t.Fatal("a revision's plan reads as edited by hand")
	}
}

// CLIP-178: a source left out of a new batch leaves the storyline — its scenes from every
// paragraph and itself from the sources the storyline was made with.
func TestRemovingASourceTakesItOutOfTheStoryline(t *testing.T) {
	h := storylineWritten(t)
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	made := p.Storyline.MadeWithSources
	if len(made) != 2 {
		t.Fatal("the fixture storyline was not made with both sources", made)
	}
	kept, removed := made[0], made[1]
	edit := []clip.StorylineParagraph{{Text: "두 원본", ObservationIDs: []string{clip.ObservationID(kept, 0), clip.ObservationID(removed, 0)}}}
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Storyline: &edit}); err != nil {
		t.Fatal(err)
	}
	// The first file again, alone: the second is removed.
	if _, err := h.sources.Create(t.Context(), "alice", h.project.ID, manifest(1)); err != nil {
		t.Fatal(err)
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Storyline.MadeWithSources, []string{kept}) {
		t.Fatal("the removed source is still one the storyline was made with", after.Storyline.MadeWithSources)
	}
	if !reflect.DeepEqual(after.Storyline.Paragraphs[0].ObservationIDs, []string{clip.ObservationID(kept, 0)}) || after.Storyline.Paragraphs[0].Text != "두 원본" {
		t.Fatalf("the removed source's scene is still in the storyline: %+v", after.Storyline.Paragraphs[0])
	}
}
