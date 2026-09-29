package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// AssignSupportTier gives an operator-assigned tier its own coverage identity. It
// joins the guarded account update, tier history, credits and export allowance in the
// billing writer transaction, without creating a provider order or renewal schedule.
func (s *Service) AssignSupportTier(ctx context.Context, userID string, target plan.Plan) error {
	if !target.Valid() {
		return fmt.Errorf("unknown plan %q", target)
	}
	now := s.now()
	return s.store.InWriteTx(ctx, func(tx Store, credits Credits, plans Plans) error {
		current, err := plans.TierOf(ctx, userID)
		if err != nil {
			return err
		}
		if target == plan.Free || target == plan.Master {
			if current == target {
				return nil
			}
			if err := plans.AssignTier(ctx, userID, target); err != nil {
				return err
			}
			return tx.DeleteSupportCoverage(ctx, userID)
		}

		support, assigned, err := tx.SupportCoverage(ctx, userID)
		if err != nil {
			return err
		}
		sub, subscribed, err := tx.Subscription(ctx, userID)
		if err != nil {
			return err
		}
		if current == target && (assigned || subscribed && sub.Status == "active" && now.Before(sub.TermEnd)) {
			return nil
		}
		var coverage Coverage
		var oldTier plan.Plan
		switch {
		case assigned:
			coverage = Coverage{ID: support.ID, Anchor: support.Anchor, Tier: support.Tier}
			oldTier = support.Tier
		case subscribed && sub.Status == "active" && now.Before(sub.TermEnd):
			coverage = Coverage{ID: sub.CoverageID, Anchor: sub.AnchorAt, Tier: sub.Tier}
			oldTier = sub.Tier
		default:
			coverage = Coverage{ID: "support:" + userID + ":" + now.UTC().Format(time.RFC3339Nano), Anchor: now, Tier: target, DailyTier: target}
		}
		correlation := "support:" + userID + ":" + now.UTC().Format(time.RFC3339Nano)
		if oldTier.Valid() && oldTier != plan.Free && oldTier != plan.Master {
			dailyStart, _ := plan.DailyWindow(coverage.Anchor, now)
			coverage.DailyTier, err = tx.TierAt(ctx, userID, coverage.ID, dailyStart)
			if err != nil {
				return err
			}
			if target.Rank() > oldTier.Rank() {
				start, end := plan.BenefitWindow(coverage.Anchor, now)
				oldOffer, _ := plan.CommercialOffer(oldTier)
				newOffer, _ := plan.CommercialOffer(target)
				bonus, err := plan.ProrateCeil(int64(newOffer.MonthlyBonus-oldOffer.MonthlyBonus), start, end, now)
				if err != nil {
					return err
				}
				exports, err := plan.ProrateFloor(int64(newOffer.ServerExports-oldOffer.ServerExports), start, end, now)
				if err != nil {
					return err
				}
				if err := credits.AddUpgradeBonus(ctx, userID, coverage, now, int(bonus), int(exports), correlation); err != nil {
					return err
				}
			}
		} else {
			if err := credits.OpenCoverage(ctx, userID, coverage, now, correlation); err != nil {
				return err
			}
		}
		if err := plans.AssignTier(ctx, userID, target); err != nil {
			return err
		}
		if err := tx.UpsertSupportCoverage(ctx, SupportCoverage{UserID: userID, ID: coverage.ID, Tier: target,
			Anchor: coverage.Anchor, UpdatedAt: now}); err != nil {
			return err
		}
		return tx.InsertTierTransition(ctx, userID, coverage.ID, now, target, correlation)
	})
}
