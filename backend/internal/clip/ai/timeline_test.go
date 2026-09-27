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
)

// timelineCut is one flow cut citing every observation it lies in, so a case
// reads as what the writer selected rather than as a stale reference.
func timelineCut(in clip.PlanningInput, id, source string, start, end, rate int) map[string]any {
	refs := []string{}
	for _, a := range in.Analyses {
		if a.Source.ID != source {
			continue
		}
		observed, _ := clip.CutEvidence(in.Analyses, clip.Cut{SourceID: source, Fingerprint: a.Source.Fingerprint, StartMS: start, EndMS: end})
		for _, o := range observed {
			refs = append(refs, o.ID)
		}
	}
	return map[string]any{"id": id, "source_id": source, "start_ms": start, "end_ms": end, "rate_permille": rate,
		"focal": map[string]any{"x": .5, "y": .5}, "volume": 1, "observation_refs": refs}
}

// sources is n copies of planningInput's one observed source under their own
// identities: source-0, source-1, …
func sources(in *clip.PlanningInput, n int) {
	base := in.Analyses[0]
	in.Analyses = nil
	for i := 0; i < n; i++ {
		a := base
		a.Source.ID = fmt.Sprintf("source-%d", i)
		a.Source.Fingerprint = fmt.Sprintf("hash-%d", i)
		a.Segments = slices.Clone(base.Segments)
		in.Analyses = append(in.Analyses, a)
	}
}

// The timing shape of the failed owner-authorized live composition: four cuts
// of one scene holding 13300 ms against a 15000 target. IDs and observations
// are synthetic; no private capture is checked in.
func failedTiming() (clip.PlanningInput, map[string]any) {
	in, wire := multiSourceFlow()
	all := wire["cuts"].([]any)
	indexes := []int{0, 3, 6, 7}
	starts, ends := []int{0, 500, 1000, 1000}, []int{3800, 4000, 4000, 4000}
	var cuts []any
	for i, index := range indexes {
		c := all[index].(map[string]any)
		c["start_ms"], c["end_ms"] = starts[i], ends[i]
		cuts = append(cuts, c)
	}
	wire["cuts"], wire["duration_ms"] = cuts, 15200
	return in, wire
}

func TestComposeActualFailureTimingWithoutAnotherPaidCall(t *testing.T) {
	for _, structured := range []bool{false, true} {
		in, wire := failedTiming()
		s, models := newService(t, raw(wire), structured)
		in.Policy = testPolicy("write")
		got, usage, err := s.Flow(t.Context(), testRef(), in)
		if err != nil {
			t.Fatal(err)
		}
		assertExecutableTimeline(t, in, got)
		if got.DurationMS != 15000 || len(got.Cuts) != 4 || len(models.calls) != 1 || usage != models.response.Usage {
			t.Fatal("composition changed target, selected cuts, calls or settlement")
		}
		// The 1700 ms shortfall is shared evenly: every cut grows 425 ms inside
		// its own observed scene.
		for i, c := range got.Cuts {
			if c.EndMS != []int{4225, 4425, 4425, 4425}[i] {
				t.Fatal("unexpected bounded timeline distribution", got.Cuts)
			}
			original := wire["cuts"].([]any)[i].(map[string]any)
			if c.SourceID != original["source_id"] || c.StartMS != original["start_ms"] || c.OriginalVolume() != 1 {
				t.Fatal("lost selected footage or original audio")
			}
		}
		again, _, err := s.Flow(t.Context(), testRef(), in)
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
	if err := clip.ValidateEditPlan(clip.DefaultRenderConfig(clip.Environment{}), got, sources); err != nil {
		t.Fatal(err)
	}
}

func TestComposeCorrectsArithmeticButKeepsValidTiming(t *testing.T) {
	for _, mode := range []string{"valid", "declared sum", "declared below minimum", "declared over maximum", "declared wrong target", "shorten", "extend"} {
		t.Run(mode, func(t *testing.T) {
			in, wire := planningInput(), flow()
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
			case "shorten", "extend":
				// Every cut is resized INSIDE CDS-37's bounds, so what the target
				// reconciles is a real overshoot or shortfall and not a cut the
				// length rule would have trimmed anyway.
				length := 6000
				if mode == "extend" {
					length = 3000
				}
				// Keep the three same-source cuts on distinct ranges: a longer
				// cut may not be made to overlap the next one (CLIP-98).
				for i, value := range wire["cuts"].([]any) {
					v := value.(map[string]any)
					v["start_ms"] = i * length
					v["end_ms"] = i*length + length
				}
			}
			s, models := newService(t, raw(wire), true)
			got, usage, err := s.Flow(t.Context(), testRef(), in)
			if err != nil || got.DurationMS != 15000 || usage != models.response.Usage || len(models.calls) != 1 {
				t.Fatalf("compilation: %+v %v", got, err)
			}
			assertExecutableTimeline(t, in, got)
		})
	}
}

