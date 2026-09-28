package clip_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// regionPlanFixture is a saved template-free plan and a project whose intro
// (cover, three slots) and outro (B, two slots) are drafted but not yet drawn.
func regionPlanFixture(t *testing.T) (clip.Project, clip.EditPlan) {
	t.Helper()
	p, plan := creationFixture(t)
	p.IntroPreset, p.OutroPreset = "cover", "b"
	p.Regions = &clip.ProjectRegions{Revision: 4,
		Intro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{
			{ID: "project-intro-1", Text: "성수 골목", OwnerFixed: true},
			{ID: "project-intro-2", Instruction: "동네 이름"},
			{ID: "project-intro-3", Text: "저녁 영업", OwnerFixed: true},
		}},
		Outro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{
			{ID: "project-outro-1", Text: "다시 만나요"},
			{ID: "project-outro-2"},
		}},
	}
	return p, plan
}

func regionEntries(plan clip.EditPlan, role string) []clip.PortableText {
	var out []clip.PortableText
	for _, t := range plan.Portable.Elements {
		if t.Resolved.Element.Role == role {
			out = append(out, t)
		}
	}
	return out
}

func rowTexts(t clip.PortableText) []string {
	var out []string
	for _, row := range t.Resolved.Rows {
		out = append(out, row.Text)
	}
	return out
}

func projected(t *testing.T, p clip.Project, plan clip.EditPlan) clip.EditPlan {
	t.Helper()
	next, _, err := clip.ProjectPlanRegions(plan, *p.Regions, p.DesignSelection().RegionPresets())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clip.EncodeEditPlan(next); err != nil {
		t.Fatal("the projected plan is not a valid plan", err)
	}
	return next
}

// Enabling a region on a plan whose composition declares none gives the plan
// that region, over the output that exists and without adding footage
// (CLIP-66, CLIP-68).
func TestEnabledRegionsEnterATemplateFreePlan(t *testing.T) {
	p, plan := regionPlanFixture(t)
	next, changed, err := clip.ProjectPlanRegions(plan, *p.Regions, p.DesignSelection().RegionPresets())
	if err != nil || !changed {
		t.Fatal("the regions were not projected", changed, err)
	}
	intro, outro := regionEntries(next, "hook"), regionEntries(next, "ending")
	if len(intro) != 1 || len(outro) != 1 {
		t.Fatal("each region is one element", intro, outro)
	}
	if got := rowTexts(intro[0]); !slices.Equal(got, []string{"성수 골목", "", "저녁 영업"}) {
		t.Fatal("the intro rows are not its slots in order", got)
	}
	if got := rowTexts(outro[0]); !slices.Equal(got, []string{"다시 만나요", ""}) {
		t.Fatal("the outro rows are not its slots in order", got)
	}
	if r := intro[0].Resolved; r.InstanceID != "project-intro" || r.StartMS != 0 || r.EndMS != 2500 || r.Element.Kind != "fixed" || !intro[0].OwnerEdited {
		t.Fatalf("the intro does not open the output with the owner's words: %+v", intro[0])
	}
	if r := outro[0].Resolved; r.InstanceID != "project-outro" || r.StartMS != 17000 || r.EndMS != 20000 || r.Element.Kind != "ai" || outro[0].OwnerEdited {
		t.Fatalf("the outro does not close the output with generated words: %+v", outro[0])
	}
	if next.DurationMS != plan.DurationMS || !reflect.DeepEqual(next.Cuts, plan.Cuts) || !reflect.DeepEqual(next.Portable.Cuts, plan.Portable.Cuts) {
		t.Fatal("a region changed the footage")
	}
	if captions := regionEntries(next, "caption"); !reflect.DeepEqual(captions, regionEntries(plan, "caption")) {
		t.Fatal("a region changed the captions")
	}
	// The placement every renderer reads keeps each slot on its own slot,
	// the blank middle one included (CDS-73).
	if rows := clip.RegionRows(clip.ResolvedElements(next.Portable.Elements), p.DesignSelection().RegionPresets(), "intro"); !slices.Equal(rows, []string{"성수 골목", "", "저녁 영업"}) {
		t.Fatal("a blank slot moved the ones after it", rows)
	}
	again, changed, err := clip.ProjectPlanRegions(next, *p.Regions, p.DesignSelection().RegionPresets())
	if err != nil || changed || !reflect.DeepEqual(again, next) {
		t.Fatal("projecting the same regions again changed the plan", changed, err)
	}
}

