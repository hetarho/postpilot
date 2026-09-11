package ai_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
)

// Reproduce the timing shape of the failed owner-authorized live composition.
// IDs, observations and copy are synthetic; no private capture is checked in.
func failedTimingPlan() (clip.PlanningInput, map[string]any) {
	in, wire := multiSourcePlan()
	all := wire["cuts"].([]any)
	indexes := []int{0, 3, 6, 7}
	starts, ends := []int{0, 500, 1000, 1000}, []int{3800, 4000, 4000, 4000}
	var cuts []any
	for i, index := range indexes {
		c := all[index].(map[string]any)
		c["start_ms"], c["end_ms"] = starts[i], ends[i]
		p := c["caption"].(map[string]any)
		p["start_ms"], p["end_ms"] = 400, 3600
		if i == 0 {
			p["start_ms"], p["end_ms"] = 500, 3500
		}
		cuts = append(cuts, c)
	}
	wire["cuts"], wire["duration_ms"] = cuts, 15200 // Actual fade timeline: 12700.
	return in, wire
}

func TestComposeActualFailureTimingWithoutAnotherPaidCall(t *testing.T) {
	for _, structured := range []bool{false, true} {
		in, wire := failedTimingPlan()
		s, models, captions := newService(t, raw(wire), structured)
		got, usage, err := s.Plan(t.Context(), testRef(), in)
		if err != nil {
			t.Fatal(err)
		}
		assertExecutableTimeline(t, in, got)
		if got.DurationMS != 15000 || len(got.Cuts) != 4 || captions.calls != 4 || len(models.calls) != 1 || usage != models.response.Usage {
			t.Fatal("composition changed target, selected cuts, calls or settlement")
		}
		for i, c := range got.Cuts {
			if c.EndMS != []int{4290, 4604, 4604, 4602}[i] {
				t.Fatal("unexpected bounded timeline distribution")
			}
			original := wire["cuts"].([]any)[i].(map[string]any)
			p := original["caption"].(map[string]any)
			if c.SourceID != original["source_id"] || c.StartMS != original["start_ms"] || c.Copy.Text != p["text"] || c.Copy.StartMS != p["start_ms"] || c.Copy.Style != "memo" || c.Copy.Accent != "amber" || c.OriginalVolume() != 1 {
				t.Fatal("lost selected footage, copy, style or original audio")
			}
		}
		again, _, err := s.Plan(t.Context(), testRef(), in)
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatal("timing compilation is not deterministic", err)
		}
	}
}

func assertExecutableTimeline(t *testing.T, in clip.PlanningInput, got clip.EditPlan) {
	t.Helper()
	var sources []clip.RenderSource
	for _, a := range in.Analyses {
		sources = append(sources, a.Source.RenderSource)
	}
	// The production renderer's unmodified strict validator remains authority.
	if err := clip.ValidateEditPlan(config.ClipRender(&config.Config{}), got, sources); err != nil {
		t.Fatal(err)
	}
}

func TestComposeCorrectsArithmeticAndExposureButKeepsValidTiming(t *testing.T) {
	for _, mode := range []string{"valid", "declared sum", "declared below minimum", "declared over maximum", "declared wrong target", "caption end", "shorten", "extend"} {
		t.Run(mode, func(t *testing.T) {
			in, wire := planningInput(), plan()
			c := firstCut(wire)
			p := c["caption"].(map[string]any)
			switch mode {
			case "declared sum":
				wire["duration_ms"] = 15001
			case "declared below minimum":
				wire["duration_ms"] = 14999
			case "declared over maximum":
				wire["duration_ms"] = 90001
			case "declared wrong target":
				wire["duration_ms"] = 17000
				c["end_ms"] = 17000
			case "caption end":
				p["end_ms"] = 15001
			case "shorten":
				c["end_ms"] = 20000
				p["end_ms"] = 19000
			case "extend":
				c["end_ms"] = 10000
				p["end_ms"] = 9000
			}
			s, models, _ := newService(t, raw(wire), true)
			got, usage, err := s.Plan(t.Context(), testRef(), in)
			if err != nil || got.DurationMS != 15000 || usage != models.response.Usage || len(models.calls) != 1 {
				t.Fatalf("compilation: %+v %v", got, err)
			}
			assertExecutableTimeline(t, in, got)
			if got.Cuts[0].Copy.StartMS != 1000 || got.Cuts[0].Copy.EndMS != min(p["end_ms"].(int), 15000) {
				t.Fatal("changed existing caption exposure other than clipping at cut end")
			}
		})
	}
}

func TestComposeCannotFillTargetFromUnobservedOrReusedFootage(t *testing.T) {
	for _, mode := range []string{"source exhausted", "scene boundary", "observation gap", "next selected cut", "existing overlap", "caption cannot shrink"} {
		t.Run(mode, func(t *testing.T) {
			in, wire := planningInput(), plan()
			c := firstCut(wire)
			c["end_ms"] = 10000
			c["caption"].(map[string]any)["end_ms"] = 9000
			a := &in.Analyses[0]
			switch mode {
			case "source exhausted":
				a.Source.Info.DurationMS, a.Segments[0].EndMS = 10000, 10000
			case "scene boundary":
				a.Segments[0].EndMS = 10000
			case "observation gap":
				a.Segments[0].StartMS = 100
			case "next selected cut", "existing overlap":
				next := firstCut(plan())
				next["id"] = "next"
				next["start_ms"], next["end_ms"] = 10000, 14000
				if mode == "existing overlap" {
					next["start_ms"], next["end_ms"] = 0, 4000
				}
				next["caption"].(map[string]any)["end_ms"] = 3000
				wire["cuts"] = []any{c, next}
				if mode == "next selected cut" {
					a.Segments[0].EndMS = 14000
				}
			case "caption cannot shrink":
				c["end_ms"] = 20000
				c["caption"].(map[string]any)["start_ms"] = 18000
				c["caption"].(map[string]any)["end_ms"] = 19000
			}
			s, models, measure := newService(t, raw(wire), true)
			_, usage, err := s.Plan(t.Context(), testRef(), in)
			var diagnostic interface{ OutputValidationCode() string }
			if !errors.Is(err, llm.ErrBadOutput) || !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != "plan_timeline" || len(models.calls) != 1 || measure.calls != 0 || usage != models.response.Usage {
				t.Fatalf("unsafe timeline adjustment or paid retry: %v", err)
			}
		})
	}
}
