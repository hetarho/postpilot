package clip

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

type Accounting struct {
	CancellationPolicyVersion                                int
	SettlementReason                                         string
	NominalReservation, ConfirmedCharge, CancellationFee     *int
	ShadowConfirmedCharge, ShadowCancellationFee             *int
	JobID, Status                                            string
	ApprovedMax, Reserved, FinalCharge, Refund, ShadowCharge *int
	Exempt, Settled                                          bool
	FaultCause                                               string
	CompensationCredits, NetCharge                           *int
	CompensationExpiresAt                                    *time.Time
	Rate                                                     plan.RateSnapshot
}
type AccountingReader interface {
	ForJob(context.Context, string, string) (*Accounting, error)
}
