package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/template/store"
)

var testNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

const body = "<write>인트로</write>"

// newStore opens a throwaway SQLite database with the embedded migrations applied. The real
// schema is the point of this suite: the cap, the unique name, the composite foreign key and
// the detach trigger are all schema behavior that a fake store cannot prove.
func newStore(t *testing.T) (*store.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(ctx, auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: testNow}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
		if _, err := handle.Writer.Exec(
			"INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,?,1,?,?)",
			id+"-voice", id, "기본", testNow.UTC().Format(time.RFC3339Nano), testNow.UTC().Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("seed voice %s: %v", id, err)
		}
	}
	return store.New(handle.Writer, handle.Reader), handle
}

func row(id, userID, name string) template.Template {
	return template.Template{
		ID: id, UserID: userID, Name: name, Body: body, CreatedAt: testNow, UpdatedAt: testNow,
	}
}

func TestInsertEnforcesTheCapAndTheNameWithinOneAccount(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()

	if err := s.Insert(ctx, row("t1", "alice", "리뷰"), 2); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, row("t2", "alice", "리뷰"), 2); !errors.Is(err, template.ErrDuplicateName) {
		t.Fatalf("duplicate name error = %v", err)
	}
	// The same name in another account is fine, and that account's own cap is separate.
	if err := s.Insert(ctx, row("b1", "bob", "리뷰"), 2); err != nil {
		t.Fatalf("a foreign account's name collided: %v", err)
	}
	if err := s.Insert(ctx, row("t3", "alice", "일기"), 2); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, row("t4", "alice", "후기"), 2); !errors.Is(err, template.ErrTooMany) {
		t.Fatalf("cap error = %v", err)
	}
}

func TestGetAndListAreScopedByAccount(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	if err := s.Insert(ctx, row("t1", "alice", "리뷰"), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, row("b1", "bob", "리뷰"), 10); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Get(ctx, "alice", "b1"); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("a foreign id was readable: %v", err)
	}
	listed, err := s.List(ctx, "alice")
	if err != nil || len(listed) != 1 || listed[0].ID != "t1" {
		t.Fatalf("list = %+v err = %v", listed, err)
	}
}

// One statement per present field: a field the patch does not carry is never written, not
// even back to the value this call read.
func TestUpdateTouchesOnlyThePresentFields(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	if err := s.Insert(ctx, row("t1", "alice", "리뷰"), 10); err != nil {
		t.Fatal(err)
	}
	name := "정보성 리뷰"
	later := testNow.Add(time.Minute)

	updated, err := s.Update(ctx, "alice", "t1", template.Patch{Name: &name}, later)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != name || updated.Body != body {
		t.Fatalf("update rewrote a field it was not given: %+v", updated)
	}
	if !updated.UpdatedAt.Equal(later) {
		t.Fatalf("updated_at = %v, want %v", updated.UpdatedAt, later)
	}
	if _, err := s.Update(ctx, "alice", "b1", template.Patch{Name: &name}, later); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("a foreign update error = %v", err)
	}
}

// The composite foreign key is the account guarantee, and the trigger is the detach: deleting
// a template clears the posts that named it and deletes none of them.
func TestDeleteDetachesPostsAndReportsTheCount(t *testing.T) {
	s, handle := newStore(t)
	ctx := context.Background()
	if err := s.Insert(ctx, row("t1", "alice", "리뷰"), 10); err != nil {
		t.Fatal(err)
	}
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	for _, slug := range []string{"p1", "p2"} {
		if _, err := handle.Writer.Exec(
			"INSERT INTO posts(slug,user_id,voice_id,template_id,status,created_at,updated_at) VALUES(?,?,?,?,'draft',?,?)",
			slug, "alice", "alice-voice", "t1", stamp, stamp,
		); err != nil {
			t.Fatalf("seed post %s: %v", slug, err)
		}
	}
	// A post can never name another account's template even if a service check is bypassed.
	if _, err := handle.Writer.Exec(
		"INSERT INTO posts(slug,user_id,voice_id,template_id,status,created_at,updated_at) VALUES('x','bob','bob-voice','t1','draft',?,?)",
		stamp, stamp,
	); err == nil {
		t.Fatal("a post named another account's template")
	}

	// The count comes from the same transaction that detaches, so it cannot be stale.
	detached, err := s.Delete(ctx, "alice", "t1")
	if err != nil || detached != 2 {
		t.Fatalf("detached = %d err = %v", detached, err)
	}
	var posts, assigned int
	if err := handle.Reader.QueryRow(
		"SELECT count(*), count(template_id) FROM posts WHERE user_id='alice'",
	).Scan(&posts, &assigned); err != nil {
		t.Fatal(err)
	}
	if posts != 2 || assigned != 0 {
		t.Fatalf("delete removed posts or left assignments: posts=%d assigned=%d", posts, assigned)
	}
	if _, err := s.Delete(ctx, "alice", "t1"); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("a second delete error = %v", err)
	}
}

// post_count is a projection over POSTS, which is why no template mutation can invalidate it.
func TestListProjectsThePostCount(t *testing.T) {
	s, handle := newStore(t)
	ctx := context.Background()
	if err := s.Insert(ctx, row("t1", "alice", "리뷰"), 10); err != nil {
		t.Fatal(err)
	}
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	if _, err := handle.Writer.Exec(
		"INSERT INTO posts(slug,user_id,voice_id,template_id,status,created_at,updated_at) VALUES('p1','alice','alice-voice','t1','draft',?,?)",
		stamp, stamp,
	); err != nil {
		t.Fatal(err)
	}
	listed, err := s.List(ctx, "alice")
	if err != nil || len(listed) != 1 || listed[0].PostCount != 1 {
		t.Fatalf("list = %+v err = %v", listed, err)
	}
}
