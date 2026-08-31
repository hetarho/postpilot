package plan

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Axis names the numeric limit a refusal hit. The values are the wire reasons the
// frontend renders copy from, never the message string — so they follow the same
// UPPER_SNAKE spelling as every other reason in the product's failure contract.
type Axis string

const (
	AxisDailyCount  Axis = "DAILY_COUNT"
	AxisDailyCost   Axis = "DAILY_COST"
	AxisMonthlyCost Axis = "MONTHLY_COST"
)

// ReasonModelLocked is the wire reason for a ref below the acting plan's floor.
const ReasonModelLocked = "MODEL_LOCKED"

// ReasonMasterOnly is the wire reason for a procedure only the operator tier may call.
const ReasonMasterOnly = "MASTER_ONLY"

// QuotaError is a refused admission. It carries everything the client needs to explain
// itself — which axis filled, its limit, what has been used, and the instant it lifts —
// so no caller has to re-derive a window it never measured.
type QuotaError struct {
	Axis     Axis
	Limit    int64
	Used     int64
	ResetsAt time.Time
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("plan quota exhausted on %s: used %d of %d, resets at %s",
		e.Axis, e.Used, e.Limit, e.ResetsAt.Format(time.RFC3339))
}

// Reason is the stable product identifier for this refusal.
func (e *QuotaError) Reason() string { return string(e.Axis) }

// Params are the display-safe values the reason allowlists. Costs stay in micro-USD —
// the client owns currency formatting, and rounding here would make the number it shows
// disagree with the limit it was refused against.
func (e *QuotaError) Params() map[string]string {
	return map[string]string{
		"limit":     strconv.FormatInt(e.Limit, 10),
		"used":      strconv.FormatInt(e.Used, 10),
		"resets_at": e.ResetsAt.UTC().Format(time.RFC3339),
	}
}

// ModelLockedError is a request naming one or more refs the acting plan may not run. It
// keeps the whole offending list because a bulk operation — applying a recommendation
// set — must tell the user every selection that blocked it, not just the first.
type ModelLockedError struct {
	Models   []string
	Required Plan
}

// NewModelLocked reports one locked ref.
func NewModelLocked(model string, required Plan) *ModelLockedError {
	return &ModelLockedError{Models: []string{model}, Required: required}
}

func (e *ModelLockedError) Error() string {
	return fmt.Sprintf("model %s requires the %s plan", strings.Join(e.Models, ", "), e.Required)
}

func (e *ModelLockedError) Reason() string { return ReasonModelLocked }

// Params carry the first offending ref as `model` (what single-ref copy renders) and the
// full list as `models`. Required is the highest floor among them, so the copy asks for
// the one upgrade that unblocks every listed ref at once.
func (e *ModelLockedError) Params() map[string]string {
	first := ""
	if len(e.Models) > 0 {
		first = e.Models[0]
	}
	return map[string]string{
		"model":         first,
		"models":        strings.Join(e.Models, ", "),
		"required_plan": e.Required.String(),
	}
}
