package ai_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
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
	// All four cuts show the same scene, so CDS-36 joins them with hard cuts and
	// the selected footage is the whole timeline: 13300 against a 15000 target.
	wire["cuts"], wire["duration_ms"] = cuts, 15200
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
		// Up to one measurement per candidate anchor and no more (CDS-38).
		if got.DurationMS != 15000 || len(got.Cuts) != 4 || captions.calls > 8 || captions.calls < 4 || len(models.calls) != 1 || usage != models.response.Usage {
			t.Fatal("composition changed target, selected cuts, calls or settlement")
		}
		for i, c := range got.Cuts {
			if c.EndMS != []int{4225, 4425, 4425, 4425}[i] {
				t.Fatal("unexpected bounded timeline distribution")
			}
			original := wire["cuts"].([]any)[i].(map[string]any)
			p := original["caption"].(map[string]any)
			// The footage the model selected is kept exactly; the copy is its
			// words (or the shorter alternative it supplied), and the placement
			// and window are the design system's (CDS-7, CDS-27).
			if c.SourceID != original["source_id"] || c.StartMS != original["start_ms"] || c.OriginalVolume() != 1 {
				t.Fatal("lost selected footage or original audio")
			}
			if c.FirstCopy().Text != p["text"] && c.FirstCopy().Text != p["short_text"] {
				t.Fatalf("copy %q is neither what was written nor its alternative", c.FirstCopy().Text)
			}
			if c.FirstCopy().Accent != "amber" || !slices.Contains(in.Template.CopyStyles, c.FirstCopy().Style) {
				t.Fatalf("cut %d styling: %+v", i, c.FirstCopy())
			}
			if start, end := c.CaptionWindow(0); start != 120 || end != c.EndMS-c.StartMS-120 {
				t.Fatalf("cut %d window %d..%d", i, start, end)
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
				// The declared total is the model's arithmetic; the timeline is the
				// caller's, and it is the one that counts.
				wire["duration_ms"] = 17000
			case "caption end":
				p["end_ms"] = 15001
			case "shorten", "extend":
				// Every cut is resized INSIDE CDS-37's bounds, so what the target
				// reconciles is a real overshoot or shortfall and not a cut the
				// length rule would have trimmed anyway.
				length := 6000
				if mode == "extend" {
					length = 3000
				}
				for _, value := range wire["cuts"].([]any) {
					v := value.(map[string]any)
					v["end_ms"] = v["start_ms"].(int) + length
				}
			}
			s, models, _ := newService(t, raw(wire), true)
			got, usage, err := s.Plan(t.Context(), testRef(), in)
			if err != nil || got.DurationMS != 15000 || usage != models.response.Usage || len(models.calls) != 1 {
				t.Fatalf("compilation: %+v %v", got, err)
			}
			assertExecutableTimeline(t, in, got)
			// The copy's window is CDS-27's, whatever the model asked for: cut
			// start + 120 ms to cut end − 120 ms, and it always fits the cut.
			cut := got.Cuts[0]
			start, end := cut.CaptionWindow(0)
			if start != 120 || end != cut.EndMS-cut.StartMS-120 || end <= start {
				t.Fatalf("caption window %d..%d in a %d ms cut", start, end, cut.EndMS-cut.StartMS)
			}
		})
	}
}

// CDS-37 caps every cut at 6.0 s, so the room a plan has to reach its target is
// itself bounded: each case here holds two cuts at that ceiling and blocks the
// one cut that still had room, in a different way each time. Without the block
// the same plan compiles, which is what makes the block the thing being tested.
func TestComposeCannotFillTargetFromUnobservedOrReusedFootage(t *testing.T) {
	// Each cut takes its own source, so one case's block reaches one cut.
	shortInput := func() clip.PlanningInput {
		in := planningInput()
		base := in.Analyses[0]
		in.Analyses = nil
		for i := 0; i < 3; i++ {
			a := base
			a.Source.ID = fmt.Sprintf("source-%d", i)
			a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
			a.Source.Info.DurationMS = 65000
			a.Segments = slices.Clone(base.Segments)
			in.Analyses = append(in.Analyses, a)
		}
		return in
	}
	shortCut := func(id, source string, start, end int) map[string]any {
		return map[string]any{"id": id, "source_id": source, "start_ms": start, "end_ms": end, "focal": map[string]any{"x": .5, "y": .5}, "chips": []string{},
			"caption": map[string]any{"text": "여행", "start_ms": 200, "end_ms": end - start - 200, "short_text": "여행", "keyword": ""}}
	}
	for _, mode := range []string{"unblocked", "source exhausted", "scene boundary", "observation gap", "next selected cut", "existing overlap", "caption cannot shrink"} {
		t.Run(mode, func(t *testing.T) {
			in := shortInput()
			// 1.5 s + 6.0 s + 4.5 s = 12 s against a 15 s target: only the first
			// cut has room left, and every case but the first takes it away.
			cuts := []any{shortCut("short", "source-0", 0, 1500), shortCut("full", "source-1", 0, 6000), shortCut("spare", "source-2", 0, 4500)}
			a := &in.Analyses[0]
			switch mode {
			case "source exhausted":
				a.Source.Info.DurationMS, a.Segments[0].EndMS = 1500, 1500
			case "scene boundary":
				a.Segments[0].EndMS = 1500
			case "observation gap":
				// The cut starts before anything was observed, so its segment
				// never contains it and lends it nothing.
				a.Segments[0].StartMS = 100
			case "next selected cut":
				// Both cuts come off the one source: the second already occupies
				// everything observed past the first, which therefore has no
				// room, and the second is itself at the end of its segment.
				cuts = []any{cuts[0], shortCut("next", "source-0", 1500, 7500)}
				a.Segments[0].EndMS = 7500
			case "existing overlap":
				// The same source range is reused by a longer cut, so the short
				// one may not be expanded into it.
				cuts = []any{cuts[0], shortCut("reused", "source-0", 0, 6000)}
				a.Segments[0].EndMS = 6000
			case "caption cannot shrink":
				// The other direction: four cuts over the target, each holding
				// its copy so late that almost nothing may be trimmed.
				cuts = nil
				for i := 0; i < 4; i++ {
					c := shortCut(fmt.Sprint("late-", i), fmt.Sprintf("source-%d", i%3), 0, 6000)
					c["caption"].(map[string]any)["start_ms"] = 5000
					c["caption"].(map[string]any)["end_ms"] = 5900
					cuts = append(cuts, c)
				}
			}
			wire := map[string]any{"ratio": "vertical", "duration_ms": 15000, "hook": "정확한 여행", "cuts": cuts}
			s, models, measure := newService(t, raw(wire), true)
			_, usage, err := s.Plan(t.Context(), testRef(), in)
			if mode == "unblocked" {
				if err != nil {
					t.Fatalf("the same plan must compile when nothing blocks it: %v", err)
				}
				return
			}
			var diagnostic interface{ OutputValidationCode() string }
			if !errors.Is(err, llm.ErrBadOutput) || !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != "plan_timeline" || len(models.calls) != 1 || measure.calls != 0 || usage != models.response.Usage {
				t.Fatalf("unsafe timeline adjustment or paid retry: %v", err)
			}
		})
	}
}

