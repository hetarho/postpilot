package design_test

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

func TestRegionPresetsMatchCDS(t *testing.T) {
	number := func(v float64) *float64 { return &v }
	slot := func(kind string, y float64, fill, stroke string) design.RegionSlot {
		return design.RegionSlot{Type: kind, Y: y, Fill: fill, Stroke: stroke, Shadow: "text"}
	}
	label := func(y, tracking float64, stroke string) design.RegionSlot {
		s := slot("label", y, "text_muted", stroke)
		s.Tracking = number(tracking)
		return s
	}
	display := slot("display", 980, "text_white", "text")
	display.Tracking = number(.04)
	caption := slot("caption", 1110, "text_white", "small")
	caption.Alpha = number(.8)
	want := map[string]map[string]design.RegionPreset{
		"intro": {
			"a": {Slots: []design.RegionSlot{slot("headline", 940, "text_white", "text"), label(1020, .02, "small")}},
			"b": {Slots: []design.RegionSlot{slot("hook", 960, "text_white", "none"), label(1090, .02, "none")}, Rules: []design.RegionRule{{Kind: "hair", Y: 846}, {Kind: "hair", Y: 1010}}},
		},
		"outro": {
			"b": {Slots: []design.RegionSlot{slot("hook", 900, "text_white", "none"), slot("body", 1040, "text_white", "small")}, Rules: []design.RegionRule{{Kind: "hair", Y: 950}}},
			"e": {Slots: []design.RegionSlot{label(836, .08, "small"), display, caption}, Rules: []design.RegionRule{{Kind: "bar", Y: 1030}}},
		},
	}
	for kind, presets := range want {
		for id, expected := range presets {
			got, ok := design.Region(kind, id)
			if !ok || !reflect.DeepEqual(got, expected) {
				t.Fatalf("%s.%s: %+v want %+v", kind, id, got, expected)
			}
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var back design.RegionPreset
			if err := json.Unmarshal(data, &back); err != nil || !reflect.DeepEqual(back, got) {
				t.Fatal("round trip", err)
			}
			for _, s := range got.Slots {
				if _, ok := design.Type[s.Type]; !ok {
					t.Fatal("unknown slot type", s.Type)
				}
			}
			got.Slots[0].Y = 0
			again, _ := design.Region(kind, id)
			if again.Slots[0].Y == 0 {
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
	changed := bytes.Replace(design.JSON(), []byte(`940`), []byte(`941`), 1)
	if bytes.Equal(changed, design.JSON()) {
		t.Fatal("mirror comparison missed a preset edit")
	}
}

// CDS-79: the block moves as one piece. Every slot baseline and every rule
// offset differs from its 9:16 value by one constant per preset and ratio, so
// consecutive spacing is identical on all three canvases.
func TestRegionBlocksKeepTheirSpacingOnEveryRatio(t *testing.T) {
	// The offsets both 1080-high canvases must produce, from the preset's own
	// midpoint: a preset that drifts from these has changed its geometry.
	offsets := map[string]float64{"intro.a": -428.75, "intro.b": -423.5, "outro.b": -424.375, "outro.e": -425.6875}
	for _, choice := range []struct{ kind, id string }{{"intro", "a"}, {"intro", "b"}, {"outro", "b"}, {"outro", "e"}} {
		preset, ok := design.Region(choice.kind, choice.id)
		if !ok {
			t.Fatal("missing preset", choice)
		}
		base := make([]float64, 0, len(preset.Slots)+len(preset.Rules))
		for _, slot := range preset.Slots {
			base = append(base, slot.Y)
		}
		for _, rule := range preset.Rules {
			base = append(base, rule.Y)
		}
		for _, ratio := range []string{"vertical", "horizontal", "square"} {
			name := choice.kind + "." + choice.id
			t.Run(name+"/"+ratio, func(t *testing.T) {
				want := offsets[name]
				if ratio == "vertical" {
					// 9:16 is the authored canvas: nothing may move there.
					want = 0
				}
				if got := design.RegionOffset(preset, ratio); math.Abs(got-want) > 1e-9 {
					t.Fatalf("offset = %v, want %v", got, want)
				}
				for i, y := range base {
					moved := design.RegionBaseline(preset, ratio, y)
					if math.Abs(moved-(y+want)) > 1e-9 {
						t.Fatalf("y %v moved to %v, want one constant %v", y, moved, want)
					}
					if i > 0 && math.Abs((moved-design.RegionBaseline(preset, ratio, base[i-1]))-(y-base[i-1])) > 1e-9 {
						t.Fatalf("spacing after y %v changed on %s", base[i-1], ratio)
					}
				}
			})
		}
	}
}