// A plan a writer drew through template entries keeps its words once and only
// in the project's element: the entries are gone rather than drawn beside it,
// the owner's slot wins over whatever an entry held, and the first entry's
// interval is kept (CLIP-147, CLIP-190).
func TestTemplateEntriesAreReplacedByTheProjectedRegion(t *testing.T) {
	p, plan := regionPlanFixture(t)
	start, end := 0, 4000
	legacy := func(id string, rows ...string) clip.PortableText {
		e := composition.Element{ID: id, Kind: "ai", Role: "hook", Style: "auto", Position: "auto", Align: "center", Basis: "output-start", StartMS: &start, EndMS: &end}
		text := clip.PortableText{Resolved: composition.ResolvedElement{InstanceID: id, Element: e, StartMS: 0, EndMS: 4000}}
		for _, row := range rows {
			text.Resolved.Rows = append(text.Resolved.Rows, composition.ResolvedRow{Text: row})
		}
		return text
	}
	plan.Portable.Elements = append([]clip.PortableText{legacy("opening", "writer first", "writer second"), legacy("more", "writer third")}, plan.Portable.Elements...)
	plan.Portable.RetiredElements = []clip.PortableText{legacy("deleted-entry", "old")}
	regions := p.Regions.Clone()
	clip.AbsorbWrittenRegions(&regions, plan)
	if got := []string{regions.Intro.Slots[0].Text, regions.Intro.Slots[1].Text, regions.Intro.Slots[2].Text}; !slices.Equal(got, []string{"성수 골목", "writer second", "저녁 영업"}) {
		t.Fatal("the writer's words did not become the generated slot's alone", got)
	}
	p.Regions = &regions
	next := projected(t, p, plan)
	intro := regionEntries(next, "hook")
	if len(intro) != 1 || intro[0].Resolved.InstanceID != "project-intro" || !slices.Equal(rowTexts(intro[0]), []string{"성수 골목", "writer second", "저녁 영업"}) {
		t.Fatal("a template entry is still drawn beside the project's region", intro)
	}
	if intro[0].Resolved.StartMS != 0 || intro[0].Resolved.EndMS != 4000 {
		t.Fatal("the region lost the interval the plan had given it", intro[0].Resolved)
	}
	if next.Portable.Elements[0].Resolved.InstanceID != "project-intro" {
		t.Fatal("the region moved from where its entries stood")
	}
	for _, t2 := range next.Portable.RetiredElements {
		if t2.Resolved.Element.Role == "hook" {
			t.Fatal("a template entry can still be restored beside the region", t2.Resolved.InstanceID)
		}
	}
}

