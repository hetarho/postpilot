package design_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

func candidate(anchor, align string, x, y, w, h float64) design.Candidate {
	return design.Candidate{Anchor: anchor, Align: align, Plate: design.Bounds{X: x, Y: y, Width: w, Height: h}, Fits: true}
}

// CDS-38, rule by rule.
func TestSelectAnchorFollowsCDS38(t *testing.T) {
	bottom := candidate("bottom", "center", 300, 1270, 400, 110)
	upper := candidate("upper_mid", "center", 300, 645, 400, 110)
	lower := candidate("lower_mid", "center", 300, 1045, 400, 110)
	top := candidate("top", "left", 96, 80, 400, 110)
	none := design.Bounds{}

	// Without a subject box the first (default) candidate wins.
	if got := design.SelectAnchor([]design.Candidate{bottom, lower}, none, nil, false, "", nil); got != 0 {
		t.Fatalf("default anchor = %d", got)
	}
	// A candidate that does not fit its safe area is dropped.
	unfit := bottom
	unfit.Fits = false
	if got := design.SelectAnchor([]design.Candidate{unfit, lower}, none, nil, false, "", nil); got != 1 {
		t.Fatalf("an unfit candidate was chosen: %d", got)
	}
	if got := design.SelectAnchor([]design.Candidate{unfit}, none, nil, false, "", nil); got != -1 {
		t.Fatalf("a copy with nowhere to go = %d", got)
	}
	// Copy yields to the badge and the chips, never the other way round.
	badge := []design.Element{{Kind: "badge", Region: design.Bounds{X: 280, Y: 1250, Width: 200, Height: 80}}}
	if got := design.SelectAnchor([]design.Candidate{bottom, lower}, none, badge, false, "", nil); got != 1 {
		t.Fatalf("copy sat on the badge: %d", got)
	}
	// The highest-ranked candidate covering ≤ 15 % of the subject wins.
	subject := design.Bounds{X: 280, Y: 1200, Width: 500, Height: 400}
	if got := design.SelectAnchor([]design.Candidate{bottom, lower}, subject, nil, false, "", nil); got != 1 {
		t.Fatalf("the covering candidate was kept: %d", got)
	}
	// When EVERY candidate covers more than 15 %, the least covering wins.
	tall := design.Bounds{X: 300, Y: 1000, Width: 200, Height: 400}
	narrower := candidate("lower_mid", "center", 350, 1045, 300, 110)
	if got := design.SelectAnchor([]design.Candidate{bottom, narrower}, tall, nil, false, "", nil); got != 1 {
		t.Fatalf("the least covering candidate was not chosen: %d", got)
	}
	// And the first candidate still wins when it is the one that covers least.
	if got := design.SelectAnchor([]design.Candidate{narrower, bottom}, tall, nil, false, "", nil); got != 0 {
		t.Fatalf("ranking lost: %d", got)
	}
	// Consecutive cuts move at most one anchor step: BOTTOM → TOP is three.
	if got := design.SelectAnchor([]design.Candidate{top, lower}, none, nil, false, "bottom", nil); got != 1 {
		t.Fatalf("a three-step jump was allowed: %d", got)
	}
	// The step is a preference, not a veto: with nowhere within one step the
	// copy is still placed rather than lost (see SelectAnchor's note).
	if got := design.SelectAnchor([]design.Candidate{top}, none, nil, false, "bottom", nil); got != 0 {
		t.Fatalf("a copy was dropped to obey the step rule: %d", got)
	}
	if got := design.SelectAnchor([]design.Candidate{upper}, none, nil, false, "lower_mid", nil); got != 0 {
		t.Fatal("one step was refused")
	}
	// Readable footage text allows only TOP or BOTTOM — and the step rule does
	// not apply there, because obeying both is impossible.
	if got := design.SelectAnchor([]design.Candidate{lower, top}, none, nil, true, "bottom", nil); got != 1 {
		t.Fatalf("readable text allowed a middle anchor: %d", got)
	}
	if got := design.SelectAnchor([]design.Candidate{lower, upper}, none, nil, true, "", nil); got != -1 {
		t.Fatal("readable text with no TOP or BOTTOM candidate")
	}
}

