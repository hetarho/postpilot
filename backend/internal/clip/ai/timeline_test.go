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
		in.Policy = testPolicy("write")
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
		// The second and third sources are used to their last observed frame:
		// under CDS-37 r3 a target ceiling yields when the approved timeline needs
		// it, so only footage that does not exist can leave a cut without room.
		for i, duration := range []int{65000, 6000, 4500} {
			a := base
			a.Source.ID = fmt.Sprintf("source-%d", i)
			a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
			a.Source.Info.DurationMS = duration
			a.Segments = slices.Clone(base.Segments)
			a.Segments[0].EndMS = duration
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
					c := shortCut(fmt.Sprint("late-", i), fmt.Sprintf("source-%d", i%2), 0, 6000)
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
		minimum, maximum := design.CutBounds(scenes[i], "")
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

// CDS-37 r3: a cut with no room to reach its target is left as it is, and the
// plan compiles — the other cuts take the length the approved timeline needs,
// past their own target where they must.
func TestACutThatCannotReachTheTargetIsKeptAndThePlanCompiles(t *testing.T) {
	in := planningInput()
	base := in.Analyses[0]
	in.Analyses = nil
	for i := 0; i < 3; i++ {
		a := base
		a.Source.ID = fmt.Sprintf("source-%d", i)
		a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
		a.Segments = slices.Clone(base.Segments)
		in.Analyses = append(in.Analyses, a)
	}
	// 900 ms of footage in all: under every target, and nowhere to grow.
	in.Analyses[0].Source.Info.DurationMS, in.Analyses[0].Segments[0].EndMS = 900, 900
	cut := func(id, source string, start, end int) map[string]any {
		return map[string]any{"id": id, "source_id": source, "start_ms": start, "end_ms": end, "focal": map[string]any{"x": .5, "y": .5}, "chips": []string{},
			"caption": map[string]any{"text": "여행", "start_ms": 100, "end_ms": end - start - 100, "short_text": "여행", "keyword": ""}}
	}
	wire := map[string]any{"ratio": "vertical", "duration_ms": 15000, "hook": "정확한 여행",
		"cuts": []any{cut("short", "source-0", 0, 900), cut("b", "source-1", 0, 6000), cut("c", "source-2", 0, 6000)}}
	s, _, _ := newService(t, raw(wire), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatalf("a cut under the target refused the plan: %v", err)
	}
	if plan.DurationMS != 15000 || plan.Cuts[0].EndMS != 900 || plan.Cuts[1].EndMS <= 6000 || plan.Cuts[2].EndMS <= 6000 {
		t.Fatalf("plan = %+v", plan.Cuts)
	}
}

// The 카페 preset aims at 4–6 s. Four 4 s cuts hold 16 s against a 15 s target
// and cannot all keep their floor: the target yields and the plan compiles at
// the approved length rather than failing.
func TestPresetTargetYieldsToTheApprovedDuration(t *testing.T) {
	in := planningInput()
	in.Template.Preset = "cafe"
	base := in.Analyses[0]
	in.Analyses = nil
	cuts := []any{}
	for i := 0; i < 4; i++ {
		a := base
		a.Source.ID = fmt.Sprintf("source-%d", i)
		a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
		a.Segments = slices.Clone(base.Segments)
		in.Analyses = append(in.Analyses, a)
		// 4.2 s each: 16.8 s, outside the tolerance of a 15 s target, and the
		// preset's floor holds only 800 ms of the 1.8 s that has to go.
		cuts = append(cuts, map[string]any{"id": fmt.Sprint("cut-", i), "source_id": a.Source.ID, "start_ms": 0, "end_ms": 4200, "focal": map[string]any{"x": .5, "y": .5}, "chips": []string{},
			"caption": map[string]any{"text": "여행", "start_ms": 200, "end_ms": 1500, "short_text": "여행", "keyword": ""}})
	}
	s, _, _ := newService(t, raw(map[string]any{"ratio": "vertical", "duration_ms": 16800, "hook": "정확한 여행", "cuts": cuts}), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatalf("the preset's floor refused a reachable timeline: %v", err)
	}
	total := 0
	for _, c := range plan.Cuts {
		total += c.EndMS - c.StartMS
		if minimum, _ := design.CutBounds("scenery", "cafe"); c.EndMS-c.StartMS >= minimum {
			t.Fatalf("cut %s kept the preset floor at the target's expense: %+v", c.ID, c)
		}
	}
	if plan.DurationMS != 15000 || total-plan.TransitionTotal() != 15000 {
		t.Fatalf("duration %d total %d", plan.DurationMS, total)
	}
	// The same four cuts under the shared range hold their 1.2 s floor and shrink
	// evenly inside the first pass.
	in.Template.Preset = ""
	s, _, _ = newService(t, raw(map[string]any{"ratio": "vertical", "duration_ms": 16800, "hook": "정확한 여행", "cuts": cuts}), true)
	if plan, _, err = s.Plan(t.Context(), testRef(), in); err != nil || plan.DurationMS != 15000 {
		t.Fatalf("shared range: %+v %v", plan.DurationMS, err)
	}
}

// A target the footage cannot hold is not the model's error. When every cut
// sits at its scene's ceiling or the end of what was observed, the compiler
// delivers the length the footage holds — as long as the length floor accepts
// it; TestComposeCannotFillTargetFromUnobservedOrReusedFootage keeps the
// refusal for a clip that would fall under the floor.
func TestFootageBoundClipShipsAtTheLengthItHolds(t *testing.T) {
	in := planningInput()
	base := in.Analyses[0]
	in.Analyses = nil
	for i := 0; i < 3; i++ {
		a := base
		a.Source.ID = fmt.Sprintf("source-%d", i)
		a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
		a.Segments = slices.Clone(base.Segments)
		in.Analyses = append(in.Analyses, a)
	}
	scene, _ := clip.CutScene(clip.Cut{SourceID: "source-0"}, in.Analyses[0])
	_, ceiling := design.CutBounds(scene, "")
	cut := func(id, source string, start, end int) map[string]any {
		return map[string]any{"id": id, "source_id": source, "start_ms": start, "end_ms": end, "focal": map[string]any{"x": .5, "y": .5}, "chips": []string{},
			"caption": map[string]any{"text": "여행", "start_ms": 200, "end_ms": end - start - 200, "short_text": "여행", "keyword": ""}}
	}
	// Every source is used to its last observed frame: the footage holds
	// 2×ceiling + 4.5 s, the target asks for 3 s more, and no pass has room.
	in.Analyses[0].Source.Info.DurationMS, in.Analyses[0].Segments[0].EndMS = ceiling, ceiling
	in.Analyses[1].Source.Info.DurationMS, in.Analyses[1].Segments[0].EndMS = ceiling, ceiling
	in.Analyses[2].Source.Info.DurationMS, in.Analyses[2].Segments[0].EndMS = 4500, 4500
	held := 2*ceiling + 4500
	in.TargetDurationMS = held + 3000
	wire := map[string]any{"ratio": "vertical", "duration_ms": in.TargetDurationMS, "hook": "정확한 여행",
		"cuts": []any{cut("a", "source-0", 0, ceiling), cut("b", "source-1", 0, ceiling), cut("c", "source-2", 0, 4500)}}
	s, _, _ := newService(t, raw(wire), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatalf("a footage-bound clip above the floor was refused: %v", err)
	}
	if plan.DurationMS != held-plan.TransitionTotal() || plan.DurationMS >= in.TargetDurationMS {
		t.Fatalf("duration %d, want the %d the footage holds less %d of transitions", plan.DurationMS, held, plan.TransitionTotal())
	}
}

// The observer reports one segment per event, so a cut across a boundary sits
// in observed footage on both sides; only a gap is unobserved. Growth may run
// on into the touching segment.
func TestACutMayGrowAcrossTouchingSegments(t *testing.T) {
	in := planningInput()
	base := in.Analyses[0]
	in.Analyses = nil
	for i := 0; i < 3; i++ {
		a := base
		a.Source.ID = fmt.Sprintf("source-%d", i)
		a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
		a.Segments = slices.Clone(base.Segments)
		in.Analyses = append(in.Analyses, a)
	}
	scene, _ := clip.CutScene(clip.Cut{SourceID: "source-0"}, in.Analyses[0])
	_, ceiling := design.CutBounds(scene, "")
	first, second := base.Segments[0], base.Segments[0]
	first.EndMS, second.StartMS = 2800, 2800
	in.Analyses[0].Segments = []clip.Segment{first, second}
	cut := func(id, source string, start, end int) map[string]any {
		return map[string]any{"id": id, "source_id": source, "start_ms": start, "end_ms": end, "focal": map[string]any{"x": .5, "y": .5}, "chips": []string{},
			"caption": map[string]any{"text": "여행", "start_ms": 200, "end_ms": end - start - 200, "short_text": "여행", "keyword": ""}}
	}
	// Only the straddling cut has room; it must reach its ceiling to meet the target.
	in.TargetDurationMS = 2*ceiling + ceiling
	wire := map[string]any{"ratio": "vertical", "duration_ms": in.TargetDurationMS, "hook": "정확한 여행",
		"cuts": []any{cut("straddle", "source-0", 1000, 4000), cut("b", "source-1", 0, ceiling), cut("c", "source-2", 0, ceiling)}}
	s, _, _ := newService(t, raw(wire), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatalf("growth across touching segments refused: %v", err)
	}
	if got := plan.Cuts[0].EndMS; got != 1000+ceiling {
		t.Fatalf("straddling cut ends at %d, want %d", got, 1000+ceiling)
	}
}
