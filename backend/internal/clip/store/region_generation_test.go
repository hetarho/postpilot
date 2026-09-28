package store_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
)

// withIntro turns the harness project's intro (preset A, two slots) on: the
// owner fixed the first slot's words and left the second to be written from an
// instruction.
func withIntro(t *testing.T, h *generationHarness) clip.Project {
	t.Helper()
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	on, owner, instruction := true, "골목 저녁", "동네 이름"
	revision := p.Regions.Revision
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{ExpectedRegionRevision: &revision, IntroRegion: &clip.RegionPatch{Enabled: &on, Slots: []clip.RegionSlotPatch{{ID: "project-intro-1", Text: &owner}, {ID: "project-intro-2", Instruction: &instruction}}}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func project(t *testing.T, h *generationHarness) clip.Project {
	t.Helper()
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func runStoryline(t *testing.T, h *generationHarness) {
	t.Helper()
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
}

// 스토리라인 먼저 drafts the enabled intro's generated slot with the body in its one
// approved writing call (CLIP-187): no call of its own and no other approval, the
// writer is handed the slots as frozen, and the save writes the draft into the
// generated slot while the owner's slot keeps its words whatever came back.
func TestStorylineFirstDraftsTheSlotsInItsOneWritingCall(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if !q.Pricing.Storyline || q.Pricing.PlanCalls() != 1+q.Pricing.Plan.ResponseRetries || q.Pricing.NarrationCalls() != 0 {
		t.Fatalf("the slots changed what the storyline call is priced as: %+v", q.Pricing)
	}
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}, {SlotID: "project-intro-1", Text: "덮어쓰기"}}
	id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if len(h.planner.storylines) != 1 || h.planner.flows != 0 || h.planner.narrations != 0 {
		t.Fatal("drafting the slots made calls of its own", len(h.planner.storylines), h.planner.flows, h.planner.narrations)
	}
	frozen := h.planner.storylines[0].Regions
	if frozen == nil || !frozen.Intro.Enabled || frozen.Intro.Slots[0].Text != "골목 저녁" || frozen.Intro.Slots[1].Instruction != "동네 이름" {
		t.Fatal("the writer was not handed the slots the approval froze", frozen)
	}
	p := project(t, h)
	if s := p.Regions.Intro.Slots; s[0].Text != "골목 저녁" || !s[0].OwnerFixed || s[1].Text != "성수동" || s[1].OwnerFixed {
		t.Fatalf("the saved slots: %+v", p.Regions.Intro)
	}
	if p.Storyline == nil || p.EditPlan != "" {
		t.Fatal("the storyline was not saved alone", p.Storyline, p.EditPlan != "")
	}
}

// 바로 만들기 drafts the slots in its flow call with the storyline and then narrates
// (CLIP-135): two calls as before, and the plan it saves draws the owner's words
// and the draft in slot order.
func TestDirectGenerationDraftsTheSlotsWithTheStorylineThenNarrates(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	h.planner.storyline = writtenStoryline()
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.flows != 1 || h.planner.narrations != 1 || len(h.planner.storylines) != 0 {
		t.Fatal("the direct generation's calls changed", h.planner.flows, h.planner.narrations, len(h.planner.storylines))
	}
	p := project(t, h)
	intro := planRegion(t, p, "hook")
	if len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"골목 저녁", "성수동"}) {
		t.Fatal("the plan does not draw the owner's words and the draft", intro)
	}
	if p.Regions.Intro.Slots[1].Text != "성수동" || p.Storyline == nil {
		t.Fatal("the draft or the storyline was not saved with the plan", p.Regions.Intro, p.Storyline)
	}
}