// CDS-42's voice: no emoji, no ㅋㅋ or ㄹㅇ, no superlatives.
func TestBannedVoice(t *testing.T) {
	for _, text := range []string{"진짜 맛있어요 🙂", "ㅋㅋ 웃겼어요", "ㄹㅇ 맛집", "역대급 맛", "최고의 하루"} {
		if !design.Banned(text) {
			t.Fatalf("%q was accepted", text)
		}
	}
	for _, text := range []string{"조용하고 아늑했어요", "9,900원", "서울 연남동 — 김밥", "O'Brien's 10:30"} {
		if design.Banned(text) {
			t.Fatalf("%q was refused", text)
		}
	}
}

// CDS-36: a hard cut is the default, a scene change earns a 200 ms fade and no
// more than 40 % of the boundaries may take one — the earliest ones.
func TestTransitionsFadeOnlyOnSceneChangeAndWithinTheRatio(t *testing.T) {
	for name, c := range map[string]struct {
		scenes []string
		want   []int
	}{
		"one cut":            {[]string{"food"}, []int{0}},
		"two of one scene":   {[]string{"food", "food"}, []int{0, 0}},
		"two that differ":    {[]string{"food", "menu"}, []int{0, 0}},
		"one change of four": {[]string{"food", "food", "menu", "menu"}, []int{0, 0, 200, 0}},
		"every boundary":     {[]string{"food", "menu", "food", "menu", "food", "menu"}, []int{0, 200, 200, 0, 0, 0}},
		"unknown scene":      {[]string{"", "scenery", "menu", "menu"}, []int{0, 0, 200, 0}},
		"none":               {nil, []int{}},
	} {
		t.Run(name, func(t *testing.T) {
			got := design.Transitions(c.scenes)
			if len(got) != len(c.want) {
				t.Fatalf("%v", got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("%v, want %v", got, c.want)
				}
			}
			// Whatever the scenes, the ratio holds and the clip never fades in.
			fades := 0
			for _, ms := range got {
				if ms != 0 {
					fades++
				}
			}
			if len(got) > 0 && got[0] != 0 {
				t.Fatal("the clip faded in from nothing")
			}
			if len(got) > 1 && float64(fades) > design.Transition.FadeRatioMax*float64(len(got)-1) {
				t.Fatalf("%d of %d boundaries faded", fades, len(got)-1)
			}
		})
	}
}

// CDS-37 r3: the target range is the preset's own where the template names one,
// the shared 1.2–6.0 s where it does not, and a food close-up is capped at
// 4.0 s under every preset.
func TestCutBoundsReadThePresetAndCapFood(t *testing.T) {
	for _, scene := range []string{"scenery", "interior", "menu", "person", "unknown"} {
		if minimum, maximum := design.CutBounds(scene, ""); minimum != 1200 || maximum != 6000 {
			t.Fatalf("%s %d..%d", scene, minimum, maximum)
		}
	}
	if minimum, maximum := design.CutBounds("food", ""); minimum != 1200 || maximum != 4000 {
		t.Fatalf("food %d..%d", minimum, maximum)
	}
	for preset, want := range map[string][2]int{"restaurant": {2500, 4000}, "cafe": {4000, 6000}, "stay": {4000, 6000}, "beauty": {2000, 3000}, "home": {3000, 4000}} {
		if minimum, maximum := design.CutBounds("interior", preset); minimum != want[0] || maximum != want[1] {
			t.Fatalf("%s interior %d..%d, want %v", preset, minimum, maximum, want)
		}
		_, maximum := design.CutBounds("food", preset)
		if maximum > 4000 {
			t.Fatalf("%s food ceiling %d", preset, maximum)
		}
	}
	// A preset whose floor sits above the food ceiling gives food a flat 4.0 s.
	if minimum, maximum := design.CutBounds("food", "cafe"); minimum != 4000 || maximum != 4000 {
		t.Fatalf("cafe food %d..%d", minimum, maximum)
	}
	if minimum, maximum := design.CutBounds("interior", "unknown-preset"); minimum != 1200 || maximum != 6000 {
		t.Fatalf("unknown preset %d..%d", minimum, maximum)
	}
}

