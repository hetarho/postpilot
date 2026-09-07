package store_test

import (
	"context"
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

func TestRegistrationReplacesTheCardAndPersistsOneNonExpiringBonus(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "billing.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO users (id, password_hash, plan, email, email_verified_at, created_at) VALUES (?, 'hash', 'free', ?, ?, ?)",
		"alice", "alice@example.com", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}

	store := billingstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(conn *sql.Conn) billing.Credits {
		return usage.NewService(usagestore.NewTx(conn), nil, 0)
	})
	store.SetPlansForTx(func(*sql.Conn) billing.Plans { return registrationPlans{} })
	provider := &registrationProvider{label: "11 1234"}
	service := billing.NewService(store, provider, registrationRates{}, nil, nil, registrationAccounts{}, nil)

	first, err := service.RegisterPaymentMethod(ctx, "alice", "auth-1", billing.CustomerKey("alice"))
	if err != nil || !first.BonusGranted {
		t.Fatalf("first registration = %+v, %v", first, err)
	}
	provider.label = "22 9876"
	second, err := service.RegisterPaymentMethod(ctx, "alice", "auth-2", billing.CustomerKey("alice"))
	if err != nil || second.BonusGranted {
		t.Fatalf("second registration = %+v, %v", second, err)
	}

	var label string
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT card_label FROM payment_methods WHERE user_id = ?", "alice").Scan(&label); err != nil || label != "22 9876" {
		t.Fatalf("stored label = %q, %v", label, err)
	}
	var kind string
	var granted int
	var expiresAt sql.NullString
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT kind, granted, expires_at FROM credit_lots WHERE id = ?", "payment-method-bonus:alice").Scan(&kind, &granted, &expiresAt); err != nil {
		t.Fatal(err)
	}
	if kind != "bonus" || granted != 100 || expiresAt.Valid {
		t.Fatalf("bonus lot = kind %q, granted %d, expires %+v", kind, granted, expiresAt)
	}
	var registrations, grants int
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM billing_events WHERE user_id = ? AND kind = 'method_registered'", "alice").Scan(&registrations); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM billing_events WHERE user_id = ? AND kind = 'grant'", "alice").Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if registrations != 2 || grants != 1 {
		t.Fatalf("events: registrations=%d grants=%d", registrations, grants)
	}
}

func TestSubscribePersistsSubscriptionTierEventsAndMonthlyLotTogether(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "subscription.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO users (id, password_hash, plan, email, email_verified_at, created_at) VALUES (?, 'hash', 'free', ?, ?, ?)",
		"alice", "alice@example.com", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}

	store := billingstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(conn *sql.Conn) billing.Credits {
		return usage.NewService(usagestore.NewTx(conn), nil, 0)
	})
	store.SetPlansForTx(func(conn *sql.Conn) billing.Plans {
		return auth.NewService(authstore.NewTx(conn), time.Hour)
	})
	if err := store.UpsertPaymentMethod(ctx, billing.PaymentMethod{
		UserID: "alice", Provider: "toss", BillingKey: "billing-key",
		CustomerKey: billing.CustomerKey("alice"), CardLabel: "11 1234", RegisteredAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := billing.NewService(store, &registrationProvider{}, registrationRates{}, nil, nil, nil, nil)
	subscription, err := service.Subscribe(ctx, "alice", plan.Pro, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}

	var tier, status string
	if err := handle.Reader.QueryRowContext(ctx, "SELECT plan FROM users WHERE id = ?", "alice").Scan(&tier); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, "SELECT status FROM subscriptions WHERE user_id = ?", "alice").Scan(&status); err != nil {
		t.Fatal(err)
	}
	var events, lots int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM billing_events WHERE user_id = ?", "alice").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_lots WHERE user_id = ? AND kind = 'monthly'", "alice").Scan(&lots); err != nil {
		t.Fatal(err)
	}
	if tier != "pro" || status != "active" || events != 2 || lots != 1 || subscription.NextGrantAt.IsZero() {
		t.Fatalf("tier=%s status=%s events=%d lots=%d subscription=%+v", tier, status, events, lots, subscription)
	}
}

type registrationAccounts struct{}

func (registrationAccounts) VerifiedEmail(context.Context, string) (string, bool, error) {
	return "alice@example.com", true, nil
}

type registrationRates struct{}

func (registrationRates) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 14_000_000, true, nil
}

type registrationPlans struct{}

func (registrationPlans) AssignTier(context.Context, string, plan.Plan) error { return nil }
func (registrationPlans) TierOf(context.Context, string) (plan.Plan, error)   { return plan.Free, nil }

type registrationProvider struct{ label string }

func (p *registrationProvider) IssueBillingKey(_ context.Context, _, customerKey string) (billing.BillingKey, error) {
	return billing.BillingKey{Value: "secret-billing-key", CustomerKey: customerKey, CardLabel: p.label}, nil
}
func (*registrationProvider) Charge(_ context.Context, request billing.ChargeRequest) (billing.Payment, error) {
	return billing.Payment{PaymentKey: "payment-1", OrderID: request.OrderID, Status: "DONE"}, nil
}
func (*registrationProvider) PaymentByOrder(context.Context, string) (billing.Payment, bool, error) {
	return billing.Payment{}, false, nil
}
func (*registrationProvider) Refund(context.Context, string, string) error { return nil }
func (*registrationProvider) ParseNotification(*http.Request) (billing.Notification, error) {
	return billing.Notification{}, nil
}