// Building from the reviewed storyline copies its current slot words unchanged
// through the flow and the narration (CLIP-187): the owner's edit after the
// storyline call stands, and no draft reaches a slot.
func TestBuildingFromTheReviewedStorylineCopiesItsSlotWords(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	runStoryline(t, h)
	reviewed := project(t, h)
	edit := "연남동"
	revision := reviewed.Regions.Revision
	if _, err := h.projects.UpdateProject(t.Context(), "alice", reviewed.ID, clip.ProjectPatch{ExpectedRegionRevision: &revision, IntroRegion: &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: "project-intro-2", Text: &edit}}}}); err != nil {
		t.Fatal(err)
	}
	q, err := h.service.QuoteFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "덮어쓰기"}}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	p := project(t, h)
	intro := planRegion(t, p, "hook")
	if len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"골목 저녁", "연남동"}) || p.Regions.Intro.Slots[1].Text != "연남동" {
		t.Fatal("the build did not carry the reviewed slot words", intro, p.Regions.Intro)
	}
}

// A storyline rewrite may rewrite the generated slots, and a plan that exists
// then draws the changed words without its cuts or captions changing; the
// render goes stale and the body waits for an explicit build (CLIP-188).
func TestAStorylineRewriteSyncsChangedSlotWordsIntoThePlan(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	h.planner.storyline = writtenStoryline()
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	h.render(t)
	before := project(t, h)
	q, err := h.service.QuoteStorylineRevision(t.Context(), "alice", h.project.ID, "동네를 바꿔줘", "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartStorylineRevision(t.Context(), "alice", h.project.ID, "동네를 바꿔줘", "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	h.planner.storylineAnswer = &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "다시 쓴 이야기"}}}
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "연남동"}}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	last := h.planner.storylines[len(h.planner.storylines)-1]
	if last.Regions == nil || last.Regions.Intro.Slots[1].Text != "성수동" {
		t.Fatal("the rewrite was not handed the slots it may keep", last.Regions)
	}
	after := project(t, h)
	if got := regionRows(planRegion(t, after, "hook")[0]); !slices.Equal(got, []string{"골목 저녁", "연남동"}) {
		t.Fatal("the plan does not draw the rewritten slot", got)
	}
	old, _ := clip.DecodeEditPlan(before.EditPlan)
	now, _ := clip.DecodeEditPlan(after.EditPlan)
	if !reflect.DeepEqual(old.Cuts, now.Cuts) || !reflect.DeepEqual(narrationTexts(old), narrationTexts(now)) {
		t.Fatal("the rewrite changed the cuts or the captions")
	}
	if after.EditPlanRevision != before.EditPlanRevision+1 || after.RenderedPlanRevision != before.RenderedPlanRevision || after.Result == nil {
		t.Fatal("the changed words did not stale the render in the same save", after.EditPlanRevision, after.RenderedPlanRevision)
	}
	if after.Storyline.Paragraphs[0].Text != "다시 쓴 이야기" {
		t.Fatal("the rewritten body was not saved")
	}
}

func narrationTexts(plan clip.EditPlan) []string {
	var out []string
	for _, text := range plan.Portable.Elements {
		if text.Scope == clip.NarrationScope {
			out = append(out, text.Resolved.Text)
		}
	}
	return out
}

// A failed storyline call leaves the body, the slots and the plan exactly as
// they were (CLIP-26).
func TestAFailedStorylineCallKeepsTheSlots(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	runStoryline(t, h)
	before := project(t, h)
	q, err := h.service.QuoteStorylineRevision(t.Context(), "alice", h.project.ID, "다시", "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartStorylineRevision(t.Context(), "alice", h.project.ID, "다시", "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "연남동"}}
	h.planner.storylineErr = llm.ErrBadOutput
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected storyline failure")
	}
	after := project(t, h)
	if !reflect.DeepEqual(after.Regions, before.Regions) || !reflect.DeepEqual(after.Storyline, before.Storyline) || after.EditPlan != before.EditPlan {
		t.Fatal("a failed storyline call left part of itself behind")
	}
}