// CDS-37 caps every cut at 6.0 s, so the room a flow has to reach its target is
// itself bounded: each case here holds two cuts at that ceiling and blocks the
// one cut that still had room, in a different way each time. Without the block
// the same flow compiles, which is what makes the block the thing being tested.
func TestComposeCannotFillTargetFromUnobservedOrReusedFootage(t *testing.T) {
	// Each cut takes its own source, so one case's block reaches one cut.
	shortInput := func() clip.PlanningInput {
		in := planningInput()
		sources(&in, 3)
		// The second and third sources are used to their last observed frame:
		// under CDS-37 r3 a target ceiling yields when the approved timeline needs
		// it, so only footage that does not exist can leave a cut without room.
		for i, duration := range []int{65000, 6000, 4500} {
			in.Analyses[i].Source.Info.DurationMS, in.Analyses[i].Segments[0].EndMS = duration, duration
		}
		return in
	}
	for _, mode := range []string{"unblocked", "source exhausted", "next selected cut"} {
		t.Run(mode, func(t *testing.T) {
			in := shortInput()
			a := &in.Analyses[0]
			switch mode {
			case "source exhausted":
				a.Source.Info.DurationMS, a.Segments[0].EndMS = 1500, 1500
			case "next selected cut":
				a.Source.Info.DurationMS, a.Segments[0].EndMS = 7500, 7500
			}
			// 1.5 s + 6.0 s + 4.5 s = 12 s against a 15 s target: only the first
			// cut has room left, and every case but the first takes it away.
			cuts := []any{timelineCut(in, "short", "source-0", 0, 1500, 1000), timelineCut(in, "full", "source-1", 0, 6000, 1000), timelineCut(in, "spare", "source-2", 0, 4500, 1000)}
			if mode == "next selected cut" {
				// Both cuts come off the one source: the second already occupies
				// everything observed past the first, which therefore has no
				// room, and the source itself ends there.
				cuts = []any{cuts[0], timelineCut(in, "next", "source-0", 1500, 7500, 1000)}
			}
			wire := map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": 15000, "cuts": cuts}
			s, models := newService(t, raw(wire), true)
			_, usage, err := s.Flow(t.Context(), testRef(), in)
			if mode == "unblocked" {
				if err != nil {
					t.Fatalf("the same flow must compile when nothing blocks it: %v", err)
				}
				return
			}
			var diagnostic interface{ OutputValidationCode() string }
			if !errors.Is(err, clip.ErrInsufficientFootage) || !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != "plan_length_floor" || len(models.calls) != 1 || usage != models.response.Usage {
				t.Fatalf("unsafe timeline adjustment or paid retry: %v", err)
			}
			d, ok := clip.DiagnosticFromError(err)
			if !ok || d.Phase != "timeline_grow" || d.Values["remaining_ms"] <= 0 || d.Values["after_ms"] <= 0 || d.Values["target_ms"] != 15000 {
				t.Fatalf("failure lost actionable timing measurements: %+v", d)
			}
		})
	}
}

