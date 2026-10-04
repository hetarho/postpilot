package db

import (
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// BILL-15: 0132 drops the dollar and exchange-rate columns of billing_events and
// credit_purchases in place, and every other value of a row written before it reads back
// unchanged — KRW above all. Down brings the columns back empty (a purchase's dollar amount as
// the zero every fixed-KRW row already held), and Up runs again over that.
func TestMigration0132DropsTheBillingUSDColumnsAndKeepsKRW(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	if _, err := p.UpTo(ctx, 131); err != nil {
		t.Fatal(err)
	}
	const at = "2026-10-01T00:00:00.000000000Z"
	mustExec(t, h.Writer, `INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at)
	mustExec(t, h.Writer, `INSERT INTO billing_events(user_id,kind,tier,term,credits,usd_cents,krw_per_usd_e4,rate_date,
		krw,provider_payment_key,order_id,note,created_at)
		VALUES('alice','charge',NULL,NULL,1000,100,13925000,'2026-09-07',3000,'payment-1','order-1','order-1',?)`, at)
	mustExec(t, h.Writer, `INSERT INTO billing_events(user_id,kind,tier,term,krw,order_id,created_at)
		VALUES('alice','charge','pro','monthly',9900,'order-2',?)`, at)
	mustExec(t, h.Writer, `INSERT INTO credit_purchases(id,user_id,lot_id,pack_id,credits,usd_cents,krw,
		provider_payment_key,order_id,charged_at,refunded_at)
		VALUES('order-1','alice','lot-1','pack-1000',1000,100,3000,'payment-1','order-1',?,?)`, at, at)

	if _, err := p.UpTo(ctx, 132); err != nil {
		t.Fatal(err)
	}
	columns := func(table string, names ...string) int {
		t.Helper()
		total := 0
		for _, name := range names {
			var n int
			if err := h.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name=?`, table, name).Scan(&n); err != nil {
				t.Fatal(err)
			}
			total += n
		}
		return total
	}
	if n := columns("billing_events", "usd_cents", "krw_per_usd_e4", "rate_date") + columns("credit_purchases", "usd_cents"); n != 0 {
		t.Fatalf("%d USD columns left after 0132", n)
	}
	var pack, tier, term, key, note string
	var credits, krw, subscriptionKRW int
	if err := h.Reader.QueryRow(`SELECT credits,krw,provider_payment_key,note FROM billing_events WHERE order_id='order-1'`).
		Scan(&credits, &krw, &key, &note); err != nil || credits != 1000 || krw != 3000 || key != "payment-1" || note != "order-1" {
		t.Fatalf("pack charge = %d %d %q %q, err %v", credits, krw, key, note, err)
	}
	if err := h.Reader.QueryRow(`SELECT tier,term,krw FROM billing_events WHERE order_id='order-2'`).
		Scan(&tier, &term, &subscriptionKRW); err != nil || tier != "pro" || term != "monthly" || subscriptionKRW != 9900 {
		t.Fatalf("subscription charge = %q %q %d, err %v", tier, term, subscriptionKRW, err)
	}
	var refunded string
	if err := h.Reader.QueryRow(`SELECT pack_id,credits,krw,refunded_at FROM credit_purchases WHERE id='order-1'`).
		Scan(&pack, &credits, &krw, &refunded); err != nil || pack != "pack-1000" || credits != 1000 || krw != 3000 || refunded != at {
		t.Fatalf("purchase = %q %d %d %q, err %v", pack, credits, krw, refunded, err)
	}
	// The table's own CHECKs and the replay guard 0121 put on credit_purchases still hold.
	mustExec(t, h.Writer, `INSERT INTO test_entitlement_resets(id,completed_at,backup_sha256,report_json) VALUES('r','x','x','{}')`)
	mustExec(t, h.Writer, `INSERT INTO test_entitlement_reset_orders(order_id,reset_id) VALUES('retired','r')`)
	for name, stmt := range map[string]string{
		"zero credits":  `INSERT INTO credit_purchases(id,user_id,lot_id,credits,krw,provider_payment_key,order_id,charged_at) VALUES('bad-credits','alice','lot',0,3000,'p','bad-credits',?)`,
		"zero KRW":      `INSERT INTO credit_purchases(id,user_id,lot_id,credits,krw,provider_payment_key,order_id,charged_at) VALUES('bad-krw','alice','lot',1000,0,'p','bad-krw',?)`,
		"retired order": `INSERT INTO credit_purchases(id,user_id,lot_id,credits,krw,provider_payment_key,order_id,charged_at) VALUES('retired','alice','lot',1000,3000,'p','retired',?)`,
	} {
		if _, err := h.Writer.Exec(stmt, at); err == nil {
			t.Fatalf("%s: accepted after 0132", name)
		}
	}

	if _, err := p.DownTo(ctx, 131); err != nil {
		t.Fatal(err)
	}
	if n := columns("billing_events", "usd_cents", "krw_per_usd_e4", "rate_date") + columns("credit_purchases", "usd_cents"); n != 4 {
		t.Fatalf("%d of 4 USD columns back after Down", n)
	}
	var cents int
	if err := h.Reader.QueryRow(`SELECT usd_cents,krw FROM credit_purchases WHERE id='order-1'`).Scan(&cents, &krw); err != nil || cents != 0 || krw != 3000 {
		t.Fatalf("purchase after Down = %d cents %d KRW, err %v", cents, krw, err)
	}
	if _, err := p.UpTo(ctx, 132); err != nil {
		t.Fatal(err)
	}
}
