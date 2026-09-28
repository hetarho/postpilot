package ai

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// draftingInput is a project whose intro (preset A) has one generated slot
// seeded from a template row declaring at most five syllables, one generated
// slot of its own, and whose outro is off.
func draftingInput() clip.PlanningInput {
	body := `<clip version="1"><text id="opening" kind="ai" role="hook"><row chars="5">가게 이름</row></text></clip>`
	return clip.PlanningInput{Ratio: "vertical", Design: clip.ProjectDesign{IntroPreset: "a", OutroPreset: "b"},
		Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: 1, Body: body}},
		Regions: &clip.ProjectRegions{
			Intro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{{ID: "project-intro-1"}, {ID: "project-intro-2", ElementID: "opening", Row: 0}}},
			Outro: clip.ProjectRegion{Slots: []clip.RegionSlot{{ID: "project-outro-1"}, {ID: "project-outro-2"}}},
		}}
}

// CDS-86 and CDS-77: a written slot that shrinks or wraps into its slot is kept
// as the writer wrote it; one still too wide at the floor takes the shorter text
// and says so, and with none fitting it draws nothing and says that. A row's own
// declared maximum stays the stricter bound (CLIP-116).
func TestADraftIsJudgedByItsSlotFit(t *testing.T) {
	in := draftingInput()
	limits := clip.DefaultCompositionLimits()
	long := "연남동 골목에서 30년째 숯불 한우만 굽는 집"
	unbreakable := strings.Repeat("하나둘셋넷", 5)
	for _, c := range []struct {
		answer regionSlotJSON
		want   clip.RegionDraft
	}{
		{regionSlotJSON{SlotID: "project-intro-1", Text: long}, clip.RegionDraft{SlotID: "project-intro-1", Text: long}},
		{regionSlotJSON{SlotID: "project-intro-1", Text: unbreakable, ShortText: "해미 한우"}, clip.RegionDraft{SlotID: "project-intro-1", Text: "해미 한우", Notice: "intro_slot_shortened"}},
		{regionSlotJSON{SlotID: "project-intro-1", Text: unbreakable}, clip.RegionDraft{SlotID: "project-intro-1", Notice: "intro_slot_omitted"}},
		{regionSlotJSON{SlotID: "project-intro-1", Text: "한 줄\n두 줄"}, clip.RegionDraft{SlotID: "project-intro-1", Notice: "intro_slot_omitted"}},
		{regionSlotJSON{SlotID: "project-intro-1"}, clip.RegionDraft{SlotID: "project-intro-1"}},
	} {
		drafts := regionDrafts(in, []regionSlotJSON{c.answer}, limits)
		if len(drafts) != 2 || drafts[0] != c.want {
			t.Fatalf("%q: %+v", c.answer.Text, drafts)
		}
	}
	drafts := regionDrafts(in, []regionSlotJSON{{SlotID: "project-intro-2", Text: "연남동 숯불 한우 오마카세", ShortText: "해미 한우"}}, limits)
	if drafts[1] != (clip.RegionDraft{SlotID: "project-intro-2", Text: "해미 한우", Notice: "intro_slot_shortened"}) {
		t.Fatal("the row's declared maximum was ignored", drafts)
	}
}

// Only the slots asked for are taken, each once: an id nobody asked for, a second
// answer, an answer to the owner's own slot and one to a region that is off are
// never taken (CLIP-65, CLIP-187).
func TestDraftsTakeOnlyTheSlotsTheWriterWasAsked(t *testing.T) {
	in := draftingInput()
	in.Regions.Intro.Slots[1].Text, in.Regions.Intro.Slots[1].OwnerFixed = "주인의 말", true
	answers := []regionSlotJSON{
		{SlotID: "project-intro-1", Text: "첫째"},
		{SlotID: "project-intro-1", Text: "둘째"},
		{SlotID: "project-intro-2", Text: "덮어쓰기"},
		{SlotID: "project-outro-1", Text: "꺼진 아웃트로"},
		{SlotID: "nobody", Text: "누구"},
	}
	drafts := regionDrafts(in, answers, clip.DefaultCompositionLimits())
	if !reflect.DeepEqual(drafts, []clip.RegionDraft{{SlotID: "project-intro-1", Text: "첫째"}}) {
		t.Fatal(drafts)
	}
	if regionDrafts(clip.PlanningInput{Composition: in.Composition}, answers, clip.DefaultCompositionLimits()) != nil {
		t.Fatal("a job frozen without slots drafted some")
	}
	// Without a template the slots are written just the same (CLIP-5).
	in.Composition.Snapshot.Body = clip.EmptyCompositionBody()
	if got := regionSlotsPayload(in, clip.DefaultCompositionLimits(), false); len(got) != 2 || got[0]["write"] != true || got[1]["write"] != false || got[1]["text"] != "주인의 말" {
		t.Fatal(got)
	}
}