// The compiler is the only thing that chooses a transition or a cut length: the
// scenes the observer reported decide both (CDS-36, CDS-37).
func TestCompilerJoinsScenesAndHoldsCutLengths(t *testing.T) {
	scenes := []string{"food", "menu", "menu", "scenery", "food", "interior"}
	in := planningInput()
	base := in.Analyses[0]
	in.Analyses = nil
	cuts := []any{}
	for i, scene := range scenes {
		a := base
		a.Source.ID = fmt.Sprintf("source-%d", i)
		a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
		a.Source.Info.DurationMS = 65000
		a.Segments = []clip.Segment{{StartMS: 0, EndMS: 65000, Event: "장면", Subjects: []string{"접시"}, Quality: "steady", Focal: clip.Point{X: .5, Y: .5}, Scene: scene}}
		in.Analyses = append(in.Analyses, a)
		// A 9 s cut on a food close-up is more than twice CDS-37's ceiling for
		// one, and the others are over the shared ceiling.
		cuts = append(cuts, map[string]any{"id": fmt.Sprint("cut-", i), "source_id": a.Source.ID, "start_ms": 0, "end_ms": 9000, "focal": map[string]any{"x": .5, "y": .5}, "chips": []string{},
			"caption": map[string]any{"text": "여행", "start_ms": 200, "end_ms": 1500, "short_text": "여행", "keyword": ""}})
	}
	in.TargetDurationMS = 30000
	s, _, _ := newService(t, raw(map[string]any{"ratio": "vertical", "duration_ms": 30000, "hook": "정확한 여행", "cuts": cuts}), true)
	got, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	// Four scene changes over five boundaries, and 40 % of five is two: the two
	// EARLIEST changes fade and the rest are hard cuts.
	want := []int{0, 200, 0, 200, 0, 0}
	for i, cut := range got.Cuts {
		if cut.TransitionMS != want[i] {
			t.Fatalf("cut %d joined with %d ms, want %d: %v", i, cut.TransitionMS, want[i], want)
		}
		minimum, maximum := design.CutBounds(scenes[i])
		if length := cut.EndMS - cut.StartMS; length < minimum || length > maximum {
			t.Fatalf("cut %d is %d ms, outside %s's %d..%d", i, length, scenes[i], minimum, maximum)
		}
	}
	total := 0
	for _, cut := range got.Cuts {
		total += cut.EndMS - cut.StartMS
	}
	if total-got.TransitionTotal() != got.DurationMS {
		t.Fatal("the timeline does not account for its own transitions", total, got.TransitionTotal(), got.DurationMS)
	}
}

// A cut with no room to reach CDS-37's floor is footage too short to read, and
// the plan is refused rather than shipped under it.
func TestACutThatCannotReachTheFloorIsRefused(t *testing.T) {
	in := planningInput()
	in.Analyses[0].Source.Info.DurationMS = 900
	in.Analyses[0].Segments[0].EndMS = 900
	wire := plan()
	cuts := wire["cuts"].([]any)[:1]
	c := cuts[0].(map[string]any)
	c["end_ms"] = 900
	c["caption"].(map[string]any)["start_ms"], c["caption"].(map[string]any)["end_ms"] = 100, 800
	wire["cuts"] = cuts
	s, _, measure := newService(t, raw(wire), true)
	_, _, err := s.Plan(t.Context(), testRef(), in)
	var diagnostic interface{ OutputValidationCode() string }
	if !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != "plan_cut_length" || measure.calls != 0 {
		t.Fatalf("a cut under the floor was composed anyway: %v", err)
	}
}
