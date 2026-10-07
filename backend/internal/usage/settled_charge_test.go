package usage

import (
	"context"
	"errors"
	"testing"
)

type chargeReader struct {
	user, job string
	charge    int
	found     bool
	err       error
	reads     int
}

func (r *chargeReader) SettledChargeForJob(_ context.Context, user, job string) (int, bool, error) {
	r.reads++
	r.user = user
	r.job = job
	return r.charge, r.found, r.err
}
func TestSettledChargesRequiresReaderAndPreservesUnknownVersusConfirmedZero(t *testing.T) {
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("missing reader silently accepted")
			}
		}()
		NewSettledCharges(nil)
	}()
	reader := &chargeReader{}
	charges := NewSettledCharges(reader)
	if _, err := charges.SettledJobCharge(t.Context(), "", "job"); !errors.Is(err, ErrChargeNotSettled) || reader.reads != 0 {
		t.Fatal("unowned query", err)
	}
	if _, err := charges.SettledJobCharge(t.Context(), "alice", "job"); !errors.Is(err, ErrChargeNotSettled) {
		t.Fatal("unknown became zero", err)
	}
	reader.found = true
	if charge, err := charges.SettledJobCharge(t.Context(), "alice", "job"); err != nil || charge != 0 || reader.user != "alice" || reader.job != "job" {
		t.Fatal("actual zero lost", charge, err)
	}
	reader.charge = 7
	if charge, err := charges.SettledJobCharge(t.Context(), "alice", "job"); err != nil || charge != 7 {
		t.Fatal(charge, err)
	}
	reader.err = errors.New("receipt read failed")
	if _, err := charges.SettledJobCharge(t.Context(), "alice", "job"); !errors.Is(err, reader.err) {
		t.Fatal("read error converted", err)
	}
}
