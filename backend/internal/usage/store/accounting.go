package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/usage/store/sqlc"
)

func nullableCredits(n *int) sql.NullInt64 {
	if n == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*n), Valid: true}
}
func optionalCredits(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func (s *Store) AccountingForJob(ctx context.Context, user, job string, kinds []string) (*usage.ReservationAccounting, error) {
	if kinds == nil {
		kinds = []string{}
	}
	encoded, err := json.Marshal(kinds)
	if err != nil {
		return nil, fmt.Errorf("encode approved kinds: %w", err)
	}
	r, err := s.read.AccountingForJob(ctx, sqlc.AccountingForJobParams{UserID: user, JobID: job, Kinds: string(encoded)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	reserved := int(r.DebitedCredits)
	out := &usage.ReservationAccounting{Approved: optionalCredits(r.ApprovedMaxCredits), Reserved: &reserved, Settled: r.SettledAt.Valid, Exempt: r.HoldCredits > 0 && reserved == 0}
	nominal := int(r.HoldCredits)
	out.NominalReservation, out.CancellationPolicyVersion = &nominal, int(r.CancellationPolicyVersion)
	out.SettlementReason = r.SettlementReason.String
	out.FaultCause = r.SettlementCause.String
	out.CompensationCredits = optionalCredits(r.CompensationCredits)
	if r.CompensationExpiresAt.Valid {
		expires, err := time.Parse(writeLayout, r.CompensationExpiresAt.String)
		if err != nil {
			return nil, err
		}
		out.CompensationExpiresAt = &expires
	}
	out.Rate = plan.RateSnapshot{Source: r.FxSource.String, PublicationDate: r.FxPublicationDate.String,
		ReferenceE4: r.FxReferenceE4.Int64, AppliedE4: r.FxAppliedE4.Int64,
		Temporary: r.FxTemporary != 0}
	out.ConfirmedCharge, out.CancellationFee = optionalCredits(r.ConfirmedChargeCredits), optionalCredits(r.CancellationFeeCredits)
	if !out.Settled {
		return out, nil
	}
	if !r.SettledCredits.Valid {
		return nil, errors.New("settled admission has no final charge")
	}
	charge := int(r.SettledCredits.Int64)
	if out.Exempt {
		out.ShadowCharge = &charge
		zero := 0
		out.FinalCharge, out.Refund = &zero, &zero
		out.ShadowConfirmedCharge, out.ShadowCancellationFee = out.ConfirmedCharge, out.CancellationFee
		if out.ConfirmedCharge != nil {
			out.ConfirmedCharge = &zero
		}
		if out.CancellationFee != nil {
			out.CancellationFee = &zero
		}
	} else {
		refund := reserved - charge
		if refund < 0 {
			return nil, errors.New("charge exceeds reservation")
		}
		out.FinalCharge, out.Refund = &charge, &refund
	}
	net := *out.FinalCharge
	if out.CompensationCredits != nil {
		net -= *out.CompensationCredits
	}
	out.NetCharge = &net
	return out, nil
}
