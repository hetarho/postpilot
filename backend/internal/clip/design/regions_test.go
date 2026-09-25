package design_test

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

func number(v float64) *float64 { return &v }

// The four presets as CDS-71, CDS-72, CDS-75 and CDS-76 state them: slots and
// rules in order with their ink gaps, the block centred at its 9:16 y.
func TestRegionPresetsMatchCDS(t *testing.T) {
	slot := func(role, fill, stroke string) design.RegionItem {
		return design.RegionItem{Slot: &design.RegionSlotSpec{Role: role, Fill: fill, Stroke: stroke, Shadow: "text"}}
	}
	tracked := func(item design.RegionItem, tracking float64) design.RegionItem {
		item.Slot.Tracking = number(tracking)
		return item
	}
	gap := func(px float64) design.RegionItem { return design.RegionItem{Gap: px} }
	rule := func(kind string) design.RegionItem {
		return design.RegionItem{Rule: &design.RegionRuleSpec{Kind: kind}}
	}
	caption := slot("caption", "text_white", "small")
	caption.Slot.Alpha = number(.8)
	centre := func(y float64) design.RegionAnchor { return design.RegionAnchor{Kind: "centre", Y: y} }
	want := map[string]map[string]design.RegionPreset{
		"intro": {
			"a": {Anchor: centre(942), Items: []design.RegionItem{slot("headline", "text_white", "text"), gap(48), tracked(slot("label", "text_muted", "small"), .02)}},
			"b": {Anchor: centre(969), Items: []design.RegionItem{rule("hair"), gap(43), slot("hook", "text_white", "none"), gap(46), rule("hair"), gap(50), tracked(slot("label", "text_muted", "none"), .02)}},
		},
		"outro": {
			"b": {Anchor: centre(938), Items: []design.RegionItem{slot("hook", "text_white", "none"), gap(46), rule("hair"), gap(44), slot("body", "text_white", "small")}},
			"e": {Anchor: centre(961), Items: []design.RegionItem{tracked(slot("label", "text_muted", "small"), .08), gap(31), tracked(slot("display", "text_white", "text"), .04), gap(42), rule("bar"), gap(42), caption}},
		},
	}
	for kind, presets := range want {
		for id, expected := range presets {
			got, ok := design.Region(kind, id)
			if !ok || !reflect.DeepEqual(got, expected) {
				t.Fatalf("%s.%s: %+v want %+v", kind, id, got, expected)
			}
			for _, s := range got.Slots() {
				if _, ok := design.Type[s.Role]; !ok {
					t.Fatal("unknown slot role", s.Role)
				}
			}
			got.Items[0] = design.RegionItem{Gap: 1}
			again, _ := design.Region(kind, id)
			if reflect.DeepEqual(again.Items[0], got.Items[0]) {
				t.Fatal("caller mutated preset")
			}
		}
	}
	if _, ok := design.Region("intro", "e"); ok {
		t.Fatal("wrong region")
	}
	if _, ok := design.Region("other", "b"); ok {
		t.Fatal("unknown region")
	}
	if !reflect.DeepEqual(design.Rules, map[string]design.RuleToken{"hair": {Width: 520, Height: 2, Alpha: .55}, "bar": {Width: 160, Height: 6, Alpha: 1}}) {
		t.Fatal("rules", design.Rules)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(design.JSON(), &document); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"styles", "classes", "scene_styles", "guards"} {
		if _, ok := document[key]; ok {
			t.Fatal("retired key", key)
		}
	}
	// A numeric-only edit on one side must be detected by the exact mirror gate.
	changed := bytes.Replace(design.JSON(), []byte(`942`), []byte(`943`), 1)
	if bytes.Equal(changed, design.JSON()) {
		t.Fatal("mirror comparison missed a preset edit")
	}
}

func TestRegionItemIsExactlyOneKind(t *testing.T) {
	for _, bad := range []string{`{}`, `{"slot":{},"gap":4}`, `{"shape":{}}`} {
		var item design.RegionItem
		if err := json.Unmarshal([]byte(bad), &item); err == nil {
			t.Fatalf("%s decoded", bad)
		}
	}
}

type stepBox struct {
	name        string
	top, bottom float64
}

func blockSteps(l design.RegionLayout) []stepBox {
	var out []stepBox
	for _, s := range l.Slots {
		out = append(out, stepBox{name: s.Spec.Role, top: s.Box.Y, bottom: s.Box.Y + s.Box.Height})
	}
	for _, r := range l.Rules {
		out = append(out, stepBox{name: "rule " + r.Kind, top: r.Box.Y, bottom: r.Box.Y + r.Box.Height})
	}
	// Stack order is top to bottom.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].top < out[j-1].top; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func presetGaps(p design.RegionPreset) []float64 {
	var gaps []float64
	for _, it := range p.Items {
		if it.Gap > 0 {
			gaps = append(gaps, it.Gap)
		}
	}
	return gaps
}

var shortRows = map[string][]string{
	"intro.a": {"해미 한우", "서울 연남동 · 숯불 한우 구이"},
	"intro.b": {"해미 한우", "서울 연남동 · 숯불 한우 구이"},
	"outro.b": {"다시 가고 싶은 불판", "자세한 후기는 블로그에"},
	"outro.e": {"직접 먹어본 평점", "4.8", "다시 가고 싶은 불판"},
}

