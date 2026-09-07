package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0029AddsNullableUniqueIdentitiesAndCheckedLinks(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const created = "2026-09-08T00:00:00.000000000Z"
	for _, id := range []string{"alice", "bob"} {
		if _, err := handle.Writer.ExecContext(ctx,
			`INSERT INTO users(id,password_hash,plan,created_at) VALUES(?,'hash','free',?)`, id, created); err != nil {
			t.Fatalf("insert nullable-email user %s: %v", id, err)
		}
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`UPDATE users SET email='owner@example.com', google_subject='google-1' WHERE id='alice'`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`UPDATE users SET email='owner@example.com' WHERE id='bob'`); err == nil {
		t.Fatal("duplicate email was accepted")
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`UPDATE users SET google_subject='google-1' WHERE id='bob'`); err == nil {
		t.Fatal("duplicate google_subject was accepted")
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO auth_links(token_hash,user_id,purpose,email,expires_at,created_at)
		 VALUES('bad','alice','magic','owner@example.com',?,?)`, created, created); err == nil {
		t.Fatal("off-list auth link purpose was accepted")
	}

	for _, index := range []string{"idx_users_email", "idx_users_google_subject", "idx_auth_links_user_purpose"} {
		var count int
		if err := handle.Reader.QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&count); err != nil || count != 1 {
			t.Fatalf("index %s count=%d err=%v", index, count, err)
		}
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 28); err != nil {
		t.Fatalf("down to 28: %v", err)
	}
	var links int
	if err := handle.Reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='auth_links'`).Scan(&links); err != nil || links != 0 {
		t.Fatalf("auth_links survived rollback: count=%d err=%v", links, err)
	}
	for _, column := range []string{
		"email", "email_verified_at", "email_unreachable_at", "failed_logins", "locked_until", "google_subject",
	} {
		var count int
		if err := handle.Reader.QueryRowContext(ctx,
			`SELECT count(*) FROM pragma_table_info('users') WHERE name=?`, column).Scan(&count); err != nil || count != 0 {
			t.Fatalf("users.%s survived rollback: count=%d err=%v", column, count, err)
		}
	}
}
