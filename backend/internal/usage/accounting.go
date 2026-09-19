package usage

import "context"

// ReservationAccounting is the ledger's owner-scoped projection, not a client estimate.
// Nil amounts are unknown/pending, never a settled zero. Shadow is master-only.
type ReservationAccounting struct {
	CancellationPolicyVersion                             int
	SettlementReason                                      string
	NominalReservation, ConfirmedCharge, CancellationFee  *int
	ShadowConfirmedCharge, ShadowCancellationFee          *int
	Approved, Reserved, FinalCharge, Refund, ShadowCharge *int
	Exempt                                                bool
	Settled                                               bool
}

type accountingReader interface {
	AccountingForJob(ctx context.Context, user, job string, kinds []string) (*ReservationAccounting, error)
}

// ReservationAccounting is the owner's view of what one piece of approved work reserved,
// spent and returned. The kinds it covers are the ones the root marked as needing an
// approval; the ledger passes them down rather than naming a product in SQL.
func (s *Service) ReservationAccounting(ctx context.Context, user, job string) (*ReservationAccounting, error) {
	r, ok := s.holds.(accountingReader)
	if !ok {
		return nil, ErrApprovalRequired
	}
	return r.AccountingForJob(ctx, user, job, s.approvedKindList())
}