// Switching a region off draws nothing and keeps every draft; switching it back
// on restores the interval the owner gave it, and an enabled region whose slots
// are all empty draws nothing either (CDS-73, CLIP-189).
func TestSwitchingARegionOffAndOnKeepsItsDraftAndInterval(t *testing.T) {
	p, plan := regionPlanFixture(t)
	next := projected(t, p, plan)
	start, end := 500, 6000
	for i, text := range next.Portable.Elements {
		if text.Resolved.InstanceID == "project-intro" {
			next.Portable.Elements[i].Resolved.Element.StartMS, next.Portable.Elements[i].Resolved.Element.EndMS = &start, &end
		}
	}
	p.Regions.Intro.Enabled = false
	off := projected(t, p, next)
	if len(regionEntries(off, "hook")) != 0 {
		t.Fatal("a region that is off is still drawn")
	}
	if p.Regions.Intro.Slots[0].Text != "성수 골목" || p.Regions.Intro.Slots[2].Text != "저녁 영업" {
		t.Fatal("switching off touched the draft")
	}
	p.Regions.Intro.Enabled = true
	on := projected(t, p, off)
	intro := regionEntries(on, "hook")
	if len(intro) != 1 || intro[0].Resolved.StartMS != 500 || intro[0].Resolved.EndMS != 6000 || !slices.Equal(rowTexts(intro[0]), []string{"성수 골목", "", "저녁 영업"}) {
		t.Fatal("switching back on lost the region's words or interval", intro)
	}
	blank := p.Regions.Clone()
	for i := range blank.Intro.Slots {
		blank.Intro.Slots[i].Text = ""
	}
	p.Regions = &blank
	if empty := projected(t, p, on); len(regionEntries(empty, "hook")) != 0 {
		t.Fatal("an enabled region with nothing to draw drew an element")
	}
}

// A smaller preset draws its own slots and notices the words it cannot hold;
// the larger one draws them again (CLIP-147, CLIP-189). A generated text too
// wide for its slot is left out and noticed, while the owner's own is refused
// by the slot it stands in (CDS-64, CDS-77).
func TestPresetCapacityAndSlotBoundsFollowTheirOwnRules(t *testing.T) {
	p, plan := regionPlanFixture(t)
	p.Regions.Intro.Slots[1].Text, p.Regions.Intro.Slots[1].OwnerFixed = "연남동", true
	p.IntroPreset = "a"
	small := projected(t, p, plan)
	if got := rowTexts(regionEntries(small, "hook")[0]); !slices.Equal(got, []string{"성수 골목", "연남동"}) {
		t.Fatal("the smaller preset did not draw its own two slots", got)
	}
	presets := p.DesignSelection().RegionPresets()
	notices := clip.RegionSlotNotices(*p.Regions, presets, p.Ratio)
	if len(notices) != 1 || notices[0].ElementID != "project-intro-3" || notices[0].Reason != clip.NoticeRegionLineSurplus || notices[0].Action != "removal" {
		t.Fatal("the unused slot was not noticed by its own id", notices)
	}
	p.IntroPreset = "cover"
	large := projected(t, p, small)
	if got := rowTexts(regionEntries(large, "hook")[0]); !slices.Equal(got, []string{"성수 골목", "연남동", "저녁 영업"}) {
		t.Fatal("the retained slot did not return", got)
	}
	if notices := clip.RegionSlotNotices(*p.Regions, p.DesignSelection().RegionPresets(), p.Ratio); len(notices) != 0 {
		t.Fatal("a notice outlived its cause", notices)
	}

	wide := strings.Repeat("하나둘셋넷", 5)
	p.Regions.Outro.Slots[0].Text = wide
	generated := projected(t, p, large)
	if got := regionEntries(generated, "ending"); len(got) != 0 {
		t.Fatal("a generated text too wide for its slot was drawn", got)
	}
	notices = clip.RegionSlotNotices(*p.Regions, p.DesignSelection().RegionPresets(), p.Ratio)
	if len(notices) != 1 || notices[0].ElementID != "project-outro-1" || notices[0].Reason != "outro_slot_omitted" {
		t.Fatal("the left-out generated text was not noticed on its slot", notices)
	}
	if p.Regions.Outro.Slots[0].Text != wide {
		t.Fatal("leaving the text out rewrote the draft")
	}
	if err := clip.ValidateOwnerRegions(*p.Regions, p.DesignSelection().RegionPresets(), p.Ratio); err != nil {
		t.Fatal("a generated overflow refused the owner", err)
	}
	p.Regions.Outro.Slots[0].OwnerFixed = true
	var problem *composition.Problem
	if err := clip.ValidateOwnerRegions(*p.Regions, p.DesignSelection().RegionPresets(), p.Ratio); !errors.As(err, &problem) || problem.ElementID != "project-outro-1" || problem.Reason != "copy_limit" {
		t.Fatal("the owner's overflow was not refused by its slot", err)
	}
	p.Regions.Outro.Enabled = false
	if err := clip.ValidateOwnerRegions(*p.Regions, p.DesignSelection().RegionPresets(), p.Ratio); err != nil {
		t.Fatal("a region that is off was checked", err)
	}
}

