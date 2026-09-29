package clip

import "time"

type RefundFunding struct {
	UserID, OrderID, Kind, CoverageID string
	Start, End                        time.Time
}

type RefundFundingEvidence struct {
	ExportsUsed, ExportsReserved, ExportsRemaining int
}
