package clip

import "time"

// RefundFunding is the export half of what one refundable payment funded, as billing computed
// it: the windows of CoverageID starting in [Start, End), plus the window carrying the export
// adjustment correlated to Correlation (an upgrade's order).
type RefundFunding struct {
	UserID, OrderID, Kind, Correlation, CoverageID string
	Start, End                                     time.Time
}

type RefundFundingEvidence struct {
	ExportsUsed, ExportsReserved, ExportsRemaining int
}
