package rpc_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/usage"
)

// stubLedger answers with a fixed balance: this file is about what the handler PUBLISHES
// from the ladder, not about how a balance is computed.
type stubLedger struct{}

func (stubLedger) BalanceFor(context.Context, string, plan.Plan) (usage.Balance, error) {
	return usage.Balance{Credits: 220}, nil
}

// stubEstimator publishes one priced combo, the way an operator who has assigned `quality`
// and nothing else leaves the catalog.
type stubEstimator struct{ err error }

func (s stubEstimator) ComboRates(context.Context) ([]planrpc.EstimatorCombo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []planrpc.EstimatorCombo{{
		Combo: "quality", ObserveLabel: "vendor/eyes", WriteLabel: "vendor/pen",
		PerPhotoMilli: 723, PerVideoMilli: 1100, Per1000CharsMilli: 3600, PerPostBaseMilli: 3800,
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
	for _, offer := range msg.Offers {
		if _, ok := planrpc.FromProto(offer.Plan); !ok {
			t.Fatalf("offer %v is not a known rung", offer.Plan)
		}
		if offer.Recommended {
			marked = append(marked, offer.Plan)
		}
	}
	if len(marked) != 1 || marked[0] != postpilotv1.Plan_PLAN_PRO {
		t.Errorf("recommended offers = %v, want exactly [PLAN_PRO]", marked)
	}

	if len(msg.EstimatorCombos) != 1 {
		t.Fatalf("combos = %d, want the one assigned", len(msg.EstimatorCombos))
	}
	combo := msg.EstimatorCombos[0]
	if combo.Combo != "quality" || combo.PerPhotoMilli != 723 || combo.PerPostBaseMilli != 3800 {
		t.Errorf("combo = %+v", combo)
	}
	if combo.PerThousandCharsMilli != 3600 || combo.PerVideoMilli != 1100 {
		t.Errorf("combo rates = %+v", combo)
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
