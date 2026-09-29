package usage

import "time"

// RefundFunding identifies credit grants paid for by one checkout. Billing
// supplies the funding identity; the usage store owns lot selection and holds.
type RefundFunding struct {
	UserID, OrderID, Kind, CoverageID, LotID string
	Start, End                               time.Time
}

type RefundFundingEvidence struct {
	CreditsUsed, CreditsReserved, CreditsRemaining, PaidJobs int
}
