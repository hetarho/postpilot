package ai_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
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
			// One paid call, and at most one measurement per candidate anchor.
			if len(models.calls) != 1 || captions.calls > 2*len(got.Cuts) || captions.calls < len(got.Cuts) || usage != response.Usage {
				t.Fatalf("lost call, caption or usage evidence: %d measurements", captions.calls)
			}
		})
	}
}

func multiSourcePlan() (clip.PlanningInput, map[string]any) {
	in := clip.PlanningInput{Policy: testPolicy("write"), Template: clip.Recipe{Name: "여덟 장면", CopyStyles: []string{"clean", "memo"}, Accent: "amber"}, Ratio: "vertical", TargetDurationMS: 15000}
	durations := []int{4290, 3744, 1480, 5010, 5108, 4508, 5428, 6702}
	// The third cut is 1200 ms, not 1000: a three-character copy earns 1170 ms of
	// exposure (CDS-41) and a cut cannot be shorter than the copy it carries. The
	// eight still sum to 16400, the fade timeline's 15000.
	lengths := []int{2000, 2000, 1200, 2200, 2300, 2300, 2200, 2200}
	var cuts []any
	for i, duration := range durations {
		id := fmt.Sprintf("%032x", i+1)
		width, height := 1440, 1920
		if i == 5 {
			width, height = height, width
		}
		in.Analyses = append(in.Analyses, clip.SourceAnalysis{
			Source:   clip.AnalysisSource{RenderSource: clip.RenderSource{ID: id, Fingerprint: id, Info: clip.MediaInfo{DurationMS: duration, Width: width, Height: height, HasAudio: true}}, Filename: fmt.Sprintf("synthetic-%d.mp4", i)},
			Segments: []clip.Segment{{EndMS: duration, Event: "합성 도형이 움직인다", Subjects: []string{"도형"}, Quality: "sharp", Focal: clip.Point{X: .5, Y: .5}, Scene: "scenery"}},
		})
		cuts = append(cuts, map[string]any{"id": fmt.Sprintf("cut-%d", i), "source_id": id, "start_ms": 0, "end_ms": lengths[i], "volume": 1, "chips": []string{},
			"focal": map[string]any{"x": .5, "y": .5}, "caption": map[string]any{"text": "조용한 장면", "start_ms": 0, "end_ms": lengths[i], "short_text": "장면", "keyword": ""}})
	}
	return in, map[string]any{"ratio": "vertical", "duration_ms": 15000, "hook": "여덟 장면", "cuts": cuts}
}

func TestEightShortSourcesComposeWithExactFadeTimeline(t *testing.T) {
	for _, structured := range []bool{false, true} {
		in, wire := multiSourcePlan()
		s, models, captions := newService(t, raw(wire), structured)
		got, usage, err := s.Plan(t.Context(), testRef(), in)
		if err != nil || len(got.Cuts) != 8 || got.DurationMS != 15000 || got.Ratio != in.Ratio {
			t.Fatalf("multi-source plan: %+v %v", got, err)
		}
		// One paid call, and at most one measurement per candidate anchor: the
		// selector measures what it might choose, never more (CDS-38).
		if len(models.calls) != 1 || captions.calls > 2*len(got.Cuts) || captions.calls < len(got.Cuts) || usage != models.response.Usage {
			t.Fatalf("call/measurement/usage contract changed: %d calls, %d measurements", len(models.calls), captions.calls)
		}
		total, dropped, styles := 0, 0, map[string]int{}
		for i, cut := range got.Cuts {
			total += cut.EndMS - cut.StartMS
			if cut.SourceID != in.Analyses[i].Source.ID || cut.EndMS > in.Analyses[i].Source.Info.DurationMS {
				t.Fatal("lost source", i)
			}
			// A cut too short for its copy's earned exposure carries none: CDS-41
			// shortens, then extends, then drops, and the choice is recorded.
			if cut.Copy.Text == "" {
				if got.Decisions[i].Fallback != "dropped" {
					t.Fatalf("cut %d lost its copy silently: %+v", i, got.Decisions[i])
				}
				dropped++
				continue
			}
			if cut.Copy.Accent != "amber" || !slices.Contains(in.Template.CopyStyles, cut.Copy.Style) {
				t.Fatalf("cut %d used %q outside the approved set", i, cut.Copy.Style)
			}
			styles[cut.Copy.Style]++
		}
		if total-200*(len(got.Cuts)-1) != got.DurationMS || models.calls[0].HasVideos() || models.calls[0].HasImages() {
			t.Fatal("invalid timeline or non-text planning")
		}
		if len(styles) < 2 {
			t.Fatalf("eight identical sentences all took one style: %v", styles)
		}
		// Exactly the 1200 ms cut is too short to earn its copy's exposure.
		if dropped != 1 {
			t.Fatalf("%d copies dropped", dropped)
		}
	}
}

func TestMultiSourceOutputDiagnosticsPreserveFailureAndUsage(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		mutate     func(map[string]any)
	}{
		{"source range", "plan_cut_range", func(v map[string]any) { firstCut(v)["end_ms"] = 5000 }},
		{"caption time", "plan_caption_time", func(v map[string]any) { firstCut(v)["caption"].(map[string]any)["start_ms"] = 2000 }},
		{"source identity", "plan_source", func(v map[string]any) { firstCut(v)["source_id"] = "private-canary" }},
		{"duplicate cut", "plan_cut_identity", func(v map[string]any) { v["cuts"].([]any)[1].(map[string]any)["id"] = firstCut(v)["id"] }},
		{"ratio", "plan_ratio", func(v map[string]any) { v["ratio"] = "horizontal" }},
		// A style or an accent is no longer part of the contract, so naming one
		// is an unknown property, not a disallowed value.
		{"style", "output_shape", func(v map[string]any) { firstCut(v)["caption"].(map[string]any)["style"] = "bold" }},
		{"accent", "output_shape", func(v map[string]any) { firstCut(v)["caption"].(map[string]any)["accent"] = "coral" }},
		{"chip label", "plan_chip_label", func(v map[string]any) { firstCut(v)["chips"] = []string{"주차"} }},
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
