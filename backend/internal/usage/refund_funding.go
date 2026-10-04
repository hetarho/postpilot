package usage

import "time"

// RefundFunding is the credit half of what one refundable payment funded, as billing computed
// it: the usage store selects, freezes and voids lots by these fields alone and never derives
// them, and the guard row it writes carries the same fields to the grant trigger.
type RefundFunding struct {
	UserID, OrderID, Kind string
	// LotID is a pack's purchased lot.
	LotID string
	// Correlation selects the lots issued for that order (an upgrade's bonus lot).
	Correlation string
	// CoverageID, Start and End select the coverage's daily and monthly grants whose window
	// starts in [Start, End): every one but upgrade bonuses, or only those issued with
	// WindowCause when it is set.
	CoverageID, WindowCause string
	Start, End              time.Time
}

type RefundFundingEvidence struct {
	CreditsUsed, CreditsReserved, CreditsRemaining, PaidJobs int
}
