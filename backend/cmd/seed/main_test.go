package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/devseed"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

func TestSeedCreatesShortEmailFreeAccountsThatCanLogIn(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "seed.db")
	t.Setenv("DB_PATH", path)
	t.Setenv("MAIL_DRIVER", "log")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	if err := run(ctx); err != nil {
		t.Fatal(err)
	}
	handle, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	store := authstore.New(handle.Writer, handle.Reader)
	svc := auth.NewService(store, 720*time.Hour, auth.Deps{Mailer: mail.NewLog()})
	expected := []struct {
		id    string
		tier  plan.Plan
		posts int
	}{
		{"free", plan.Free, 0}, {"base", plan.Basic, 3}, {"pro", plan.Pro, 8},
		{"max", plan.Max, 14}, {"master", plan.Master, 23},
	}
	var accounts int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if accounts != len(expected) {
		t.Fatalf("accounts = %d", accounts)
	}
	for _, want := range expected {
		user, token, err := svc.Login(ctx, want.id, devseed.Password)
		if err != nil {
			t.Fatalf("login %s: %v", want.id, err)
		}
		if user.ID != want.id || user.Plan != want.tier || user.Email != "" || user.EmailVerifiedAt != nil || token == "" {
			t.Fatalf("unexpected account identity for %s", want.id)
		}
		var posts int
		if err := handle.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts WHERE user_id = ?", want.id).Scan(&posts); err != nil {
			t.Fatal(err)
		}
		if posts != want.posts {
			t.Errorf("%s posts = %d, want %d", want.id, posts, want.posts)
		}
	}
}
