package main

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type refundBenefits struct {
	credits *usagestore.Store
	exports *clipstore.Store
}

func creditFunding(p billing.RefundPayment) usage.RefundFunding {
	return usage.RefundFunding{UserID: p.UserID, OrderID: p.OrderID, Kind: p.Kind,
		CoverageID: p.CoverageID, LotID: p.PackLotID, Start: p.EffectiveAt, End: p.FundingEnd}
}

func exportFunding(p billing.RefundPayment) clip.RefundFunding {
	return clip.RefundFunding{UserID: p.UserID, OrderID: p.OrderID, Kind: p.Kind,
		CoverageID: p.CoverageID, Start: p.EffectiveAt, End: p.FundingEnd}
}

func (r refundBenefits) Inspect(ctx context.Context, p billing.RefundPayment, at time.Time) (billing.RefundEvidence, error) {
	credits, err := r.credits.RefundFundingEvidence(ctx, creditFunding(p), at)
	if err != nil {
		return billing.RefundEvidence{}, err
	}
	result := billing.RefundEvidence{PaidModelJobs: credits.PaidJobs, CreditsUsed: credits.CreditsUsed,
		CreditsReserved: credits.CreditsReserved, FundedCreditsRemaining: credits.CreditsRemaining}
	if p.Kind == "pack" {
		return result, nil
	}
	exports, err := r.exports.RefundFundingEvidence(ctx, exportFunding(p))
	if err != nil {
		return billing.RefundEvidence{}, err
	}
	result.ServerExportsUsed, result.ServerExportsReserved = exports.ExportsUsed, exports.ExportsReserved
	result.FundedExportsRemaining = exports.ExportsRemaining
	return result, nil
}

func (r refundBenefits) Guard(ctx context.Context, request billing.RefundRequest, p billing.RefundPayment, _ time.Time) error {
	if err := r.credits.GuardRefundFunding(ctx, creditFunding(p), request.ID); err != nil {
		return err
	}
	if p.Kind != "pack" {
		return r.exports.GuardRefundFunding(ctx, exportFunding(p), request.ID)
	}
	return nil
}

func (r refundBenefits) Release(ctx context.Context, request billing.RefundRequest, p billing.RefundPayment) error {
	if err := r.credits.ReleaseRefundFunding(ctx, request.ID); err != nil {
		return err
	}
	if p.Kind != "pack" {
		return r.exports.ReleaseRefundFunding(ctx, request.ID)
	}
	return nil
}

func (r refundBenefits) Confirm(ctx context.Context, request billing.RefundRequest, p billing.RefundPayment, at time.Time) error {
	if err := r.credits.ConfirmRefundFunding(ctx, request.ID, at); err != nil {
		return err
	}
	if p.Kind != "pack" {
		return r.exports.ConfirmRefundFunding(ctx, exportFunding(p), request.ID, at)
	}
	return nil
}