// correctRegions applies a correction draft the way the save does and returns
// the plan it saves with the regions it changed.
func correctRegions(t *testing.T, p clip.Project, draft clip.CorrectionPlan) (clip.EditPlan, *clip.ProjectRegions, error) {
	t.Helper()
	old, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil {
		return clip.EditPlan{}, nil, err
	}
	return clip.CorrectRegions(p, old, next)
}

func savedRegions(t *testing.T, p clip.Project, plan clip.EditPlan) clip.Project {
	t.Helper()
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	p.EditPlan = raw
	return p
}

// A correction of a region's line is that slot's owner-fixed text in the same
// save; removing the region's element switches the region off and restoring it
// switches it back on, neither rewriting a draft (CLIP-188, CLIP-189).
func TestCorrectedRegionLinesBecomeTheProjectSlots(t *testing.T) {
	p, plan := regionPlanFixture(t)
	p = savedRegions(t, p, projected(t, p, plan))
	saved, _ := clip.DecodeEditPlan(p.EditPlan)
	draft := creationDraft(saved)
	for i := range draft.Elements {
		if draft.Elements[i].InstanceID == "project-intro" {
			draft.Elements[i].Rows[1].Text = "연남동 골목"
		}
	}
	next, regions, err := correctRegions(t, p, draft)
	if err != nil || regions == nil {
		t.Fatal("the region line was not carried into the slots", err)
	}
	if s := regions.Intro.Slots[1]; s.Text != "연남동 골목" || !s.OwnerFixed || s.Instruction != "동네 이름" || regions.Revision != 4 {
		t.Fatalf("the corrected slot: %+v", regions)
	}
	if regions.Outro.Slots[0].OwnerFixed || regions.Intro.Slots[0].Text != "성수 골목" {
		t.Fatal("an uncorrected slot changed", regions)
	}
	if got := rowTexts(regionEntries(next, "hook")[0]); !slices.Equal(got, []string{"성수 골목", "연남동 골목", "저녁 영업"}) {
		t.Fatal("the saved plan does not draw the corrected slot", got)
	}
	// A correction touching no region line changes no slot.
	untouched := creationDraft(saved)
	if _, regions, err := correctRegions(t, p, untouched); err != nil || regions != nil {
		t.Fatal("a correction without region edits changed the regions", regions, err)
	}

	removed := creationDraft(saved)
	removed.Elements = slices.DeleteFunc(removed.Elements, func(e clip.CorrectionText) bool { return e.InstanceID == "project-intro" })
	next, regions, err = correctRegions(t, p, removed)
	if err != nil || regions == nil || regions.Intro.Enabled || regions.Intro.Slots[0].Text != "성수 골목" || regions.Intro.Slots[1].OwnerFixed {
		t.Fatal("removing the intro did not switch it off with its draft kept", regions, err)
	}
	if len(regionEntries(next, "hook")) != 0 {
		t.Fatal("the removed region is still drawn")
	}
	// Undo brings the element back as it was archived.
	p.Regions = regions
	p = savedRegions(t, p, next)
	off, _ := clip.DecodeEditPlan(p.EditPlan)
	restored := creationDraft(off)
	before := creationDraft(saved)
	restored.Elements = append(restored.Elements, before.Elements[slices.IndexFunc(before.Elements, func(e clip.CorrectionText) bool { return e.InstanceID == "project-intro" })])
	next, regions, err = correctRegions(t, p, restored)
	if err != nil || regions == nil || !regions.Intro.Enabled || regions.Intro.Slots[1].OwnerFixed {
		t.Fatal("restoring the intro did not switch it back on as it was", regions, err)
	}
	if got := rowTexts(regionEntries(next, "hook")[0]); !slices.Equal(got, []string{"성수 골목", "", "저녁 영업"}) {
		t.Fatal("the restored region draws other words", got)
	}
}