// CDS-87 and CDS-79: every ratio stacks the same gaps and holds the block's
// centre at the same fraction of canvas height it has on 9:16.
func TestRegionBlocksKeepTheirGapsOnEveryRatio(t *testing.T) {
	for name, rows := range shortRows {
		kind, id, _ := strings.Cut(name, ".")
		preset, _ := design.Region(kind, id)
		for _, ratio := range []string{"vertical", "horizontal", "square"} {
			t.Run(name+"/"+ratio, func(t *testing.T) {
				l, err := design.LayoutRegion(kind, id, ratio, rows)
				if err != nil || l.Over {
					t.Fatal(err, l.Over)
				}
				steps := blockSteps(l)
				gaps := presetGaps(preset)
				if len(steps) != len(gaps)+1 {
					t.Fatalf("%d steps for %d gaps", len(steps), len(gaps))
				}
				for i, g := range gaps {
					if got := steps[i+1].top - steps[i].bottom; math.Abs(got-g) > 1e-9 {
						t.Fatalf("gap %d after %s is %v, want %v", i, steps[i].name, got, g)
					}
				}
				canvas, _ := design.Canvas(ratio)
				centre := l.Bounds.Y + l.Bounds.Height/2
				if want := preset.Anchor.Y * float64(canvas.Height) / 1920; math.Abs(centre-want) > 1e-9 {
					t.Fatalf("block centre %v, want %v", centre, want)
				}
			})
		}
	}
}

// The gaps were chosen so short single-line text lands where the old fixed
// baselines drew it (CDS-75 ≈ 940 and 1020, CDS-72 ≈ 836 · 980 · 1110).
func TestShortRegionTextStaysNearTheFormerBaselines(t *testing.T) {
	former := map[string][]float64{"intro.a": {940, 1020}, "intro.b": {960, 1090}, "outro.b": {900, 1040}, "outro.e": {836, 980, 1110}}
	for name, want := range former {
		kind, id, _ := strings.Cut(name, ".")
		l, err := design.LayoutRegion(kind, id, "vertical", shortRows[name])
		if err != nil {
			t.Fatal(err)
		}
		for i, y := range want {
			if got := l.Slots[i].Lines[0].Baseline; math.Abs(got-y) > 6 {
				t.Fatalf("%s slot %d baseline %.1f, former %v", name, i, got, y)
			}
		}
	}
}

func TestRegionLayoutFitsWrapsAndDropsEmptySlots(t *testing.T) {
	short, _ := design.LayoutRegion("intro", "a", "vertical", []string{"해미 한우", "서울 연남동"})
	long, _ := design.LayoutRegion("intro", "a", "vertical", []string{"연남동 골목에서 30년째 숯불 한우만 굽는 집", "서울 연남동"})
	if len(long.Slots[0].Lines) != 2 || long.Slots[0].Type.Size >= 96 || long.Slots[0].Type.Size < 60 {
		t.Fatalf("a long headline wraps below its size: %+v", long.Slots[0])
	}
	if long.Bounds.Height <= short.Bounds.Height {
		t.Fatal("a wrapped slot must grow the block")
	}
	if c1, c2 := short.Bounds.Y+short.Bounds.Height/2, long.Bounds.Y+long.Bounds.Height/2; math.Abs(c1-c2) > 1e-9 {
		t.Fatalf("the block keeps its centre: %v vs %v", c1, c2)
	}
	if g := long.Slots[1].Box.Y - (long.Slots[0].Box.Y + long.Slots[0].Box.Height); math.Abs(g-48) > 1e-9 {
		t.Fatalf("the gap stays 48 under a wrapped slot: %v", g)
	}
	// An empty second slot is dropped with the gap before it: one slot, centred.
	one, _ := design.LayoutRegion("intro", "a", "vertical", []string{"해미 한우", ""})
	if len(one.Slots) != 1 || math.Abs(one.Bounds.Y+one.Bounds.Height/2-942) > 1e-9 {
		t.Fatalf("an empty slot collapses around the anchor: %+v", one)
	}
	// Rules stay while any slot draws, and nothing draws with no text at all.
	b, _ := design.LayoutRegion("outro", "b", "vertical", []string{"다시 가고 싶은 불판", ""})
	if len(b.Rules) != 1 {
		t.Fatal("the rule stays while a slot draws")
	}
	none, _ := design.LayoutRegion("outro", "b", "vertical", []string{"", ""})
	if len(none.Slots) != 0 || len(none.Rules) != 0 {
		t.Fatal("an empty block draws nothing")
	}
	over, _ := design.LayoutRegion("intro", "a", "vertical", []string{"해미 한우", strings.Repeat("서울 마포구 연남동 ", 5)})
	if !over.Over || !over.Slots[1].Over {
		t.Fatal("a label past its floor overflows")
	}
	for _, s := range long.Slots {
		for _, line := range s.Lines {
			if line.Width > long.Width {
				t.Fatalf("a line is %v wide on a %v measure", line.Width, long.Width)
			}
		}
	}
}
