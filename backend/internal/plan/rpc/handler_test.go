package rpc_test

import (
	"context"
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

func getMyPlan(t *testing.T, acting plan.Plan) *postpilotv1.GetMyPlanResponse {
	t.Helper()
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "alice", Plan: acting})
	res, err := planrpc.NewHandler(stubLedger{}).GetMyPlan(
		ctx, connect.NewRequest(&postpilotv1.GetMyPlanRequest{}),
	)
	if err != nil {
		t.Fatalf("GetMyPlan: %v", err)
	}
	return res.Msg
}

// Every figure a comparison screen shows crosses here, so the offers it publishes are
// pinned against the ladder itself rather than against copies of the numbers.
func TestGetMyPlanPublishesEstimatesAndTheRecommendedRung(t *testing.T) {
	msg := getMyPlan(t, plan.Basic)

	if len(msg.Offers) != len(plan.Offers()) {
		t.Fatalf("offers = %d, want %d", len(msg.Offers), len(plan.Offers()))
	}

	marked := make([]postpilotv1.Plan, 0, 1)
	for _, offer := range msg.Offers {
		domain, ok := planrpc.FromProto(offer.Plan)
		if !ok {
			t.Fatalf("offer %v is not a known rung", offer.Plan)
		}
		if want := int32(plan.EstimatedPosts(domain)); offer.EstimatedPosts != want {
			t.Errorf("%s estimated_posts = %d, want %d", domain, offer.EstimatedPosts, want)
		}
		// Every shipped rung's grant covers at least one reference post, so a zero here
		// means the estimate was dropped on the wire rather than genuinely floored.
		if offer.EstimatedPosts == 0 {
			t.Errorf("%s published no post estimate", domain)
		}
		if offer.Recommended {
			marked = append(marked, offer.Plan)
		}
	}

	if len(marked) != 1 || marked[0] != postpilotv1.Plan_PLAN_PRO {
		t.Errorf("recommended offers = %v, want exactly [PLAN_PRO]", marked)
	}
}