// The slots are writing input: an instruction or a preset edited after the quote
// invalidates the approval, as an edited storyline does (QUOTA-45, CLIP-69).
func TestASlotEditAfterTheQuoteInvalidatesIt(t *testing.T) {
	h := generationSetup(t)
	p := withIntro(t, h)
	for _, edit := range []func(clip.Project) clip.ProjectPatch{
		func(p clip.Project) clip.ProjectPatch {
			instruction := "다른 지시"
			revision := p.Regions.Revision
			return clip.ProjectPatch{ExpectedRegionRevision: &revision, IntroRegion: &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: "project-intro-2", Instruction: &instruction}}}}
		},
		func(clip.Project) clip.ProjectPatch {
			cover := "cover"
			return clip.ProjectPatch{IntroPreset: &cover}
		},
	} {
		q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
		if err != nil {
			t.Fatal(err)
		}
		if p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, edit(p)); err != nil {
			t.Fatal(err)
		}
		if _, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q)); !errors.Is(err, clip.ErrQuoteChanged) {
			t.Fatal("a slot edited after its quote was written", err)
		}
	}
	if h.planner.observe != 0 || len(h.planner.storylines) != 0 {
		t.Fatal("a refused start made a provider call")
	}
}

// Words an answer binds into a slot are drawn as written, so words the slot
// cannot draw are refused by that slot before any paid work (CDS-77, CLIP-102).
func TestBoundSlotWordsTooWideAreRefusedBeforeAnyPaidWork(t *testing.T) {
	h := generationSetup(t)
	body := `<clip version="1"><field id="place" label="장소" required="true">어디였나요?</field><text id="intro" kind="fixed" role="hook"><row><value field="place"/></row></text></clip>`
	template, err := h.projects.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "지역", CompositionBody: body})
	if err != nil {
		t.Fatal(err)
	}
	wide := strings.Repeat("하나둘셋넷", 12)
	inputs := clip.CompositionInputs{Values: map[string]string{"place": wide}, Items: map[string][]composition.Item{}}
	p, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{VideoTemplateID: &template.ID, CompositionInputs: &inputs})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Regions.Intro.Enabled || p.Regions.Intro.Slots[0].Text != wide {
		t.Fatal("the template did not seed its bound slot", p.Regions.Intro)
	}
	var problem *composition.Problem
	if _, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w"); !errors.As(err, &problem) || problem.ElementID != "project-intro-1" || problem.Reason != "copy_limit" {
		t.Fatal("bound words too wide for their slot were quoted", err)
	}
	if h.planner.observe != 0 || h.planner.plans != 0 {
		t.Fatal("the refusal came after a provider call")
	}
}

// A continuation that resumes on the kept flow keeps the words that flow drafted;
// once the owner has written a slot since, the kept flow no longer answers the
// inputs, it is written again, and the owner's words win the save (CLIP-93,
// CLIP-187).
func TestAContinuationKeepsTheDraftsButNeverTheOwnersOldWords(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	h.planner.storyline = writtenStoryline()
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	h.planner.narrateErr = llm.ErrBadOutput
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected narration failure")
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil || !state.FlowReady || !reflect.DeepEqual(state.RegionDrafts, h.planner.regionDrafts) {
		t.Fatal("the recovery did not carry the drafts", state, err)
	}
	h.planner.narrateErr, h.planner.regionDrafts = nil, nil
	flows := h.planner.flows
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.flows != flows || project(t, h).Regions.Intro.Slots[1].Text != "성수동" {
		t.Fatal("the continuation rewrote the flow or lost its drafts", h.planner.flows, flows, project(t, h).Regions.Intro)
	}

	// The same failure, then the owner writes the slot the kept flow drafted.
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "옛 초안"}}
	h.planner.narrateErr = llm.ErrBadOutput
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected narration failure")
	}
	p := project(t, h)
	owner := "주인이 쓴 말"
	revision := p.Regions.Revision
	if _, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{ExpectedRegionRevision: &revision, IntroRegion: &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: "project-intro-2", Text: &owner}}}}); err != nil {
		t.Fatal(err)
	}
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || q.Pricing.SkipFlow || q.Pricing.ObservationCalls != 0 {
		t.Fatal("a kept flow drafted against other slot words was reused, or the observations were not", q.Pricing.SkipFlow, q.Pricing.ObservationCalls, err)
	}
	h.planner.narrateErr = nil
	flows = h.planner.flows
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	p = project(t, h)
	if h.planner.flows != flows+1 || p.Regions.Intro.Slots[1].Text != owner || !slices.Equal(regionRows(planRegion(t, p, "hook")[0]), []string{"골목 저녁", owner}) {
		t.Fatal("a retry restored stale slot words", p.Regions.Intro)
	}
}

