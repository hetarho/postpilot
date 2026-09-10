package store

import (
	"context"
	"database/sql"
	"errors"

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

func (s *Store) ClipAccountingForJob(ctx context.Context, user, job string) (*usage.ClipAccounting, error) {
	r, err := s.read.ClipAccountingForJob(ctx, sqlc.ClipAccountingForJobParams{UserID: user, JobID: job})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	reserved := int(r.DebitedCredits)
	out := &usage.ClipAccounting{Approved: optionalCredits(r.ApprovedMaxCredits), Reserved: &reserved, Settled: r.SettledAt.Valid, Exempt: r.HoldCredits > 0 && reserved == 0}
	if !out.Settled {
		return out, nil
	}
	if !r.SettledCredits.Valid {
		return nil, errors.New("settled clip has no final charge")
	}
	charge := int(r.SettledCredits.Int64)
	if out.Exempt {
		out.ShadowCharge = &charge
		zero := 0
		out.FinalCharge, out.Refund = &zero, &zero
	} else {
		refund := reserved - charge
		if refund < 0 {
			return nil, errors.New("clip charge exceeds reservation")
		}
		out.FinalCharge, out.Refund = &charge, &refund
	}
	return out, nil
}
