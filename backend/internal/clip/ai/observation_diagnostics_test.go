package ai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func TestObservationFailuresHavePrivateBoundedDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, check string
		change      func(map[string]any, map[string]any)
	}{
		{"source", "observe_source_identity", func(v, s map[string]any) { v["source_id"] = "private-canary" }},
		{"index", "observe_chunk_identity", func(v, s map[string]any) { v["chunk_index"] = -1 }},
		{"unknown field", "output_shape", func(v, s map[string]any) { s["private-canary"] = "private-canary" }},
		{"inverted", "observe_segment_time", func(v, s map[string]any) { s["start_ms"], s["end_ms"] = 4000, 3000 }},
		{"past end", "observe_segment_time", func(v, s map[string]any) { s["start_ms"], s["end_ms"] = 6000, 7000 }},
		{"overlap", "observe_segment_overlap", func(v, s map[string]any) { v["segments"] = []any{s, s} }},
		{"focal", "observe_focal", func(v, s map[string]any) { s["focal"].(map[string]any)["x"] = -.1 }},
		{"box", "observe_subject_bounds", func(v, s map[string]any) { s["subject"].(map[string]any)["width"] = 1 }},
		{"text", "observe_text_length", func(v, s map[string]any) { s["event"] = strings.Repeat("한", 2001) }},
		{"quality", "observe_quality", func(v, s map[string]any) { s["quality"] = " " }},
		{"subjects", "observe_subject_count", func(v, s map[string]any) {
			subjects := make([]string, 21)
			for i := range subjects {
				subjects[i] = "private-canary"
			}
			s["subjects"] = subjects
		}},
		{"subject text", "observe_subject_text", func(v, s map[string]any) { s["subjects"] = []string{" "} }},
		{"description", "observe_description", func(v, s map[string]any) { s["event"], s["subjects"] = " ", []string{} }},
		{"scene", "observe_scene", func(v, s map[string]any) { s["scene"] = "private-canary" }},
		{"silent speech", "observe_silent_speech", func(v, s map[string]any) { s["speech"] = "private-canary" }},
		{"empty", "observe_segment_count", func(v, s map[string]any) { v["segments"] = []any{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := observation()
			tc.change(v, firstSegment(v))
			s, models, _ := newService(t, raw(v), true)
			in := chunk()
			if tc.name == "silent speech" {
				in.Source.Info.HasAudio = false
			}
			_, usage, err := s.ObserveChunk(t.Context(), testRef(), in)
			d, ok := clip.DiagnosticFromError(err)
			var code interface{ OutputValidationCode() string }
			if !errors.Is(err, llm.ErrBadOutput) || !ok || d.Check != tc.check || !errors.As(err, &code) || code.OutputValidationCode() != tc.check || usage != models.response.Usage || len(models.calls) != 1 {
				t.Fatalf("lost failure or made paid retry: %v %+v", err, d)
			}
			encoded, _ := json.Marshal(d)
			if strings.Contains(string(encoded), "private-canary") || strings.Contains(string(encoded), "한") || d.Values["duration_ms"] != 5000 {
				t.Fatal("unsafe diagnostic", string(encoded))
			}
			if tc.name == "overlap" && (d.Values["segment"] != 2 || d.Values["previous_end_ms"] != 65000 || d.Values["raw_start_ms"] != 0) {
				t.Fatal(d)
			}
			if tc.name == "focal" && d.Values["focal_x_ppm"] != -100000 {
				t.Fatal(d)
			}
			if tc.name == "index" && (d.Values["expected_index"] != 1 || d.Values["actual_index"] != -1) {
				t.Fatal(d)
			}
		})
	}
}

func TestObservationTruncationKeepsDiagnosticAndUsage(t *testing.T) {
	v := observation()
	firstSegment(v)["subject"].(map[string]any)["width"] = 1
	s, models, _ := newService(t, raw(v), true)
	models.response.FinishReason = "length"
	_, usage, err := s.ObserveChunk(t.Context(), testRef(), chunk())
	d, ok := clip.DiagnosticFromError(err)
	if !errors.Is(err, llm.ErrOutputTruncated) || !ok || d.Check != "observe_subject_bounds" || d.Values["segment"] != 1 || usage != models.response.Usage || len(models.calls) != 1 {
		t.Fatal("truncation lost evidence", err, d)
	}
}
