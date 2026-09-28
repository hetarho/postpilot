package clip

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// A slot's fit is the renderer's own: for every preset, ratio and slot, the
// verdict admission and drafting read is the one the block layout reaches
// (CDS-77, CDS-86), so nothing admitted is refused at render and nothing
// refused would have drawn.
func TestRegionSlotFitIsTheLayoutsVerdict(t *testing.T) {
	texts := []string{"오늘의 장면", "연남동 골목에서 30년째 숯불 한우만 굽는 집", strings.Repeat("하나둘셋넷", 5)}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, kind := range []string{"intro", "outro"} {
			for _, id := range design.RegionIDs(kind) {
				preset, _ := design.Region(kind, id)
				for i := range preset.Slots() {
					for _, text := range texts {
						rows := make([]string, len(preset.Slots()))
						rows[i] = text
						layout, err := design.LayoutRegion(kind, id, ratio, rows)
						if err != nil {
							t.Fatal(err)
						}
						slot, drawn := layout.Slot(i)
						if !drawn {
							t.Fatal("a filled slot was not laid out", kind, id, i)
						}
						if refused := regionSlotFit(kind, id, ratio, i, text) != ""; refused != slot.Over {
							t.Fatalf("%s %s/%s slot %d %q: admission %v, layout %v", ratio, kind, id, i, text, refused, slot.Over)
						}
					}
				}
			}
		}
	}
}

// Before generation the words the owner fixed or an answer supplies are held to
// their slot by width, not by a count (CDS-86): nine syllables shrink into intro
// A's headline, an unbreakable line too wide at the floor is refused by its
// slot, and so is a character the preset's face lacks (CDS-84). A generated slot
// and a region that is off are nobody's words yet and refuse nothing (CDS-77).
func TestAuthoredSlotsAreAdmittedByTheirOwnFit(t *testing.T) {
	regions := func(text string, owner bool) ProjectRegions {
		slot := RegionSlot{ID: "project-intro-1", Text: text, OwnerFixed: owner}
		if !owner {
			slot.Binding = []composition.Part{{Field: "place"}}
		}
		return ProjectRegions{Intro: ProjectRegion{Enabled: true, Slots: []RegionSlot{slot, {ID: "project-intro-2"}}}}
	}
	presets := composition.DesignSelection{Intro: "a", Outro: "e"}
	for _, owner := range []bool{true, false} {
		if err := ValidateAuthoredRegions(regions("하나둘셋넷다섯여섯", owner), presets, "vertical"); err != nil {
			t.Fatal("nine syllables did not shrink into the headline", err)
		}
		var problem *composition.Problem
		if err := ValidateAuthoredRegions(regions(strings.Repeat("하나둘셋넷", 5), owner), presets, "vertical"); !errors.As(err, &problem) || problem.ElementID != "project-intro-1" || problem.Reason != "copy_limit" {
			t.Fatal("an unbreakable line too wide at the floor was admitted", owner, err)
		}
	}
	generated := regions(strings.Repeat("하나둘셋넷", 5), false)
	generated.Intro.Slots[0].Binding = nil
	if err := ValidateAuthoredRegions(generated, presets, "vertical"); err != nil {
		t.Fatal("a generated slot's words were refused before anything wrote them", err)
	}
	off := regions(strings.Repeat("하나둘셋넷", 5), true)
	off.Intro.Enabled = false
	if err := ValidateAuthoredRegions(off, presets, "vertical"); err != nil {
		t.Fatal("a region that is off was checked", err)
	}
	missing := ""
	for c := rune(0xAC00); c <= 0xD7A3; c++ {
		if !design.Covers("jua", 400, string(c)) && design.Covers("paperlogy", 800, string(c)) {
			missing = string(c)
			break
		}
	}
	var problem *composition.Problem
	if err := ValidateAuthoredRegions(regions("맛집 "+missing, true), composition.DesignSelection{Intro: "sticker", Outro: "b"}, "vertical"); !errors.As(err, &problem) || problem.ElementID != "project-intro-1" || problem.Reason != "unsupported_glyph" {
		t.Fatal("a character the sticker face lacks was admitted", err)
	}
	if err := ValidateAuthoredRegions(regions("맛집 "+missing, true), presets, "vertical"); err != nil {
		t.Fatal("Paperlogy draws the syllable, so intro A admits it", err)
	}
	// A region slot takes no substitute face (CDS-84): Paperlogy maps 갂 to a glyph
	// with no outline, so intro A's headline refuses it by its slot.
	if err := ValidateAuthoredRegions(regions("맛집 갂", true), presets, "vertical"); !errors.As(err, &problem) || problem.ElementID != "project-intro-1" || problem.Reason != "unsupported_glyph" {
		t.Fatal("a character the headline's face does not draw was admitted", err)
	}
}

