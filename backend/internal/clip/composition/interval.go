package composition

// ResolveInterval is shared by initial composition, retained-plan correction and
// rendering. Explicit intervals are checked as authored, never clamped to fit.
func ResolveInterval(element Element, outputMS, cutMS, cutOffsetMS, autoInsetMS int) (int, int, *Problem) {
	a, b := 0, outputMS
	invalid := func(reason string) (int, int, *Problem) { return 0, 0, fail(element.ID, element.Span.Line, reason) }
	if outputMS <= 0 || autoInsetMS < 0 {
		return invalid("invalid_limits")
	}
	switch element.Basis {
	case "whole":
	case "output-start", "output-end":
		if element.StartMS == nil || element.EndMS == nil {
			return invalid("invalid_interval")
		}
		a, b = *element.StartMS, *element.EndMS
		if element.Basis == "output-end" {
			if a < -outputMS || a > 0 || b < -outputMS || b > 0 {
				return invalid("interval_outside")
			}
			a, b = outputMS+a, outputMS+b
		}
	case "cut":
		if cutMS <= 0 {
			return invalid("binding_scope")
		}
		if element.StartMS == nil && element.EndMS == nil {
			a, b = autoInsetMS, cutMS-autoInsetMS
		} else {
			if element.StartMS == nil || element.EndMS == nil {
				return invalid("invalid_interval")
			}
			a, b = *element.StartMS, *element.EndMS
		}
		if cutOffsetMS < 0 || cutOffsetMS > outputMS || cutMS > outputMS-cutOffsetMS || a < 0 || b > cutMS || a >= b {
			return invalid("interval_outside")
		}
		a, b = a+cutOffsetMS, b+cutOffsetMS
	default:
		return invalid("invalid_interval")
	}
	if a < 0 || b > outputMS || a >= b {
		return invalid("interval_outside")
	}
	return a, b, nil
}
