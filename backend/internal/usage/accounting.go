package usage

import "context"

// ClipAccounting is the ledger's owner-scoped projection, not a client estimate.
// Nil amounts are unknown/pending, never a settled zero. Shadow is master-only.
type ClipAccounting struct {
	CancellationPolicyVersion                             int
	SettlementReason                                      string
	NominalReservation, ConfirmedCharge, CancellationFee  *int
	ShadowConfirmedCharge, ShadowCancellationFee          *int
	Approved, Reserved, FinalCharge, Refund, ShadowCharge *int
	Exempt                                                bool
	Settled                                               bool
}

type clipAccountingReader interface {
	ClipAccountingForJob(context.Context, string, string) (*ClipAccounting, error)
}

func (s *Service) ClipAccounting(ctx context.Context, user, job string) (*ClipAccounting, error) {
	r, ok := s.store.(clipAccountingReader)
	if !ok {
		return nil, ErrClipApproval
	}
	return r.ClipAccountingForJob(ctx, user, job)
}
