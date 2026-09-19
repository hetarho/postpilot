package ai_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
)

type correctionModels struct {
	*fakeModels
	validAfter int
	invalid    string
	failure    error
	finish     string
}

func (m *correctionModels) Complete(ctx context.Context, ref llm.ModelRef, req llm.Request) (llm.Response, error) {
	response, _ := m.fakeModels.Complete(ctx, ref, req)
	if len(m.calls) < m.validAfter {
		response.Text = m.invalid
		response.FinishReason = m.finish
	}
	return response, m.failure
}

// A coverage or status failure is the model's own output problem, so it uses the
// SAME reserved same-model correction allowance as a malformed response — no new
// budget, no extra provider, and it stops the moment a complete record arrives
// (CLIP-94, CLIP-95).
func TestIncompleteCoverageUsesTheReservedCorrectionAllowance(t *testing.T) {
	for _, broken := range []string{"missing end", "internal gap", "status"} {
		t.Run(broken, func(t *testing.T) {
			bad := observation()
			switch broken {
			case "missing end":
				firstSegment(bad)["end_ms"] = 4000
			case "internal gap":
				first, second := firstSegment(bad), firstSegment(observation())
				first["end_ms"], second["start_ms"], second["end_ms"] = 2000, 2001, 5000
				bad["segments"] = []any{first, second}
			case "status":
				firstSegment(bad)["certainty"] = "probably"
			}
			_, base, sizer := newService(t, raw(observation()), true)
			models := &correctionModels{fakeModels: base, validAfter: 3, invalid: raw(bad)}
			service, err := ai.New(models, sizer, ai.DefaultConfig(clip.Environment{}))
			if err != nil {
				t.Fatal(err)
			}
			var checks []string
			ctx := clip.WithResponseCorrectionObserver(t.Context(), func(_, _ int, d clip.AttemptDiagnostic) error {
				checks = append(checks, d.Check)
				return nil
			})
			in := chunk()
			in.Policy.ResponseRetries = 3
			got, _, err := service.ObserveChunk(ctx, testRef(), in)
			want := map[string]string{"missing end": "observe_coverage_end", "internal gap": "observe_coverage_gap", "status": "observe_status"}[broken]
			if err != nil || len(base.calls) != 3 || len(checks) != 2 || checks[0] != want || checks[1] != want {
				t.Fatalf("wrong correction path: calls=%d checks=%v err=%v", len(base.calls), checks, err)
			}
			if got.Segments[0].StartMS != 60000 || got.Segments[0].EndMS != 65000 {
				t.Fatalf("the accepted record lost its frozen offset: %+v", got.Segments[0])
			}
		})
	}
}

// An invalid rate, an overlapping selection and an unusable one are all things
// the MODEL got wrong, so each enters the same bounded same-model correction
// with its own typed cut diagnostic — never a silent drop and never a quiet
// fall back to 1x. Insufficient footage is NOT one of them: it is a fact about
// the material, and TestReconciliationCannotReachAnotherItemsFootage pins that
// it stops after one call (CLIP-94, CLIP-95, CLIP-99).
func TestReadableRuleBreaksNeverSpendCorrectionCalls(t *testing.T) {
	for _, tc := range []struct {
		name, check string
		break_      func(*clip.PlanningInput, map[string]any)
	}{
		{"rate", "plan_cut_rate", func(_ *clip.PlanningInput, p map[string]any) {
			firstCut(p)["rate_permille"] = 500
		}},
		{"overlap", "plan_source_overlap", func(_ *clip.PlanningInput, p map[string]any) {
			second := p["cuts"].([]any)[1].(map[string]any)
			second["start_ms"], second["end_ms"] = 0, 5000
		}},
		{"unusable", "plan_cut_usability", func(in *clip.PlanningInput, p map[string]any) {
			// The second half of the source is unknowable; only the bad
			// response reaches into it.
			in.Analyses[0].Segments[0].EndMS = 20000
			unknown := in.Analyses[0].Segments[0]
			unknown.StartMS, unknown.EndMS = 20000, 65000
			unknown.Certainty, unknown.Usability = clip.CertaintyUnknown, clip.UsabilityUsable
			in.Analyses[0].Segments = append(in.Analyses[0].Segments, unknown)
			for i, value := range p["cuts"].([]any) {
				c := value.(map[string]any)
				c["start_ms"], c["end_ms"] = 20000+i*5000, 25000+i*5000
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, good := planningInput(), plan()
			bad := plan()
			tc.break_(&in, bad)
			_, base, sizer := newService(t, raw(good), true)
			// The corrected attempt answers with the valid plan, so a working
			// correction shows up as exactly two calls.
			models := &correctionModels{fakeModels: base, validAfter: 2, invalid: raw(bad)}
			service, err := ai.New(models, sizer, ai.DefaultConfig(clip.Environment{}))
			if err != nil {
				t.Fatal(err)
			}
			var checks []string
			ctx := clip.WithResponseCorrectionObserver(t.Context(), func(_, _ int, d clip.AttemptDiagnostic) error {
				checks = append(checks, d.Check)
				return nil
			})
			in.Policy.ResponseRetries = 3
			delivered, _, err := service.Plan(ctx, testRef(), in)
			if len(base.calls) != 1 || len(checks) != 0 {
				t.Fatalf("readable plan consumed retries: calls=%d checks=%v", len(base.calls), checks)
			}
			if tc.name == "unusable" {
				d, ok := clip.DiagnosticFromError(err)
				if err == nil || !ok || d.Check != "plan_cut_count" {
					t.Fatal("empty remainder must fail", err)
				}
			} else if err != nil || !hasNotice(delivered, tc.check) {
				t.Fatalf("repair/removal missing: %v %+v", err, delivered.Notices)
			}

		})
	}
}

func TestResponseCorrectionStopsOnCancellationProviderAndLegacyLimits(t *testing.T) {
	for _, kind := range []string{"legacy", "cancel", "provider", "length", "authored", "unreported"} {
		t.Run(kind, func(t *testing.T) {
			_, base, sizer := newService(t, raw(observation()), true)
			models := &correctionModels{fakeModels: base, validAfter: 5, invalid: `{`}
			if kind == "unreported" {
				base.response.Usage.CostReported = false
			}
			if kind == "provider" {
				models.failure = llm.ErrRateLimited
			}
			if kind == "length" {
				models.finish = "length"
			}
			service, _ := ai.New(models, sizer, ai.DefaultConfig(clip.Environment{}))
			ctx := t.Context()
			if kind == "cancel" {
				ctx = clip.WithResponseCorrectionObserver(ctx, func(int, int, clip.AttemptDiagnostic) error { return context.Canceled })
			}
			in := chunk()
			if kind != "legacy" {
				in.Policy.ResponseRetries = 3
			}
			if kind == "authored" {
				in.Index = -1
			}
			_, _, err := service.ObserveChunk(ctx, testRef(), in)
			want := 1
			if kind == "authored" {
				want = 0
			}
			if err == nil || len(base.calls) != want {
				t.Fatalf("unexpected retry calls=%d err=%v", len(base.calls), err)
			}
			if kind == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation lost")
			}
		})
	}
}
