package rpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
)

// stubLedger answers with a fixed balance: this file is about what the handler PUBLISHES
// from the ladder, not about how a balance is computed.
type stubLedger struct{}

type stubExports struct{ window planrpc.ExportBalance }

func (s stubExports) Current(context.Context, string, time.Time) (planrpc.ExportBalance, bool, error) {
	return s.window, true, nil
}

func TestGetMyPlanKeepsExportCountsOutsideCreditBalance(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "alice", Plan: plan.Basic})
	h := planrpc.NewHandler(stubLedger{}, stubEstimator{}).WithExports(stubExports{window: planrpc.ExportBalance{
		CoverageID: "paid:alice", StartsAt: start, EndsAt: start.AddDate(0, 1, 0), Allowance: 6, Used: 2, Reserved: 1,
	}})
	response, err := h.GetMyPlan(ctx, connect.NewRequest(&postpilotv1.GetMyPlanRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	w := response.Msg.ServerExportWindow
	if w == nil || w.Allowance != 6 || w.Used != 2 || w.Reserved != 1 || w.Remaining != 3 || w.EndsAt != "2026-10-01T00:00:00Z" || response.Msg.Balance.Credits != 220 {
		t.Fatalf("export=%+v balance=%+v", w, response.Msg.Balance)
	}
}

func (stubLedger) BalanceFor(context.Context, string, plan.Plan) (planrpc.Balance, error) {
	return planrpc.Balance{Credits: 220}, nil
}

// stubEstimator publishes one priced combo, the way an operator who has assigned `top`
// and nothing else leaves the catalog.
type stubEstimator struct {
	err       error
	clipRates *plan.ClipRates
}

type snapshotEstimator struct {
	rate plan.RateSnapshot
	err  error
	seen *plan.RateSnapshot
}

func (s snapshotEstimator) CurrentRate(context.Context) (plan.RateSnapshot, error) {
	return s.rate, s.err
}
func (s snapshotEstimator) ComboRates(context.Context) ([]planrpc.EstimatorCombo, error) {
	panic("unrounded fallback must not be used")
}
func (s snapshotEstimator) ComboRatesAt(_ context.Context, rate plan.RateSnapshot) ([]planrpc.EstimatorCombo, error) {
	*s.seen = rate
	return []planrpc.EstimatorCombo{{Combo: "value", PostCredits: plan.PostFigure{Credits: 12, Basis: plan.PostCreditsEstimate}}}, nil
}

// Estimates are priced at the selected rate for everyone, but only the operator is shown that
// rate (QUOTA-65).
func TestGetMyPlanPricesAtTheRateItShowsOnlyToMaster(t *testing.T) {
	rate := plan.RateSnapshot{Source: "test", PublicationDate: "2026-09-30", ReferenceE4: 14_001_000, AppliedE4: 14_100_000}
	var seen plan.RateSnapshot
	msg := getMyPlanWith(t, plan.Light, snapshotEstimator{rate: rate, seen: &seen})
	if seen != rate || msg.FxRate != nil || len(msg.EstimatorCombos) != 1 {
		t.Fatalf("non-master: rate used=%+v published=%+v combos=%+v", seen, msg.FxRate, msg.EstimatorCombos)
	}
	msg = getMyPlanWith(t, plan.Master, snapshotEstimator{rate: rate, seen: &seen})
	if msg.FxRate == nil || msg.FxRate.AppliedE4 != rate.AppliedE4 {
		t.Fatalf("master rate = %+v", msg.FxRate)
	}
	msg = getMyPlanWith(t, plan.Light, snapshotEstimator{err: errors.New("no FX"), seen: &seen})
	if !msg.FxUnavailable || len(msg.EstimatorCombos) != 0 {
		t.Fatalf("missing FX published estimates: %+v", msg)
	}
}

func (s stubEstimator) ComboRates(context.Context) ([]planrpc.EstimatorCombo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []planrpc.EstimatorCombo{{
		Combo: "top", ObserveLabel: "vendor/eyes", WriteLabel: "vendor/pen",
		PostCredits: plan.PostFigure{Credits: 38, Basis: plan.PostCreditsRecentUsage},
		ClipRates:   s.clipRates,
	}}, nil
}

func getMyPlanWith(t *testing.T, acting plan.Plan, estimator planrpc.Estimator) *postpilotv1.GetMyPlanResponse {
	t.Helper()
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "alice", Plan: acting})
	res, err := planrpc.NewHandler(stubLedger{}, estimator).GetMyPlan(
		ctx, connect.NewRequest(&postpilotv1.GetMyPlanRequest{}),
	)
	if err != nil {
		t.Fatalf("GetMyPlan: %v", err)
	}
	return res.Msg
}

func getMyPlan(t *testing.T, acting plan.Plan) *postpilotv1.GetMyPlanResponse {
	t.Helper()
	return getMyPlanWith(t, acting, stubEstimator{})
}

