package ai_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// Recorded synthetic model responses are replayed without network or payment.
// This is not a reproduction of the owner's unavailable failed response.
func TestRecordedMultiSourcePlans(t *testing.T) {
	data, err := os.ReadFile("testdata/multi-source.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Input     clip.PlanningInput
		Responses []llm.Response
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Input.Analyses) != 8 || len(fixture.Responses) != 2 {
		t.Fatal("incomplete recorded multi-source evidence")
	}
	fixture.Input.Policy = testPolicy("write")
	for i, response := range fixture.Responses {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			s, models, captions := newService(t, response.Text, true)
			models.response = response
			got, usage, err := s.Plan(t.Context(), testRef(), fixture.Input)
			if err != nil || len(got.Cuts) != 4 || got.DurationMS != 15000 || got.Ratio != "vertical" {
				t.Fatalf("recorded plan rejected: %+v %v", got, err)
			}
			if len(models.calls) != 1 || captions.calls != 4 || usage != response.Usage {
				t.Fatal("lost call, caption or usage evidence")
			}
		})
	}
}

func multiSourcePlan() (clip.PlanningInput, map[string]any) {
	in := clip.PlanningInput{Policy: testPolicy("write"), Template: clip.Recipe{Name: "여덟 장면", CopyStyles: []string{"diary"}, Accent: "amber"}, Ratio: "vertical", TargetDurationMS: 15000}
	durations := []int{4290, 3744, 1480, 5010, 5108, 4508, 5428, 6702}
	lengths := []int{2000, 2000, 1000, 2400, 2300, 2300, 2200, 2200}
	var cuts []any
	for i, duration := range durations {
		id := fmt.Sprintf("%032x", i+1)
		width, height := 1440, 1920
		if i == 5 {
			width, height = height, width
		}
		in.Analyses = append(in.Analyses, clip.SourceAnalysis{
			Source:   clip.AnalysisSource{RenderSource: clip.RenderSource{ID: id, Fingerprint: id, Info: clip.MediaInfo{DurationMS: duration, Width: width, Height: height, HasAudio: true}}, Filename: fmt.Sprintf("synthetic-%d.mp4", i)},
			Segments: []clip.Segment{{EndMS: duration, Event: "합성 도형이 움직인다", Subjects: []string{"도형"}, Quality: "sharp", Focal: clip.Point{X: .5, Y: .5}}},
		})
		cuts = append(cuts, map[string]any{"id": fmt.Sprintf("cut-%d", i), "source_id": id, "start_ms": 0, "end_ms": lengths[i], "volume": 1,
			"focal": map[string]any{"x": .5, "y": .5}, "caption": map[string]any{"text": fmt.Sprintf("장면 %d", i+1), "start_ms": 0, "end_ms": lengths[i], "position": "bottom", "style": "diary", "accent": "amber"}})
	}
	return in, map[string]any{"ratio": "vertical", "duration_ms": 15000, "cuts": cuts}
}

func TestEightShortSourcesComposeWithExactFadeTimeline(t *testing.T) {
	for _, structured := range []bool{false, true} {
		in, wire := multiSourcePlan()
		s, models, captions := newService(t, raw(wire), structured)
		got, usage, err := s.Plan(t.Context(), testRef(), in)
		if err != nil || len(got.Cuts) != 8 || got.DurationMS != 15000 || got.Ratio != in.Ratio {
			t.Fatalf("multi-source plan: %+v %v", got, err)
		}
		if len(models.calls) != 1 || captions.calls != 8 || usage != models.response.Usage {
			t.Fatal("call/measurement/usage contract changed")
		}
		total := 0
		for i, cut := range got.Cuts {
			total += cut.EndMS - cut.StartMS
			if cut.SourceID != in.Analyses[i].Source.ID || cut.EndMS > in.Analyses[i].Source.Info.DurationMS || cut.Copy.Style != "diary" || cut.Copy.Accent != "amber" {
				t.Fatal("lost source or approved styling", i)
			}
		}
		if total-200*(len(got.Cuts)-1) != got.DurationMS || models.calls[0].HasVideos() || models.calls[0].HasImages() {
			t.Fatal("invalid timeline or non-text planning")
		}
	}
}

func TestMultiSourceOutputDiagnosticsPreserveFailureAndUsage(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		mutate     func(map[string]any)
	}{
		{"timeline", "plan_timeline", func(v map[string]any) { v["duration_ms"] = 15001 }},
		{"source range", "plan_cut_range", func(v map[string]any) { firstCut(v)["end_ms"] = 5000 }},
		{"caption time", "plan_caption_time", func(v map[string]any) { firstCut(v)["caption"].(map[string]any)["end_ms"] = 2001 }},
		{"source identity", "plan_source", func(v map[string]any) { firstCut(v)["source_id"] = "private-canary" }},
		{"duplicate cut", "plan_cut_identity", func(v map[string]any) { v["cuts"].([]any)[1].(map[string]any)["id"] = firstCut(v)["id"] }},
		{"ratio", "plan_ratio", func(v map[string]any) { v["ratio"] = "horizontal" }},
		{"target", "plan_target_duration", func(v map[string]any) { v["duration_ms"] = 18000 }},
		{"style", "plan_style", func(v map[string]any) { firstCut(v)["caption"].(map[string]any)["style"] = "clean" }},
		{"accent", "plan_accent", func(v map[string]any) { firstCut(v)["caption"].(map[string]any)["accent"] = "coral" }},
		{"shape", "output_shape", func(v map[string]any) { v["private-canary"] = "private-canary" }},
		{"field type", "output_field_type", func(v map[string]any) { firstCut(v)["start_ms"] = 1.5 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, wire := multiSourcePlan()
			tc.mutate(wire)
			s, models, captions := newService(t, raw(wire), true)
			_, usage, err := s.Plan(t.Context(), testRef(), in)
			var diagnostic interface{ OutputValidationCode() string }
			if !errors.Is(err, llm.ErrBadOutput) || !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != tc.code {
				t.Fatalf("missing exact safe failure: %v", err)
			}
			if strings.Contains(err.Error(), "private-canary") || llm.NormalizeFailure(err).Reason != llm.FailureReasonOutputInvalid || usage != models.response.Usage || len(models.calls) != 1 || captions.calls != 0 {
				t.Fatal("diagnostics changed privacy, failure, usage or retry behavior")
			}
		})
	}
}
