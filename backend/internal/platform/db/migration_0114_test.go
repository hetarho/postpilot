package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0114AdmitsLightWithoutLosingAccountsOrChildren(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 113); err != nil {
		t.Fatal(err)
	}

	for _, statement := range []string{
		"INSERT INTO users(id,password_hash,created_at,plan,email,email_verified_at,failed_logins) VALUES('alice','hash','2026-09-30T00:00:00Z','pro','alice@example.com','2026-09-30T00:00:00Z',2)",
		"INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES('token','alice','2026-10-30T00:00:00Z','2026-09-30T00:00:00Z')",
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := provider.UpTo(ctx, 114); err != nil {
		t.Fatal(err)
	}
	var plan, email string
	var failures int
	if err := handle.Reader.QueryRow("SELECT plan,email,failed_logins FROM users WHERE id='alice'").Scan(&plan, &email, &failures); err != nil || plan != "pro" || email != "alice@example.com" || failures != 2 {
		t.Fatalf("account changed: plan=%q email=%q failures=%d err=%v", plan, email, failures, err)
	}
	var children int
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id='alice'").Scan(&children); err != nil || children != 1 {
		t.Fatalf("child session lost: count=%d err=%v", children, err)
	}
	if _, err := handle.Writer.Exec("UPDATE users SET plan='light' WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec("UPDATE users SET plan='unknown' WHERE id='alice'"); err == nil {
		t.Fatal("unknown plan accepted")
	}
	if _, err := provider.DownTo(ctx, 113); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow("SELECT plan FROM users WHERE id='alice'").Scan(&plan); err != nil || plan != "basic" {
		t.Fatalf("rollback should map light to basic, got %q: %v", plan, err)
	}
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id='alice'").Scan(&children); err != nil || children != 1 {
		t.Fatalf("rollback lost child session: count=%d err=%v", children, err)
	}
}
