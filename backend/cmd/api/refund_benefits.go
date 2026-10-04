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

// creditFunding and exportFunding hand billing's one funding value to the credit and export
// owners field for field; neither derives anything of its own.
func creditFunding(p billing.RefundPayment) usage.RefundFunding {
	f := p.Funding()
	return usage.RefundFunding{UserID: f.UserID, OrderID: f.OrderID, Kind: f.Kind, LotID: f.LotID,
		Correlation: f.Correlation, CoverageID: f.CoverageID, WindowCause: f.WindowCause, Start: f.Start, End: f.End}
}

func exportFunding(p billing.RefundPayment) clip.RefundFunding {
	f := p.Funding()
	return clip.RefundFunding{UserID: f.UserID, OrderID: f.OrderID, Kind: f.Kind,
		Correlation: f.Correlation, CoverageID: f.CoverageID, Start: f.Start, End: f.End}
}

func (r refundBenefits) Inspect(ctx context.Context, p billing.RefundPayment, at time.Time) (billing.RefundEvidence, error) {
	credits, err := r.credits.RefundFundingEvidence(ctx, creditFunding(p), at)
	if err != nil {
		return billing.RefundEvidence{}, err
	}
	exports, err := r.exports.RefundFundingEvidence(ctx, exportFunding(p))
	if err != nil {
		return billing.RefundEvidence{}, err
	}
	return billing.RefundEvidence{PaidModelJobs: credits.PaidJobs, CreditsUsed: credits.CreditsUsed,
		CreditsReserved: credits.CreditsReserved, FundedCreditsRemaining: credits.CreditsRemaining,
		ServerExportsUsed: exports.ExportsUsed, ServerExportsReserved: exports.ExportsReserved,
		FundedExportsRemaining: exports.ExportsRemaining}, nil
}

func (r refundBenefits) Guard(ctx context.Context, request billing.RefundRequest, p billing.RefundPayment, _ time.Time) error {
	if err := r.credits.GuardRefundFunding(ctx, creditFunding(p), request.ID); err != nil {
		return err
	}
	return r.exports.GuardRefundFunding(ctx, exportFunding(p), request.ID)
}

func (r refundBenefits) Release(ctx context.Context, request billing.RefundRequest, _ billing.RefundPayment) error {
	if err := r.credits.ReleaseRefundFunding(ctx, request.ID); err != nil {
		return err
	}
	return r.exports.ReleaseRefundFunding(ctx, request.ID)
}

func (r refundBenefits) Confirm(ctx context.Context, request billing.RefundRequest, p billing.RefundPayment, at time.Time) error {
	if err := r.credits.ConfirmRefundFunding(ctx, request.ID, at); err != nil {
		return err
	}
	return r.exports.ConfirmRefundFunding(ctx, exportFunding(p), request.ID, at)
}
