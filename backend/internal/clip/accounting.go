package clip

import (
	"context"
)

type Accounting struct {
	CancellationPolicyVersion                                int
	SettlementReason                                         string
	NominalReservation, ConfirmedCharge, CancellationFee     *int
	ShadowConfirmedCharge, ShadowCancellationFee             *int
	JobID, Status                                            string
	ApprovedMax, Reserved, FinalCharge, Refund, ShadowCharge *int
	Exempt, Settled                                          bool
}
type AccountingReader interface {
	ForJob(context.Context, string, string) (*Accounting, error)
}
