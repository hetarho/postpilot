package ai_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// multiSourceFlow is eight short synthetic sources of one scene and a flow that
// takes one cut from each. The third cut is 1200 ms, CDS-37's own floor; the
// eight sum to 16400 and the target pulls them back to 15000.
func multiSourceFlow() (clip.PlanningInput, map[string]any) {
	composed := clip.NoTemplateComposition()
	in := clip.PlanningInput{Policy: testPolicy("write"), Composition: &composed, Ratio: "vertical", TargetDurationMS: 15000}
	durations := []int{4290, 3744, 1480, 5010, 5108, 4508, 5428, 6702}
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
			Segments: []clip.Segment{{EndMS: duration, Event: "합성 도형이 움직인다", Subjects: []string{"도형"}, Quality: "sharp", Focal: clip.Point{X: .5, Y: .5}, Scene: "scenery", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}},
		})
		cuts = append(cuts, map[string]any{"id": fmt.Sprintf("cut-%d", i), "source_id": id, "start_ms": 0, "end_ms": lengths[i], "rate_permille": 1000, "volume": 1,
			"focal": map[string]any{"x": .5, "y": .5}, "observation_refs": []string{clip.ObservationID(id, 0)}})
	}
	return in, map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": 15000, "cuts": cuts}
}

func TestEightShortSourcesComposeWithExactTransitionTimeline(t *testing.T) {
	for _, structured := range []bool{false, true} {
		in, wire := multiSourceFlow()
		s, models := newService(t, raw(wire), structured)
		in.Policy = testPolicy("write")
		got, usage, err := s.Flow(t.Context(), testRef(), in)
		if err != nil || len(got.Cuts) != 8 || got.DurationMS != in.TargetDurationMS || !hasNotice(got, "plan_target_duration") || got.Ratio != in.Ratio {
			t.Fatalf("multi-source flow: %+v %v", got, err)
		}
		if len(models.calls) != 1 || usage != models.response.Usage || models.calls[0].HasVideos() || models.calls[0].HasImages() {
			t.Fatalf("call/usage contract changed: %d calls", len(models.calls))
		}
		total := 0
		for i, cut := range got.Cuts {
			total += cut.OutputDurationMS()
			if cut.SourceID != in.Analyses[i].Source.ID || cut.EndMS > in.Analyses[i].Source.Info.DurationMS {
				t.Fatal("lost source", i)
			}
		}
		// Eight cuts of one scene: CDS-36 joins every boundary with a hard cut,
		// so the clip is exactly as long as the footage it selected.
		if got.TransitionTotal() != 0 || total-got.TransitionTotal() != got.DurationMS {
			t.Fatal("invalid timeline")
		}
		assertExecutableTimeline(t, in, got)
	}
}

func TestMultiSourceOutputDiagnosticsPreserveFailureAndUsage(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		mutate     func(map[string]any)
	}{
		{"source range", "plan_cut_range", func(v map[string]any) { firstCut(v)["end_ms"] = 5000 }},
		{"source identity", "plan_source", func(v map[string]any) { firstCut(v)["source_id"] = "private-canary" }},
		{"duplicate cut", "plan_cut_identity", func(v map[string]any) { v["cuts"].([]any)[1].(map[string]any)["id"] = firstCut(v)["id"] }},
		{"ratio", "plan_ratio", func(v map[string]any) { v["ratio"] = "horizontal" }},
		// Words, styles and accents are the narration's, not the flow's: a
		// caption on a cut is an unknown property of this contract.
		{"caption", "output_shape", func(v map[string]any) { firstCut(v)["caption"] = map[string]any{"text": "조용한 장면"} }},
		{"shape", "output_shape", func(v map[string]any) { v["private-canary"] = "private-canary" }},
		{"field type", "output_field_type", func(v map[string]any) { firstCut(v)["start_ms"] = 1.5 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, wire := multiSourceFlow()
			tc.mutate(wire)
			s, models := newService(t, raw(wire), true)
			delivered, usage, err := s.Flow(t.Context(), testRef(), in)
			if !strings.HasPrefix(tc.code, "output_") {
				if err != nil || !hasNotice(delivered, tc.code) || usage != models.response.Usage || len(models.calls) != 1 {
					t.Fatalf("readable correction lost its notice or usage: %v %+v", err, delivered.Notices)
				}
				assertExecutableTimeline(t, in, delivered)
				return
			}
			var diagnostic interface{ OutputValidationCode() string }
			if !errors.Is(err, llm.ErrBadOutput) || !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != tc.code {
				t.Fatalf("missing exact safe failure: %v", err)
			}
			if strings.Contains(err.Error(), "private-canary") || llm.NormalizeFailure(err).Reason != llm.FailureReasonOutputInvalid || usage != models.response.Usage || len(models.calls) != 1 {
				t.Fatal("diagnostics changed privacy, failure, usage or retry behavior")
			}
		})
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestNarrationKeepsTheMixedStylesItNames(t *testing.T) {
	in := narrationInput(t)
	in.Design.CaptionStyles = []string{"keynote", "word-pop", "film"}
	first := narrationCaption("첫 장면입니다", 1000, 5000)
	second := narrationCaption("다음 장면입니다", 6000, 10000)
	first["style"], second["style"] = "film", "word-pop"
	plan, payload, system := narrate(t, in, narrationResponse(first, second))
	captions := narrationOf(plan)
	if len(captions) != 2 || captions[0].Resolved.Element.Style != "film" || captions[1].Resolved.Element.Style != "word-pop" {
		t.Fatalf("the narration lost its mixed treatments: %+v", captions)
	}
	styles := payload["allowed_caption_styles"].([]any)
	if len(styles) != 3 || styles[0].(map[string]any)["id"] != "keynote" {
		t.Fatal("the prompt did not carry this project's selection", styles)
	}
	for _, style := range styles {
		if style.(map[string]any)["reads_as"] == "" {
			t.Fatal("the writer was given a style id without its meaning", style)
		}
	}
	if strings.Contains(system, "Do not choose a style") || !strings.Contains(system, "vary it across the clip") {
		t.Fatal("the prompt still forbids mixed caption styles")
	}
}