// A storyline job cancelled while its call runs saves nothing: the body, the slots
// and the plan stay as they were (CLIP-79, CLIP-26).
func TestACancelledStorylineJobKeepsTheSlots(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	before := project(t, h)
	q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	j, err := h.jobs.PickNextQueued(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	err = h.service.RunStoryline(ctx, "alice", id, h.project.ID, j.Payload, func(stage string, _, _ int) {
		if stage == "storyline" {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("the cancelled storyline job did not stop", err)
	}
	after := project(t, h)
	if !reflect.DeepEqual(after.Regions, before.Regions) || after.Storyline != nil || after.EditPlan != before.EditPlan {
		t.Fatal("a cancelled storyline job left part of itself behind", after.Regions)
	}
}

// A revision request rewrites the flow or the narration and never a region word:
// its writer is told what the intro shows, and the plan it saves draws the
// slots as they stand (CLIP-131, CLIP-188).
func TestARevisionKeepsTheRegionWords(t *testing.T) {
	h := generationSetup(t)
	withIntro(t, h)
	h.planner.storyline = writtenStoryline()
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	before := project(t, h)
	q, err := h.service.QuoteRevision(t.Context(), "alice", h.project.ID, "더 짧게", clip.RevisionNarration, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartRevision(t.Context(), "alice", h.project.ID, "더 짧게", clip.RevisionNarration, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "덮어쓰기"}}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	last := h.planner.revisions[len(h.planner.revisions)-1]
	if last.Regions == nil || last.Regions.Intro.Slots[1].Text != "성수동" {
		t.Fatal("the revision's writer was not told what the intro shows", last.Regions)
	}
	after := project(t, h)
	if !reflect.DeepEqual(after.Regions, before.Regions) || !slices.Equal(regionRows(planRegion(t, after, "hook")[0]), []string{"골목 저녁", "성수동"}) {
		t.Fatal("a revision changed the region words", after.Regions.Intro)
	}
	if after.EditPlanRevision == before.EditPlanRevision {
		t.Fatal("the revision did not save its plan")
	}
}

// Choosing a preset, saving slot words and rendering never ask a writer to fill
// a slot (CLIP-20, CLIP-187): an enabled region with nothing to draw stays
// empty and is rendered as it is.
func TestNoRegionActionCallsTheWriter(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	calls := func() int {
		return h.planner.plans + h.planner.flows + h.planner.narrations + len(h.planner.storylines) + len(h.planner.revisions)
	}
	before := calls()
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-outro-1", Text: "누가 채움"}}
	outro := "credits"
	p, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{OutroPreset: &outro})
	if err != nil || !p.Regions.Outro.Enabled {
		t.Fatal("choosing a preset did not switch the outro on", err)
	}
	words := "다시 올게요"
	revision := p.Regions.Revision
	if _, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{ExpectedRegionRevision: &revision, OutroRegion: &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: "project-outro-2", Text: &words}}}}); err != nil {
		t.Fatal(err)
	}
	h.render(t)
	if calls() != before {
		t.Fatal("a region action called the writer", calls(), before)
	}
	p = project(t, h)
	if p.Regions.Outro.Slots[0].Text != "" || !slices.Equal(regionRows(planRegion(t, p, "ending")[0]), []string{"", "다시 올게요", "", ""}) || p.Result == nil || p.RenderedPlanRevision != p.EditPlanRevision {
		t.Fatal("the region was filled or not rendered as it stands", p.Regions.Outro)
	}
}
