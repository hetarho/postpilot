package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

func TestUsageAnchorPrefersAnActiveSubscriptionAndFallsBackAfterLapse(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "anchor.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	created := time.Now().AddDate(0, 0, -3)
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Pro, CreatedAt: created}); err != nil {
		t.Fatal(err)
	}

	anchor := time.Now().AddDate(0, -1, -7)
	start, end := plan.AnchorWindow(anchor, time.Now())
	billingStore := billingstore.New(handle.Writer, handle.Reader)
	active := billing.Subscription{UserID: "alice", Tier: plan.Pro, Term: billing.TermMonthly, AnchorAt: anchor, TermStart: start, TermEnd: end, NextGrantAt: end, AutoRenew: true, Status: "active", CreatedAt: anchor, UpdatedAt: anchor}
	if err := billingStore.UpsertSubscription(ctx, active); err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour)
	billingService := billing.NewService(billingStore, nil, nil, nil, nil, nil, nil)
	anchors := usageAnchors{auth: authService, billing: billingService}
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), emptyModels{}, 0)
	ledger.SetAnchors(anchors)

	response, err := planrpc.NewHandler(ledger, emptyBillingEstimator{}).GetMyPlan(
		auth.WithActor(ctx, auth.Actor{UserID: "alice", Plan: plan.Pro}),
		connect.NewRequest(&postpilotv1.GetMyPlanRequest{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	renewsAt, err := time.Parse(time.RFC3339, response.Msg.GetBalance().GetRenewsAt())
	if err != nil {
		t.Fatal(err)
	}
	if want := plan.NextRenewal(anchor, time.Now()); !renewsAt.Equal(want) {
		t.Fatalf("GetMyPlan renews_at = %s, want %s", renewsAt, want)
	}

	active.Status = "lapsed"
	active.UpdatedAt = time.Now()
	if err := billingStore.UpsertSubscription(ctx, active); err != nil {
		t.Fatal(err)
	}
	fallback, err := anchors.AnchorFor(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !fallback.Equal(created) {
		t.Fatalf("lapsed fallback = %s, want creation %s", fallback, created)
	}
}

type emptyBillingEstimator struct{}

func (emptyBillingEstimator) ComboRates(context.Context) ([]planrpc.EstimatorCombo, error) {
	return nil, nil
}