// A correction cannot add or drop a region's lines, give lined region text a
// second copy, or leave the owner's words too wide for their slot (CDS-64).
func TestRegionCorrectionsRefuseWhatTheSlotsCannotHold(t *testing.T) {
	p, plan := regionPlanFixture(t)
	p = savedRegions(t, p, projected(t, p, plan))
	saved, _ := clip.DecodeEditPlan(p.EditPlan)
	edit := func(change func(*clip.CorrectionText)) error {
		draft := creationDraft(saved)
		for i := range draft.Elements {
			if draft.Elements[i].InstanceID == "project-intro" {
				change(&draft.Elements[i])
			}
		}
		_, _, err := correctRegions(t, p, draft)
		return err
	}
	if err := edit(func(e *clip.CorrectionText) { e.Rows = append(e.Rows, composition.ResolvedRow{Text: "넷째"}) }); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("a correction added a slot", err)
	}
	if err := edit(func(e *clip.CorrectionText) { e.Text = "두 번째 사본" }); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("a correction gave lined region text a second copy", err)
	}
	var problem *composition.Problem
	if err := edit(func(e *clip.CorrectionText) { e.Rows[0].Text = strings.Repeat("하나둘셋넷", 12) }); !errors.As(err, &problem) || problem.ElementID != "project-intro-1" || problem.Reason != "copy_limit" {
		t.Fatal("the owner's overflow was not refused by its slot", err)
	}
}

// A correction touching no region line leaves the plan's region entries as the
// owner left them, the interval given to one included; an interval the output
// no longer holds is the owner's to correct and refuses the projection, named
// by the element it stands on (CLIP-67).
func TestCorrectionsLeaveUntouchedRegionsAndRefuseAnIntervalOutsideTheOutput(t *testing.T) {
	p, plan := regionPlanFixture(t)
	p.Regions.Outro.Enabled = false
	start, end := 0, 2500
	e := composition.Element{ID: "opening", Kind: "fixed", Role: "hook", Style: "auto", Position: "auto", Align: "center", Basis: "output-start", StartMS: &start, EndMS: &end}
	entry := clip.PortableText{Resolved: composition.ResolvedElement{InstanceID: "opening", Element: e, StartMS: 0, EndMS: 2500, Rows: []composition.ResolvedRow{{Text: "성수 골목"}, {}, {Text: "저녁 영업"}}}}
	plan.Portable.Elements = append(plan.Portable.Elements, entry)
	p = savedRegions(t, p, plan)
	saved, _ := clip.DecodeEditPlan(p.EditPlan)
	draft := creationDraft(saved)
	for i := range draft.Elements {
		if draft.Elements[i].InstanceID == "opening" {
			longer := 4000
			draft.Elements[i].EndMS = &longer
		}
	}
	next, regions, err := correctRegions(t, p, draft)
	if err != nil || regions != nil {
		t.Fatal("an interval edit changed the regions", regions, err)
	}
	intro := regionEntries(next, "hook")
	if len(intro) != 1 || intro[0].Resolved.InstanceID != "opening" || intro[0].Resolved.EndMS != 4000 {
		t.Fatal("the owner's interval did not stay on the element it was given", intro)
	}
	late := 25000
	for i, text := range saved.Portable.Elements {
		if text.Resolved.InstanceID == "opening" {
			saved.Portable.Elements[i].Resolved.Element.EndMS = &late
		}
	}
	p.Regions.Intro.Slots[1].Text, p.Regions.Intro.Slots[1].OwnerFixed = "연남동", true
	var problem *composition.Problem
	if _, _, err := clip.ProjectPlanRegions(saved, *p.Regions, p.DesignSelection().RegionPresets()); !errors.As(err, &problem) || problem.ElementID != "opening" || problem.Reason != "interval_outside" {
		t.Fatal("an interval outside the output was moved to fit", err)
	}
}
