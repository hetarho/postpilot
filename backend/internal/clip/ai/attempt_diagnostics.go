package ai

import (
	"errors"

	"github.com/postpilot/backend/internal/clip"
)

func planFailure(err error, in clip.PlanningInput, plan clip.EditPlan, phase string, cut int) error {
	if _, exists := clip.DiagnosticFromError(err); exists {
		return err
	}
	check := "unknown"
	var code interface{ OutputValidationCode() string }
	if errors.As(err, &code) {
		check = clip.SafeAttemptCheck(code.OutputValidationCode())
	}
	values := map[string]int{"cut_count": len(plan.Cuts), "target_ms": in.TargetDurationMS}
	if cut > 0 {
		values["cut"] = cut
	}
	if phase == "validation" {
		values["after_ms"] = plan.DurationMS
		total := 0
		for _, c := range plan.Cuts {
			total += c.EndMS - c.StartMS
		}
		values["before_ms"] = total - plan.TransitionTotal()
		values["transition_ms"] = plan.TransitionTotal()
		if check == "plan_timeline" {
			phase = "timeline_total"
		}
	}
	return clip.WithAttemptDiagnostic(err, clip.AttemptDiagnostic{Check: check, Phase: phase, Values: clip.SafeAttemptValues(values), Ranges: clip.AttemptRangeDiagnostics(plan, in.Analyses)})
}
