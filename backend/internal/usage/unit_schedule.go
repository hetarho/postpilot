package usage

// UnitCallsForSchedule expands a job's merged (model, stage) call counts into
// the exact-input budgets its server quote froze. Identical refs can legitimately
// have different speech inputs; neither input disappears into the merged count.
func UnitCallsForSchedule(schedule []PlannedCall, budgets []UnitBudget) ([]PlannedCall, error) {
	type key struct{ model, stage string }
	actual, expected := map[key]int{}, map[key]int{}
	for _, c := range schedule {
		if c.Count < 1 || c.CompletionTokens != 0 {
			return nil, ErrUnitApproval
		}
		actual[key{c.Ref.String(), c.Stage}] += c.Count
	}
	out := make([]PlannedCall, 0, len(budgets))
	for _, b := range budgets {
		if b.Count < 1 {
			return nil, ErrUnitApproval
		}
		expected[key{b.Ref.String(), b.Operation}] += b.Count
		copy := b
		out = append(out, PlannedCall{Ref: b.Ref, Stage: b.Operation, Count: b.Count, Units: &copy})
	}
	if len(expected) == 0 || len(actual) != len(expected) {
		return nil, ErrUnitApproval
	}
	for k, n := range expected {
		if actual[k] != n {
			return nil, ErrUnitApproval
		}
	}
	return out, nil
}
