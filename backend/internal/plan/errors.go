package plan

import (
	"fmt"
	"strconv"
	"time"
)

// ReasonInsufficientCredits is the wire reason for work the account's balance cannot
// cover. It follows the product's UPPER_SNAKE spelling, like every other reason in the
// failure contract, and the frontend renders copy from it rather than from the message.
const ReasonInsufficientCredits = "INSUFFICIENT_CREDITS"

// ReasonMasterOnly is the wire reason for a procedure only the operator tier may call.
const ReasonMasterOnly = "MASTER_ONLY"

// InsufficientCreditsError is a refused hold. It carries what the work would have cost,
// what the account has, and when the next grant opens, so no caller has to re-derive a
// balance it never read.
//
// Unlike the plan-floor refusal it replaces, this one is temporary: the same request
// succeeds after a renewal or a grant, which is why nothing downstream treats a refused
// model as invalidated.
type InsufficientCreditsError struct {
	Required int
	Balance  int
	RenewsAt time.Time
}

func (e *InsufficientCreditsError) Error() string {
	return fmt.Sprintf("insufficient credits: %d required, %d available, renews at %s",
		e.Required, e.Balance, e.RenewsAt.Format(time.RFC3339))
}

// Reason is the stable product identifier for this refusal.
func (e *InsufficientCreditsError) Reason() string { return ReasonInsufficientCredits }

// Params are the display-safe values the reason allowlists. They stay machine values —
// integer credits and an RFC3339 instant — because the browser owns every bit of
// formatting and a server that formatted either would be guessing at a locale it cannot
// see.
func (e *InsufficientCreditsError) Params() map[string]string {
	return map[string]string{
		"required":  strconv.Itoa(e.Required),
		"balance":   strconv.Itoa(e.Balance),
		"renews_at": e.RenewsAt.UTC().Format(time.RFC3339),
	}
}
