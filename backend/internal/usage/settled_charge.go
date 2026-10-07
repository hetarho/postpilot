package usage

import (
	"context"
	"errors"
)

var ErrChargeNotSettled = errors.New("job charge is not settled")

type SettledChargeReader interface {
	SettledChargeForJob(context.Context, string, string) (int, bool, error)
}
type SettledCharges struct{ reader SettledChargeReader }

func NewSettledCharges(reader SettledChargeReader) *SettledCharges {
	if reader == nil {
		panic("usage: settled charge reader is required")
	}
	return &SettledCharges{reader: reader}
}

// SettledJobCharge is a narrow owner-checked read of the actual terminal debit.
// A pending or missing receipt is never converted into an estimated or zero cost.
func (s *SettledCharges) SettledJobCharge(ctx context.Context, user, job string) (int, error) {
	if user == "" || job == "" {
		return 0, ErrChargeNotSettled
	}
	charge, found, err := s.reader.SettledChargeForJob(ctx, user, job)
	if err != nil {
		return 0, err
	}
	if !found || charge < 0 {
		return 0, ErrChargeNotSettled
	}
	return charge, nil
}