// Every figure a comparison screen shows crosses here, so the rungs and the estimator rates
// are pinned against the ladder itself rather than against copies of the numbers.
func TestGetMyPlanPublishesTheRungsAndTheEstimatorRates(t *testing.T) {
	msg := getMyPlan(t, plan.Basic)

	if len(msg.Offers) != len(plan.Offers()) {
		t.Fatalf("offers = %d, want %d", len(msg.Offers), len(plan.Offers()))
	}

	marked := make([]postpilotv1.Plan, 0, 1)
	for i, offer := range msg.Offers {
		if _, ok := planrpc.FromProto(offer.Plan); !ok {
			t.Fatalf("offer %v is not a known rung", offer.Plan)
		}
		rule := plan.Offers()[i]
		if offer.MonthlyKrw != int32(rule.MonthlyKRW) || offer.AnnualKrw != int32(rule.AnnualKRW) || offer.DailyCredits != int32(rule.DailyCredits) || offer.MonthlyBonus != int32(rule.MonthlyBonus) || offer.ModelCeiling != rule.ModelCeiling || offer.MonthlyServerExports != int32(rule.ServerExports) {
			t.Errorf("published %s offer = %+v, want %+v", rule.Plan, offer, rule)
		}
		if offer.Recommended {
			marked = append(marked, offer.Plan)
		}
	}
	if len(marked) != 1 || marked[0] != postpilotv1.Plan_PLAN_PRO {
		t.Errorf("recommended offers = %v, want exactly [PLAN_PRO]", marked)
	}
	if packs := msg.CreditPacks; len(packs) != 3 || packs[0].PriceKrw != 3000 || packs[0].Credits != 1000 || packs[2].PriceKrw != 30000 || packs[2].Credits != 10000 {
		t.Errorf("published packs = %+v", packs)
	}

	if len(msg.EstimatorCombos) != 1 {
		t.Fatalf("combos = %d, want the one assigned", len(msg.EstimatorCombos))
	}
	combo := msg.EstimatorCombos[0]
	if combo.Combo != "top" || combo.PostCredits != 38 || combo.PostCreditsBasis != postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_RECENT_USAGE {
		t.Errorf("combo = %+v", combo)
	}
}

// A combo read that fails leaves the comparison without an estimate, not without a plan: the
// grants and the balance are what this call is for, and the operator can fix an assignment.
func TestGetMyPlanSurvivesAnEstimatorFailure(t *testing.T) {
	msg := getMyPlanWith(t, plan.Basic, stubEstimator{err: errors.New("catalog down")})

	if len(msg.Offers) == 0 {
		t.Error("an estimator failure took the rungs with it")
	}
	if len(msg.EstimatorCombos) != 0 {
		t.Errorf("combos = %+v, want none", msg.EstimatorCombos)
	}
}

func TestGetMyPlanPublishesOptionalClipRatesAndSourceAssumption(t *testing.T) {
	msg := getMyPlanWith(t, plan.Basic, stubEstimator{clipRates: &plan.ClipRates{PerSource: 7654, PerOutputSecond: 123, PerClipBase: 4321}})
	got := msg.EstimatorCombos[0].ClipRates
	if got == nil || got.PerSourceMilli != 7654 || got.PerOutputSecondMilli != 123 || got.PerClipBaseMilli != 4321 {
		t.Fatalf("clip rates lost at transport: %+v", got)
	}
	without := getMyPlan(t, plan.Basic)
	if without.EstimatorCombos[0].ClipRates != nil {
		t.Fatal("missing clip rate became a zero-cost quote")
	}
	failed := getMyPlanWith(t, plan.Basic, stubEstimator{err: errors.New("no catalog")})
	for _, response := range []*postpilotv1.GetMyPlanResponse{msg, without, failed} {
		if response.ClipSourceSeconds != plan.EstimatorClipSourceSeconds {
			t.Fatalf("source assumption lost: %d", response.ClipSourceSeconds)
		}
	}
}

// QUOTA-66: estimator rates beside the models they price would give those prices back, so only
// the operator's copy names the combo's models.
func TestEstimatorComboNamesItsModelsToMasterOnly(t *testing.T) {
	for acting, want := range map[plan.Plan][2]string{
		plan.Pro:    {"", ""},
		plan.Master: {"vendor/eyes", "vendor/pen"},
	} {
		combos := getMyPlan(t, acting).EstimatorCombos
		if len(combos) != 1 || combos[0].GetObserveLabel() != want[0] || combos[0].GetWriteLabel() != want[1] {
			t.Fatalf("%s combos = %+v, want labels %v", acting, combos, want)
		}
		if combos[0].GetPostCredits() != 38 {
			t.Fatalf("%s lost its per-post figure: %+v", acting, combos[0])
		}
	}
}

// QUOTA-64: each level carries its pair's per-post figure and where it came from; a level with
// no figure says so with UNSPECIFIED rather than a zero a client could read as free.
func TestEstimatorComboCarriesItsPerPostFigure(t *testing.T) {
	msg := getMyPlanWith(t, plan.Pro, figureEstimator{})
	if len(msg.EstimatorCombos) != 2 {
		t.Fatalf("combos = %+v", msg.EstimatorCombos)
	}
	recent, missing := msg.EstimatorCombos[0], msg.EstimatorCombos[1]
	if recent.GetPostCredits() != 48 || recent.GetPostCreditsBasis() != postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_RECENT_USAGE {
		t.Fatalf("recent combo = %+v", recent)
	}
	if missing.GetPostCredits() != 0 || missing.GetPostCreditsBasis() != postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_UNSPECIFIED {
		t.Fatalf("figureless combo = %+v", missing)
	}
}

type figureEstimator struct{}

func (figureEstimator) ComboRates(context.Context) ([]planrpc.EstimatorCombo, error) {
	return []planrpc.EstimatorCombo{
		{Combo: "value", PostCredits: plan.PostFigure{Credits: 48, Basis: plan.PostCreditsRecentUsage}},
		{Combo: "top"},
	}, nil
}