// Drafts fill the generated slots of the enabled regions and nothing else: a
// slot the owner fixed, one an answer binds, a region that is off, a slot past
// the preset and an unknown id keep what they hold (CLIP-187). The digest the
// quote binds moves with every input a call reads and is empty with both off.
func TestDraftsFillOnlyGeneratedSlotsAndTheDigestFollowsTheInputs(t *testing.T) {
	presets := composition.DesignSelection{Intro: "a", Outro: "e"}
	r := ProjectRegions{
		Intro: ProjectRegion{Enabled: true, Slots: []RegionSlot{{ID: "project-intro-1"}, {ID: "project-intro-2", Text: "주인", OwnerFixed: true}, {ID: "project-intro-3", Text: "남은 말"}}},
		Outro: ProjectRegion{Slots: []RegionSlot{{ID: "project-outro-1"}}},
	}
	before := r.WritingDigest(presets)
	ApplyRegionDrafts(&r, []RegionDraft{{SlotID: "project-intro-1", Text: "성수동", Notice: NoticeShortened("intro")}, {SlotID: "project-intro-2", Text: "덮어쓰기"}, {SlotID: "project-intro-3", Text: "넘친 말"}, {SlotID: "project-outro-1", Text: "꺼짐"}, {SlotID: "nobody", Text: "누구"}}, presets)
	if s := r.Intro.Slots; s[0].Text != "성수동" || s[0].Notice != "intro_slot_shortened" || s[1].Text != "주인" || s[2].Text != "남은 말" || r.Outro.Slots[0].Text != "" {
		t.Fatalf("a draft reached a slot it may not write: %+v", r)
	}
	if notices := RegionSlotNotices(r, presets, "vertical"); len(notices) != 2 || notices[0].ElementID != "project-intro-1" || notices[0].Reason != "intro_slot_shortened" || notices[0].Action != "repair" {
		t.Fatal("the shortened draft was not noticed on its slot", notices)
	}
	edit := "주인이 고침"
	if err := ApplyRegionPatch(&r.Intro, &RegionPatch{Slots: []RegionSlotPatch{{ID: "project-intro-1", Text: &edit}}}, DefaultLimits()); err != nil || r.Intro.Slots[0].Notice != "" {
		t.Fatal("the owner's edit left the writer's notice behind", err)
	}
	// A slot the owner deliberately left empty stays empty (CLIP-186).
	blank := ""
	if err := ApplyRegionPatch(&r.Intro, &RegionPatch{Slots: []RegionSlotPatch{{ID: "project-intro-2", Text: &blank}}}, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	ApplyRegionDrafts(&r, []RegionDraft{{SlotID: "project-intro-2", Text: "채우기"}}, presets)
	if r.Intro.Slots[1].Text != "" || !r.Intro.Slots[1].OwnerFixed {
		t.Fatal("a draft filled the slot the owner left empty", r.Intro.Slots[1])
	}
	if after := r.WritingDigest(presets); after == before || after == "" {
		t.Fatal("the digest did not follow the slot words")
	}
	instructed := r.Clone()
	instructed.Intro.Slots[0].Instruction = "동네"
	if instructed.WritingDigest(presets) == r.WritingDigest(presets) || r.WritingDigest(composition.DesignSelection{Intro: "cover", Outro: "e"}) == r.WritingDigest(presets) {
		t.Fatal("the digest ignores an instruction or a preset")
	}
	r.Intro.Enabled = false
	if r.WritingDigest(presets) != "" {
		t.Fatal("a project with both regions off changed its digest")
	}
}