// The compiler is the only thing that chooses a transition or a cut length: the
// scenes the observer reported decide both (CDS-36, CDS-37).
func TestCompilerJoinsScenesAndHoldsCutLengths(t *testing.T) {
	scenes := []string{"food", "menu", "menu", "scenery", "food", "interior"}
	in := planningInput()
	sources(&in, len(scenes))
	cuts := []any{}
	for i, scene := range scenes {
		in.Analyses[i].Segments[0].Scene = scene
		// A 9 s cut on a food close-up is more than twice CDS-37's ceiling for
		// one, and the others are over the shared ceiling.
		cuts = append(cuts, timelineCut(in, fmt.Sprint("cut-", i), in.Analyses[i].Source.ID, 0, 9000, 1000))
	}
	in.TargetDurationMS = 30000
	s, _ := newService(t, raw(map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": 30000, "cuts": cuts}), true)
	got, _, err := s.Flow(t.Context(), testRef(), in)
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

// CDS-37 r3: a cut with no room to reach its target is left as it is, and the
// flow compiles — the other cuts take the length the approved timeline needs,
// past their own target where they must.
func TestACutThatCannotReachTheTargetIsKeptAndThePlanCompiles(t *testing.T) {
	in := planningInput()
	sources(&in, 3)
	// 900 ms of footage in all: under every target, and nowhere to grow.
	in.Analyses[0].Source.Info.DurationMS, in.Analyses[0].Segments[0].EndMS = 900, 900
	wire := map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": 15000,
		"cuts": []any{timelineCut(in, "short", "source-0", 0, 900, 1000), timelineCut(in, "b", "source-1", 0, 6000, 1000), timelineCut(in, "c", "source-2", 0, 6000, 1000)}}
	s, _ := newService(t, raw(wire), true)
	plan, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil {
		t.Fatalf("a cut under the target refused the flow: %v", err)
	}
	if plan.DurationMS != 15000 || plan.Cuts[0].EndMS != 900 || plan.Cuts[1].EndMS <= 6000 || plan.Cuts[2].EndMS <= 6000 {
		t.Fatalf("plan = %+v", plan.Cuts)
	}
}

// Four 4.2 s cuts hold 16.8 s against a 15 s target. The shared range is the
// only one — no template or category imposes its own (CDS-37) — so the tail
// gives up the 1.8 s and the flow compiles at the approved length.
func TestTheTargetTrimsTheTailToTheApprovedDuration(t *testing.T) {
	in := planningInput()
	sources(&in, 4)
	cuts := []any{}
	for i := 0; i < 4; i++ {
		cuts = append(cuts, timelineCut(in, fmt.Sprint("cut-", i), in.Analyses[i].Source.ID, 0, 4200, 1000))
	}
	s, _ := newService(t, raw(map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": 16800, "cuts": cuts}), true)
	plan, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil {
		t.Fatalf("a reachable timeline was refused: %v", err)
	}
	total := 0
	for i, c := range plan.Cuts {
		total += c.SourceSpanMS()
		want := 4200
		if i == 3 {
			want = 2400
		}
		if c.SourceSpanMS() != want {
			t.Fatalf("tail trim changed cut %d: %+v", i, c)
		}
	}
	if !hasNotice(plan, "plan_target_duration") {
		t.Fatal("tail trim lost its notice")
	}
	if plan.DurationMS != 15000 || total-plan.TransitionTotal() != 15000 {
		t.Fatalf("duration %d total %d", plan.DurationMS, total)
	}
}

// A target the footage cannot hold is not the model's error. When every cut
// sits at its scene's ceiling or the end of what was observed, the compiler
// delivers the length the footage holds — as long as the length floor accepts
// it; TestComposeCannotFillTargetFromUnobservedOrReusedFootage keeps the
// refusal for a clip that would fall under the floor.
func TestFootageBoundClipShipsAtTheLengthItHolds(t *testing.T) {
	in := planningInput()
	sources(&in, 3)
	scene, _ := clip.CutScene(clip.Cut{SourceID: "source-0"}, in.Analyses[0])
	_, ceiling := design.CutBounds(scene)
	// Every source is used to its last observed frame: the footage holds
	// 2×ceiling + 4.5 s, the target asks for 3 s more, and no pass has room.
	in.Analyses[0].Source.Info.DurationMS, in.Analyses[0].Segments[0].EndMS = ceiling, ceiling
	in.Analyses[1].Source.Info.DurationMS, in.Analyses[1].Segments[0].EndMS = ceiling, ceiling
	in.Analyses[2].Source.Info.DurationMS, in.Analyses[2].Segments[0].EndMS = 4500, 4500
	held := 2*ceiling + 4500
	in.TargetDurationMS = held + 3000
	wire := map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": in.TargetDurationMS,
		"cuts": []any{timelineCut(in, "a", "source-0", 0, ceiling, 1000), timelineCut(in, "b", "source-1", 0, ceiling, 1000), timelineCut(in, "c", "source-2", 0, 4500, 1000)}}
	s, _ := newService(t, raw(wire), true)
	plan, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil {
		t.Fatalf("a footage-bound clip above the floor was refused: %v", err)
	}
	if plan.DurationMS != held-plan.TransitionTotal() || plan.DurationMS >= in.TargetDurationMS {
		t.Fatalf("duration %d, want the %d the footage holds less %d of transitions", plan.DurationMS, held, plan.TransitionTotal())
	}
}

// A cut belongs to ONE observed scene. Two scenes that merely touch in time are
// still two scenes, so a selection spanning both is narrowed rather than grown
// across the boundary (CLIP-7, CLIP-98).
func TestACutMayNotCrossTouchingScenes(t *testing.T) {
	in := planningInput()
	sources(&in, 3)
	scene, _ := clip.CutScene(clip.Cut{SourceID: "source-0"}, in.Analyses[0])
	_, ceiling := design.CutBounds(scene)
	first, second := in.Analyses[0].Segments[0], in.Analyses[0].Segments[0]
	first.EndMS, second.StartMS = 2800, 2800
	in.Analyses[0].Segments = []clip.Segment{first, second}
	in.TargetDurationMS = 2*ceiling + ceiling
	wire := map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": in.TargetDurationMS,
		"cuts": []any{timelineCut(in, "straddle", "source-0", 1000, 4000, 1000), timelineCut(in, "b", "source-1", 0, ceiling, 1000), timelineCut(in, "c", "source-2", 0, ceiling, 1000)}}
	s, models := newService(t, raw(wire), true)
	delivered, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil || !hasNotice(delivered, "plan_cut_scene") || len(models.calls) != 1 {
		t.Fatalf("scene narrowing failed: %v", err)
	}
	if delivered.Cuts[0].EndMS > 2800 {
		t.Fatal("repair crossed into the next observed scene")
	}
	// The same cut inside ONE of those scenes is fine.
	wire["cuts"].([]any)[0] = timelineCut(in, "straddle", "source-0", 0, 2800, 1000)
	s, _ = newService(t, raw(wire), true)
	if _, _, err = s.Flow(t.Context(), testRef(), in); err != nil {
		t.Fatalf("a contained cut was refused: %v", err)
	}
}

// Every length the timeline reasons about is OUTPUT time after the rate. Six
// cuts at the six supported rates compile to the sum of their transformed
// durations, and their ORIGINAL source ranges are untouched (CDS-62, CLIP-98).
func TestTheTimelineMeasuresEveryRateOnTransformedOutputTime(t *testing.T) {
	in := planningInput()
	rates := clip.PlaybackRates()
	sources(&in, len(rates))
	cuts := []any{}
	total := 0
	for i, rate := range rates {
		a := &in.Analyses[i]
		// 60 fps footage, so even 0.5x reaches the 30 fps output.
		a.Source.Info.FrameRateNumerator, a.Source.Info.FrameRateDenominator = 60, 1
		a.Source.Info.DecodedFrames, a.Source.Info.DecodedDurationMS = 3900, 65000
		a.Source.Info.CadenceVerified = true
		// A source span chosen so the OUTPUT length is 3000 ms at every rate.
		span := 3000 * rate / clip.RateUnitPermille
		cuts = append(cuts, timelineCut(in, fmt.Sprint("rate-", rate), a.Source.ID, 0, span, rate))
		total += 3000
	}
	in.TargetDurationMS = total
	wire := map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": total, "cuts": cuts}
	s, models := newService(t, raw(wire), true)
	plan, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil || len(models.calls) != 1 {
		t.Fatalf("rate-aware timeline refused: %v", err)
	}
	sum := 0
	for i, c := range plan.Cuts {
		if c.Rate() != rates[i] {
			t.Fatalf("cut %d changed rate to %d", i, c.Rate())
		}
		if c.StartMS != 0 || c.EndMS != 3000*rates[i]/clip.RateUnitPermille {
			t.Fatalf("cut %d source range moved to %d..%d", i, c.StartMS, c.EndMS)
		}
		if c.OutputDurationMS() != 3000 {
			t.Fatalf("cut %d occupies %d ms of output, want 3000", i, c.OutputDurationMS())
		}
		sum += c.OutputDurationMS()
	}
	if plan.DurationMS != sum-plan.TransitionTotal() {
		t.Fatalf("timeline %d, want %d output ms less %d of transitions", plan.DurationMS, sum, plan.TransitionTotal())
	}
}

// Reconciliation is stated in OUTPUT milliseconds and converted back to the
// source footage that produces them, so a sped-up cut gives up twice the source
// footage for the same output second.
func TestReconciliationConvertsOutputDeltaBackToSourceDelta(t *testing.T) {
	for _, rate := range []int{1000, 2000} {
		in := planningInput()
		in.Analyses[0].Source.Info.DurationMS = 65000
		in.Analyses[0].Segments[0].EndMS = 65000
		in.TargetDurationMS = 15000
		span := 6000 * rate / clip.RateUnitPermille
		cuts := []any{}
		for i := 0; i < 3; i++ {
			cuts = append(cuts, timelineCut(in, fmt.Sprint("cut-", i), "source", i*span, i*span+span, rate))
		}
		wire := map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": 15000, "cuts": cuts}
		s, _ := newService(t, raw(wire), true)
		plan, _, err := s.Flow(t.Context(), testRef(), in)
		if err != nil || plan.DurationMS != 15000 {
			t.Fatalf("rate %d: %v %d", rate, err, plan.DurationMS)
		}
		// The last cut gives up 3 s of output: 3 s of source at 1x, 6 s at 2x.
		for i, c := range plan.Cuts {
			want := 6000
			if i == 2 {
				want = 3000
			}
			if c.OutputDurationMS() != want {
				t.Fatalf("rate %d: cut occupies %d output ms, want %d", rate, c.OutputDurationMS(), want)
			}
			if c.SourceSpanMS() != want*rate/clip.RateUnitPermille {
				t.Fatalf("rate %d: cut gave up %d source ms", rate, c.SourceSpanMS())
			}
		}
	}
}

// A readable, schema-valid flow whose assembled output cannot reach the length
// floor fails as insufficient selected footage (CLIP-120) rather than as an
// unreadable response, and asks for no correction attempt: the response was
// read and validated, so resending the prompt cannot lengthen the footage. The
// same flow with one more second of observed footage ships.
func TestPlanUnderTheLengthFloorFailsAsInsufficientFootage(t *testing.T) {
	for _, tc := range []struct {
		name     string
		observed int
		ok       bool
	}{
		{"just under the floor", clip.MinDurationMS - 1000, false},
		{"exactly at the floor", clip.MinDurationMS, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := planningInput()
			in.Analyses[0].Source.Info.DurationMS = tc.observed
			in.Analyses[0].Segments[0].EndMS = tc.observed
			span := tc.observed / 3
			cuts := []any{}
			for i := range 3 {
				end := (i + 1) * span
				if i == 2 {
					end = tc.observed
				}
				cuts = append(cuts, timelineCut(in, fmt.Sprint("cut-", i), "source", i*span, end, 1000))
			}
			s, models := newService(t, raw(map[string]any{"storyline": []any{}, "ratio": "vertical", "duration_ms": 15000, "cuts": cuts}), true)
			plan, _, err := s.Flow(t.Context(), testRef(), in)
			if len(models.calls) != 1 {
				t.Fatalf("a validated flow was resent: %d calls", len(models.calls))
			}
			if tc.ok {
				if err != nil || plan.DurationMS != tc.observed {
					t.Fatalf("a flow at the floor was refused: %v %d", err, plan.DurationMS)
				}
				return
			}
			var code interface{ OutputValidationCode() string }
			if !errors.Is(err, clip.ErrInsufficientFootage) || errors.Is(err, llm.ErrBadOutput) || !errors.As(err, &code) || code.OutputValidationCode() != "plan_length_floor" {
				t.Fatalf("wrong cause below the floor: %v", err)
			}
			d, ok := clip.DiagnosticFromError(err)
			if !ok || d.Check != "plan_length_floor" || d.Values["min_ms"] != clip.MinDurationMS || d.Values["after_ms"] != tc.observed {
				t.Fatalf("lost the shortfall measurements: %+v", d)
			}
			// CLIP-85: the validated ranges stay inspectable after the refusal.
			if len(d.Ranges) != 3 || !d.Ranges[0].Valid {
				t.Fatalf("lost the validated ranges: %+v", d.Ranges)
			}
		})
	}
}
