package db

import (
	"context"
	"strings"
	"testing"
)

func TestMigration0030CreatesCheckedBillingLedger(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-08T00:00:00.000000000Z"
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO users(id,password_hash,plan,created_at) VALUES('alice','hash','free',?)`, at); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		sql  string
	}{
		{"subscription tier", `INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,status,created_at,updated_at) VALUES('alice','free','monthly',?,?,?,?, 'active',?,?)`},
		{"subscription term", `INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,status,created_at,updated_at) VALUES('alice','basic','weekly',?,?,?,?, 'active',?,?)`},
		{"subscription status", `INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,status,created_at,updated_at) VALUES('alice','basic','monthly',?,?,?,?, 'paused',?,?)`},
		{"subscription auto renew", `INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,auto_renew,status,created_at,updated_at) VALUES('alice','basic','monthly',?,?,?,?,2,'active',?,?)`},
		{"subscription scheduled tier", `INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,scheduled_tier,status,created_at,updated_at) VALUES('alice','basic','monthly',?,?,?,?,'free','active',?,?)`},
		{"subscription scheduled term", `INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,scheduled_term,status,created_at,updated_at) VALUES('alice','basic','monthly',?,?,?,?,'weekly','active',?,?)`},
		{"event kind", `INSERT INTO billing_events(user_id,kind,created_at) VALUES('alice','changed',?)`},
		{"event tier", `INSERT INTO billing_events(user_id,kind,tier,created_at) VALUES('alice','charge','free',?)`},
		{"event term", `INSERT INTO billing_events(user_id,kind,term,created_at) VALUES('alice','charge','weekly',?)`},
		{"purchase credits", `INSERT INTO credit_purchases(id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at) VALUES('invalid-credits','alice','lot',0,100,1300,'pay','invalid-credits',?)`},
		{"purchase USD", `INSERT INTO credit_purchases(id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at) VALUES('invalid-usd','alice','lot',100,0,1300,'pay','invalid-usd',?)`},
		{"purchase KRW", `INSERT INTO credit_purchases(id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at) VALUES('invalid-krw','alice','lot',100,100,0,'pay','invalid-krw',?)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []any{at}
			if strings.HasPrefix(tc.name, "subscription") {
				args = []any{at, at, at, at, at, at}
			}
			if _, err := handle.Writer.ExecContext(ctx, tc.sql, args...); err == nil {
				t.Fatalf("CHECK accepted invalid row")
			}
		})
	}

	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO billing_events(user_id,kind,order_id,created_at) VALUES('alice','charge','order-1',?)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO billing_events(user_id,kind,order_id,created_at) VALUES('alice','refund','order-1',?)`, at); err == nil {
		t.Fatal("duplicate billing event order_id was accepted")
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO credit_purchases(id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at) VALUES('p1','alice','lot-1',100,100,1300,'pay-1','purchase-1',?)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO credit_purchases(id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at) VALUES('p2','alice','lot-2',100,100,1300,'pay-2','purchase-1',?)`, at); err == nil {
		t.Fatal("duplicate purchase order_id was accepted")
	}
}
