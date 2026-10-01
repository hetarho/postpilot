package rpc_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	"github.com/postpilot/backend/internal/experiment"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
)

type fakeComparisonCosts struct {
	stage  experiment.Stage
	window experiment.Window
	rows   []experiment.ComparisonCostRow
	err    error
}

func (f *fakeComparisonCosts) ComparisonCosts(_ context.Context, stage experiment.Stage, window experiment.Window) ([]experiment.ComparisonCostRow, error) {
	f.stage, f.window = stage, window
	return f.rows, f.err
}

func TestComparisonCostsAreMasterOnlyAndProjectCountedModelTotals(t *testing.T) {
	authClient, admin, _ := newPlanServer(t)
	free := loginAs(t, authClient, "alice")
	request := &postpilotv1.ListComparisonCostsRequest{
		Stage: postpilotv1.Stage_STAGE_WRITE, Window: postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_DAY,
	}
	_, err := admin.ListComparisonCosts(context.Background(), withCookie(request, free))
	if connect.CodeOf(err) != connect.CodePermissionDenied || authAppErrorDetail(t, err).GetReason() != plan.ReasonMasterOnly {
		t.Fatalf("non-master comparison cost read = %v", err)
	}

	reader := &fakeComparisonCosts{rows: []experiment.ComparisonCostRow{{
		Model: experiment.ModelRef{ProviderID: "p", ModelID: "model"}, ModelLabel: "Model",
		EvaluatedComparisons: 2, TotalCostMicrousd: 4_200, CostQuality: experiment.CostEstimated,
	}}}
	handler := authrpc.NewAdminHandler(nil, nil, nil, reader)
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "root", Plan: plan.Master})
	response, err := handler.ListComparisonCosts(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	rows := response.Msg.GetRows()
	if reader.stage != experiment.StageWrite || reader.window != experiment.WindowDay || len(rows) != 1 ||
		rows[0].GetModel().GetModelId() != "model" || rows[0].GetEvaluatedComparisons() != 2 ||
		rows[0].GetTotalCostMicrousd() != 4_200 || rows[0].GetCostQuality() != postpilotv1.CostSource_COST_SOURCE_ESTIMATED {
		t.Fatalf("admin cost projection = %+v / %+v", reader, rows)
	}
	if _, err := handler.ListComparisonCosts(ctx, connect.NewRequest(&postpilotv1.ListComparisonCostsRequest{})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("missing stage/window accepted: %v", err)
	}
	if _, err := handler.ListComparisonCosts(ctx, connect.NewRequest(&postpilotv1.ListComparisonCostsRequest{Stage: postpilotv1.Stage_STAGE_WRITE})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("missing window accepted: %v", err)
	}
	reader.err = errors.New("private storage failure")
	if _, err := handler.ListComparisonCosts(ctx, connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("storage failure = %v", err)
	}
}
