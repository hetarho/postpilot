package clip_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func regionEntry(id, role string, rows ...string) clip.PortableText {
	e := composition.ResolvedElement{InstanceID: id, Element: composition.Element{ID: id, Role: role, Kind: "fixed"}}
	for _, row := range rows {
		e.Rows = append(e.Rows, composition.ResolvedRow{Text: row})
	}
	return clip.PortableText{Resolved: e}
}

// CLIP-147: the preset the PROJECT chose decides how many of a region's lines
// are drawn, and entries of one region fill its slots in the order they stand in.
func TestRegionLinesFillTheProjectPresetInOutlineOrder(t *testing.T) {
	// Two intro entries of one line each fill exactly what one entry of two
	// lines fills, and only the first paints the preset's rules.
	split := []clip.PortableText{regionEntry("a", "hook", "성수 곱창"), regionEntry("b", "hook", "성수동")}
	placements := clip.RegionPlacements(clip.ResolvedElements(split), composition.DesignSelection{Intro: "b", Outro: "e"})
	if got := placements["a"]; got != (clip.RegionPlacement{Offset: 0, Drawn: 1, Rules: true}) {
		t.Fatal("the first intro entry did not take the first slot", got)
	}
	if got := placements["b"]; got != (clip.RegionPlacement{Offset: 1, Drawn: 1}) {
		t.Fatal("the second intro entry did not take the second slot", got)
	}
	// A third line has no slot in intro B, and nothing is refused for it.
	over := []clip.PortableText{regionEntry("intro", "hook", "하나", "둘", "셋")}
	if got := clip.RegionPlacements(clip.ResolvedElements(over), composition.DesignSelection{Intro: "b", Outro: "e"})["intro"]; got.Drawn != 2 {
		t.Fatal("the surplus line was counted as drawn", got)
	}
}

// The same body against two presets: the selection is what decides, and the
// notice for a line nobody draws is derived rather than stored (CLIP-139).
func TestRegionSurplusIsNoticedPerProjectPreset(t *testing.T) {
	plan := clip.EditPlan{Portable: &clip.PortablePlan{Elements: []clip.PortableText{regionEntry("outro", "ending", "오늘의 점수", "9.5", "또 갈래요")}}}
	if notices := clip.RegionSurplusNotices(plan, composition.DesignSelection{Intro: "b", Outro: "e"}); len(notices) != 0 {
		t.Fatal("outro E holds three lines and still noticed one", notices)
	}
	notices := clip.RegionSurplusNotices(plan, composition.DesignSelection{Intro: "b", Outro: "b"})
	if len(notices) != 1 || notices[0].Reason != clip.NoticeRegionLineSurplus || notices[0].ElementID != "outro" || notices[0].Action != "removal" {
		t.Fatal("outro B drew two of three lines without saying so", notices)
	}
	// The plan itself records nothing, so choosing E again clears it.
	if len(plan.Notices) != 0 {
		t.Fatal("a derived notice was stored", plan.Notices)
	}
	if active := clip.ActivePlanNotices(plan, composition.DesignSelection{Intro: "b", Outro: "b"}); len(active) != 1 {
		t.Fatal("the surplus notice did not reach the owner", active)
	}
	if active := clip.ActivePlanNotices(plan, composition.DesignSelection{Intro: "b", Outro: "e"}); len(active) != 0 {
		t.Fatal("changing the preset in ① left the notice behind", active)
	}
}
