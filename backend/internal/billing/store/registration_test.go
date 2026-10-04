package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

func TestRegistrationReplacesTheCardWithoutMintingCredits(t *testing.T) {
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
	store.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return testCredits{Service: usage.NewService(usagestore.NewTx(tx), nil, 0, fixedAnchor{at: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, testRates)}
	})
	store.SetPlansForTx(func(*sql.Tx) billing.Plans { return registrationPlans{} })
	provider := &registrationProvider{label: "11 1234"}
	service := billing.NewService(store, provider, nil, nil, registrationAccounts{}, nil)

	first, err := service.RegisterPaymentMethod(ctx, "alice", "auth-1", billing.CustomerKey("alice"))
	if err != nil || first.BonusGranted {
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
	var grants int
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM credit_lots WHERE user_id = ?", "alice").Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if grants != 0 {
		t.Fatalf("registration minted %d credit lots", grants)
	}
	var registrations, grantEvents int
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM billing_events WHERE user_id = ? AND kind = 'method_registered'", "alice").Scan(&registrations); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM billing_events WHERE user_id = ? AND kind = 'grant'", "alice").Scan(&grantEvents); err != nil {
		t.Fatal(err)
	}
	if registrations != 2 || grantEvents != 0 {
		t.Fatalf("events: registrations=%d grants=%d", registrations, grantEvents)
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
	store.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return testCredits{Service: usage.NewService(usagestore.NewTx(tx), nil, 0, fixedAnchor{at: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, testRates)}
	})
	store.SetPlansForTx(func(tx *sql.Tx) billing.Plans {
		return auth.NewService(authstore.NewTx(tx), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	})
	if err := store.UpsertPaymentMethod(ctx, billing.PaymentMethod{
		UserID: "alice", Provider: "toss", BillingKey: "billing-key",
		CustomerKey: billing.CustomerKey("alice"), CardLabel: "11 1234", RegisteredAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := billing.NewService(store, &registrationProvider{}, nil, nil, nil, nil)
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

func TestPurchaseAndRefundPersistOneMoneyLedgerAndOneCreditLot(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "purchase.db"))
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

	usageStore := usagestore.New(handle.Writer, handle.Reader)
	ledger := usage.NewService(usageStore, nil, 0, fixedAnchor{at: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, testRates)
	store := billingstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return testCredits{Service: usage.NewService(usagestore.NewTx(tx), nil, 0, fixedAnchor{at: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, testRates)}
	})
	store.SetPlansForTx(func(*sql.Tx) billing.Plans { return registrationPlans{} })
	if err := store.UpsertPaymentMethod(ctx, billing.PaymentMethod{
		UserID: "alice", Provider: "toss", BillingKey: "billing-key",
		CustomerKey: billing.CustomerKey("alice"), CardLabel: "11 1234", RegisteredAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := billing.NewService(store, &registrationProvider{}, testCredits{Service: ledger}, nil, nil, nil)
	// A pack is for an active paid subscriber (BILL-9).
	if _, err := service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	purchase, err := service.PurchasePack(ctx, "alice", "pack-1000")
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.GetMyBilling(ctx, "alice")
	if err != nil || len(view.Purchases) != 1 || !view.Purchases[0].Refundable {
		t.Fatalf("billing view=%+v err=%v", view, err)
	}
	refunded, err := service.RefundPurchase(ctx, "alice", purchase.ID)
	if err != nil || refunded.RefundedAt == nil {
		t.Fatalf("refund=%+v err=%v", refunded, err)
	}
	var remaining, events int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT remaining FROM credit_lots WHERE id = ?", purchase.LotID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM billing_events WHERE user_id = ? AND kind IN ('charge','refund') AND note = ?", "alice", purchase.ID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 || events != 2 {
		t.Fatalf("remaining=%d events=%d", remaining, events)
	}
}

type registrationAccounts struct{}

func (registrationAccounts) VerifiedEmail(context.Context, string) (string, bool, error) {
	return "alice@example.com", true, nil
}

type registrationPlans struct{}

func (registrationPlans) AssignTier(context.Context, string, plan.Plan) error   { return nil }
func (registrationPlans) ReassignTier(context.Context, string, plan.Plan) error { return nil }
func (registrationPlans) TierOf(context.Context, string) (plan.Plan, error)     { return plan.Free, nil }

// registrationProvider captures every charge in full and answers the order read-back with it,
// the evidence settlement applies a payment on.
type registrationProvider struct {
	label  string
	mu     sync.Mutex
	orders map[string]billing.Payment
}

func (p *registrationProvider) IssueBillingKey(_ context.Context, _, customerKey string) (billing.BillingKey, error) {
	return billing.BillingKey{Value: "secret-billing-key", CustomerKey: customerKey, CardLabel: p.label}, nil
}
func (p *registrationProvider) Charge(_ context.Context, request billing.ChargeRequest) (billing.Payment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.orders == nil {
		p.orders = map[string]billing.Payment{}
	}
	payment := billing.Payment{PaymentKey: "payment-" + request.OrderID, OrderID: request.OrderID, Status: "DONE",
		AmountKRW: request.KRW, BalanceKRW: request.KRW, Currency: "KRW"}
	p.orders[request.OrderID] = payment
	return payment, nil
}
func (p *registrationProvider) PaymentByOrder(_ context.Context, orderID string) (billing.Payment, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	payment, found := p.orders[orderID]
	return payment, found, nil
}
func (*registrationProvider) Refund(context.Context, string, string) error { return nil }
func (*registrationProvider) ParseNotification([]byte) (billing.Notification, error) {
	return billing.Notification{}, nil
}
