package ai_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestOptionalCaptionSafeRegionsValidateAndRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		boxes   []map[string]float64
		invalid bool
	}{
		{name: "legacy absent"},
		{name: "empty", boxes: []map[string]float64{}},
		{name: "safe", boxes: []map[string]float64{{"x": 0, "y": .1, "width": .8, "height": .2}}},
		{name: "outside", boxes: []map[string]float64{{"x": .5, "y": .1, "width": .8, "height": .2}}, invalid: true},
		{name: "inverted", boxes: []map[string]float64{{"x": 0, "y": .1, "width": -.1, "height": .2}}, invalid: true},
		{name: "zero", boxes: []map[string]float64{{"x": 0, "y": .1, "width": .8, "height": 0}}, invalid: true},
		{name: "too many", boxes: make([]map[string]float64, 5), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := observation()
			if tc.boxes != nil {
				for i, box := range tc.boxes {
					if box == nil {
						tc.boxes[i] = map[string]float64{"x": 0, "y": 0, "width": .1, "height": .1}
					}
				}
				firstSegment(v)["caption_safe"] = tc.boxes
			}
			service, _, _ := newService(t, raw(v), true)
			got, _, err := service.ObserveChunk(t.Context(), testRef(), chunk())
			if tc.invalid {
				d, ok := clip.DiagnosticFromError(err)
				if err == nil || !ok || d.Check != "observe_subject_bounds" {
					t.Fatalf("wrong validation: %v %+v", err, d)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Segments[0].CaptionSafe) != len(tc.boxes) {
				t.Fatal("lost regions")
			}
			data, err := json.Marshal(got)
			var decoded clip.ChunkAnalysis
			if err != nil || json.Unmarshal(data, &decoded) != nil || !reflect.DeepEqual(decoded, got) {
				t.Fatal("analysis round trip lost regions", err)
			}
		})
	}
}
