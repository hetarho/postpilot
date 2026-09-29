package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0117KeepsOldPurchasesAndAllowsKRWOnlyPacks(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 116); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-30T00:00:00.000000000Z"
	mustExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at,plan) VALUES('alice','hash',?,'basic')", at)
	mustExec(t, handle.Writer, `INSERT INTO credit_purchases
      (id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at)
      VALUES('old','alice','lot-old',100,100,1400,'payment-old','order-old',?)`, at)
	if _, err := provider.UpTo(ctx, 117); err != nil {
		t.Fatal(err)
	}
	mustExec(t, handle.Writer, `INSERT INTO credit_purchases
      (id,user_id,lot_id,pack_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at)
      VALUES('new','alice','lot-new','pack-1000',1000,0,3000,'payment-new','order-new',?)`, at)
	var count int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM credit_purchases WHERE user_id='alice'").Scan(&count); err != nil || count != 2 {
		t.Fatalf("purchases after migration=%d err=%v", count, err)
	}
	if _, err := provider.DownTo(ctx, 116); err != nil {
		t.Fatal(err)
	}
	var cents int
	if err := handle.Reader.QueryRow("SELECT usd_cents FROM credit_purchases WHERE id='old'").Scan(&cents); err != nil || cents != 100 {
		t.Fatalf("legacy purchase changed: cents=%d err=%v", cents, err)
	}
	if err := handle.Reader.QueryRow("SELECT usd_cents FROM credit_purchases WHERE id='new'").Scan(&cents); err != nil || cents != 1 {
		t.Fatalf("rollback sentinel: cents=%d err=%v", cents, err)
	}
}
