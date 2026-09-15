package design_test

import (
	"bytes"
	"encoding/json"
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