func TestCaptionOffersAlternativeWhenDefaultCoversSubject(t *testing.T) {
	rule := design.Caption()
	if rule.Anchor != "upper_mid" || rule.AnchorAlt != "lower_mid" {
		t.Fatalf("simple anchors: %+v", rule)
	}
	bottom := candidate(rule.Anchor, rule.Align, 300, 1270, 400, 110)
	top := candidate(rule.AnchorAlt, rule.Align, 300, 80, 400, 110)
	subject := design.Bounds{X: 280, Y: 1200, Width: 500, Height: 400}
	if got := design.SelectAnchor([]design.Candidate{bottom, top}, subject, nil, false, "", nil); got != 1 {
		t.Fatalf("simple covers subject: %d", got)
	}
}

func TestObservedSpaceRanksOnlyAfterPlacementGuards(t *testing.T) {
	bottom := candidate("bottom", "center", 300, 1270, 400, 110)
	top := candidate("top", "center", 300, 80, 400, 110)
	lower := candidate("lower_mid", "center", 300, 1045, 400, 110)
	upper := candidate("upper_mid", "center", 300, 645, 400, 110)
	safeTop := []design.Bounds{{X: 0, Y: 40, Width: 1080, Height: 200}}
	none := design.Bounds{}
	if got := design.SelectAnchor([]design.Candidate{bottom, top}, none, nil, false, "", safeTop); got != 1 {
		t.Fatal("empty-space evidence ignored", got)
	}
	// The plate must fit WHOLLY, not just have its center in empty space.
	if got := design.SelectAnchor([]design.Candidate{bottom, top}, none, nil, false, "", []design.Bounds{{X: 300, Y: 80, Width: 399, Height: 110}}); got != 0 {
		t.Fatal("partial containment treated as safe")
	}
	unfit := top
	unfit.Fits = false
	if got := design.SelectAnchor([]design.Candidate{bottom, unfit}, none, nil, false, "", safeTop); got != 0 {
		t.Fatal("evidence overrode safe-area bounds")
	}
	badge := []design.Element{{Kind: "badge", Region: top.Plate}}
	if got := design.SelectAnchor([]design.Candidate{bottom, top}, none, badge, false, "", safeTop); got != 0 {
		t.Fatal("evidence displaced badge")
	}
	if got := design.SelectAnchor([]design.Candidate{bottom, top}, top.Plate, nil, false, "", safeTop); got != 0 {
		t.Fatal("evidence overrode subject coverage")
	}
	if got := design.SelectAnchor([]design.Candidate{bottom, top}, none, nil, false, "bottom", safeTop); got != 0 {
		t.Fatal("evidence overrode one-step restriction")
	}
	if got := design.SelectAnchor([]design.Candidate{bottom, top}, none, nil, true, "bottom", safeTop); got != 1 {
		t.Fatal("readable-text exception lost")
	}
	if got := design.SelectAnchor([]design.Candidate{lower, top}, none, nil, true, "", []design.Bounds{lower.Plate}); got != 1 {
		t.Fatal("evidence allowed middle anchor over readable text")
	}
	// Within the subject ceiling, safe evidence changes ranking without moving
	// the subject preference below it, and ties preserve original candidate order.
	safeMid := []design.Bounds{upper.Plate, lower.Plate}
	if got := design.SelectAnchor([]design.Candidate{upper, lower}, none, nil, false, "", safeMid); got != 0 {
		t.Fatal("unstable equal evidence")
	}
}
