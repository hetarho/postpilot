package main

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/plan"
)

// voucherPaidCoverage asks billing for the effective entitlement through the
// voucher writer transaction, so a concurrent lapse cannot race redemption.
type voucherPaidCoverage struct{ billing *billing.Service }

func (v voucherPaidCoverage) ActivePaidAt(ctx context.Context, userID string, at time.Time) (bool, error) {
	coverage, found, err := v.billing.CoverageAt(ctx, userID, at)
	return found && coverage.Tier != plan.Free && coverage.Tier != plan.Master, err
}
