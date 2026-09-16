package ai_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestNativeTimelineUsesObservedRoomBeforeSelectedStarts(t *testing.T) {
	for _, duration := range []int{8000, 6000} {
		t.Run(fmt.Sprint(duration), func(t *testing.T) {
			in := nativeInput()
			base := in.Analyses[0]
			in.Analyses = nil
			wire := nativePlan()
			cuts := wire["cuts"].([]any)
			for i := 0; i < 2; i++ {
				a := base
				a.Source.ID = fmt.Sprintf("footage-%d", i)
				a.Source.Fingerprint = fmt.Sprintf("fingerprint-%d", i)
				a.Source.Info.DurationMS = duration
				a.Segments = []clip.Segment{base.Segments[i]}
				a.Segments[0].StartMS, a.Segments[0].EndMS = 0, duration
				in.Analyses = append(in.Analyses, a)
				c := cuts[i].(map[string]any)
				c["source_id"], c["start_ms"], c["end_ms"] = a.Source.ID, duration-2000, duration
				c["observation_refs"] = []string{clip.ObservationID(a.Source.ID, 0)}
			}
			wire["generated"] = []any{}
			s, models, _ := newService(t, raw(wire), true)
			plan, _, err := s.Plan(t.Context(), testRef(), in)
			if len(models.calls) != 1 {
				t.Fatal("timeline repair made another paid call")
			}
			if duration == 6000 {
				d, ok := clip.DiagnosticFromError(err)
				if !errors.Is(err, clip.ErrInsufficientFootage) || !ok || d.Check != "plan_length_floor" || d.Phase != "timeline_grow" || d.Values["before_ms"] != 4000 || d.Values["after_ms"] != 12000 || d.Values["remaining_ms"] != 3000 || len(d.Ranges) != 2 || !d.Ranges[0].Valid {
					t.Fatalf("lost failure measurements: %v %+v", err, d)
				}
				return
			}
			if err != nil || plan.DurationMS != 15000 || len(plan.Cuts) != 2 {
				t.Fatalf("reachable authored timeline failed: %v %+v", err, plan)
			}
			total := 0
			for i, c := range plan.Cuts {
				if c.SourceID != in.Analyses[i].Source.ID || c.StartMS < 0 || c.StartMS >= duration-2000 || c.EndMS != duration {
					t.Fatalf("unsafe range: %+v", c)
				}
				if _, ok := clip.CutEvidence(in.Analyses, c); !ok {
					t.Fatal("crossed an observation gap")
				}
				total += c.EndMS - c.StartMS
			}
			if total-plan.TransitionTotal() != plan.DurationMS {
				t.Fatal("duration arithmetic drifted")
			}
		})
	}
}

func TestNativeInvalidSelectionKeepsOnlySafeRangeDiagnostics(t *testing.T) {
	in := nativeInput()
	wire := nativePlan()
	wire["cuts"].([]any)[1].(map[string]any)["source_id"] = "https://private.example/secret?token=private"
	s, models, _ := newService(t, raw(wire), true)
	_, _, err := s.Plan(t.Context(), testRef(), in)
	d, ok := clip.DiagnosticFromError(err)
	if !ok || len(models.calls) != 1 || d.Check != "plan_length_floor" || len(d.Ranges) != 1 || !d.Ranges[0].Valid || d.Values["after_ms"] != 7500 {
		t.Fatalf("unsafe selection diagnostic: %v %+v", err, d)
	}
}
