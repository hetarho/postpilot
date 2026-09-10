package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

type accountingReader struct {
	value     *clip.Accounting
	user, job string
}

func (r *accountingReader) ForJob(_ context.Context, user, job string) (*clip.Accounting, error) {
	r.user, r.job = user, job
	if r.value == nil {
		return nil, nil
	}
	copy := *r.value
	return &copy, nil
}

func TestProjectAccountingDistinguishesEverySettlementPhase(t *testing.T) {
	h := generationSetup(t)
	reader := &accountingReader{}
	h.service.WithCredits(&quotePricing{}, reader)
	q := quote(t, h)
	id, err := accept(h, q)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a, err := h.service.Accounting(ctx, "alice", h.project.ID)
	if err != nil || a.JobID != id || a.Status != "not_reserved" || a.ApprovedMax == nil || *a.ApprovedMax != q.Pricing.MaxCredits || a.Reserved == nil || *a.Reserved != 0 || a.FinalCharge != nil {
		t.Fatal(a, err)
	}
	if _, err = h.service.Accounting(ctx, "bob", h.project.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if reader.user != "alice" || reader.job != id {
		t.Fatal("foreign read reached ledger")
	}
	reserved := 12
	reader.value = &clip.Accounting{ApprovedMax: &q.Pricing.MaxCredits, Reserved: &reserved}
	a, err = h.service.Accounting(ctx, "alice", h.project.ID)
	if err != nil || a.Status != "reserved" || a.FinalCharge != nil {
		t.Fatal(a, err)
	}
	if err = h.run(t); !errors.Is(err, clip.ErrQuoteRequired) {
		t.Fatal(err)
	}
	a, err = h.service.Accounting(ctx, "alice", h.project.ID)
	if err != nil || a.Status != "settling" || a.Settled || a.FinalCharge != nil {
		t.Fatal(a, err)
	}
	zero := 0
	reader.value.FinalCharge, reader.value.Refund, reader.value.Settled = &zero, &reserved, true
	a, err = h.service.Accounting(ctx, "alice", h.project.ID)
	if err != nil || a.Status != "settled" || !a.Settled || *a.FinalCharge != 0 || *a.Refund != 12 {
		t.Fatal(a, err)
	}
	reader.value.Exempt = true
	reader.value.Reserved = &zero
	reader.value.Refund = &zero
	reader.value.ShadowCharge = &reserved
	a, err = h.service.Accounting(ctx, "alice", h.project.ID)
	if err != nil || a.Status != "exempt" || *a.Reserved != 0 || *a.ShadowCharge != 12 {
		t.Fatal(a, err)
	}
	reader.value = nil
	a, err = h.service.Accounting(ctx, "alice", h.project.ID)
	if err != nil || a.Status != "not_reserved" || !a.Settled || a.FinalCharge == nil || *a.FinalCharge != 0 {
		t.Fatal(a, err)
	}
	if _, err = h.db.Writer.Exec("UPDATE generation_jobs SET payload='{}' WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	a, err = h.service.Accounting(ctx, "alice", h.project.ID)
	if err != nil || a.Status != "unavailable" || a.ApprovedMax != nil || a.FinalCharge != nil {
		t.Fatal("legacy unknown became zero", a, err)
	}
}
