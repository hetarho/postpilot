package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/purpose"
	"github.com/postpilot/backend/internal/purpose/store"
)

var testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// newStore opens a throwaway SQLite database with the embedded migrations applied and seeds
// the two accounts, their voices, and one post each — a post names a voice, and the purpose
// counts and detaches are about real posts pointing at real purposes.
func newStore(t *testing.T) (*store.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(context.Background(), auth.User{ID: id, PasswordHash: "hash", CreatedAt: testNow}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
		if _, err := handle.Writer.Exec("INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,?,1,?,?)",
			id+"-voice", id, "기본 말투", stamp, stamp); err != nil {
			t.Fatalf("seed voice %s: %v", id, err)
		}
	}
	return store.New(handle.Writer, handle.Reader), handle
}

func seedPost(t *testing.T, handle *db.DB, slug, userID, purposeID string) {
	t.Helper()
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	var value any
	if purposeID != "" {
		value = purposeID
	}
	if _, err := handle.Writer.Exec(
		"INSERT INTO posts(slug,user_id,voice_id,purpose_id,title,memo,status,created_at,updated_at) VALUES(?,?,?,?,'','','draft',?,?)",
		slug, userID, userID+"-voice", value, stamp, stamp); err != nil {
		t.Fatalf("seed post %s: %v", slug, err)
	}
}

func newPurpose(id, userID, name string) purpose.Purpose {
	return purpose.Purpose{
		ID: id, UserID: userID, Name: name, Description: "설명", Instructions: "지침",
		CreatedAt: testNow, UpdatedAt: testNow,
	}
}

func TestInsertRefusesADuplicateNameWithinTheAccountOnly(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	if err := s.Insert(ctx, newPurpose("p1", "alice", "리뷰")); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, newPurpose("p2", "alice", "리뷰")); !errors.Is(err, purpose.ErrDuplicateName) {
		t.Fatalf("duplicate within the account: %v", err)
	}
	if err := s.Insert(ctx, newPurpose("p3", "bob", "리뷰")); err != nil {
		t.Fatalf("same name in another account refused: %v", err)
	}
}

// The presence contract, proved against the real statements: an edit writes only the columns
// it names, so a value another writer saved in between is still there afterwards.
func TestUpdateWritesOnlyThePresentColumns(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newPurpose("p1", "alice", "리뷰")); err != nil {
		t.Fatal(err)
	}

	later := testNow.Add(time.Minute)
	if _, err := s.Update(ctx, "alice", "p1", purpose.Patch{Name: strptr("새 이름")}, later); err != nil {
		t.Fatal(err)
	}
	updated, err := s.Update(ctx, "alice", "p1", purpose.Patch{Instructions: strptr("새 지침")}, later.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "새 이름" || updated.Description != "설명" || updated.Instructions != "새 지침" {
		t.Fatalf("presence update rewrote absent columns: %+v", updated)
	}
}

func TestUpdateRefusesForeignUnknownAndDuplicateNames(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for _, p := range []purpose.Purpose{newPurpose("p1", "alice", "리뷰"), newPurpose("p2", "alice", "일기")} {
		if err := s.Insert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.Update(ctx, "bob", "p1", purpose.Patch{Name: strptr("훔치기")}, testNow); !errors.Is(err, purpose.ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if _, err := s.Update(ctx, "alice", "nope", purpose.Patch{Name: strptr("없음")}, testNow); !errors.Is(err, purpose.ErrNotFound) {
		t.Fatalf("unknown update: %v", err)
	}
	if _, err := s.Update(ctx, "alice", "p2", purpose.Patch{Name: strptr("리뷰")}, testNow); !errors.Is(err, purpose.ErrDuplicateName) {
		t.Fatalf("rename onto a taken name: %v", err)
	}
	// The refused rename must have rolled back with the rest of its transaction.
	kept, err := s.Get(ctx, "alice", "p2")
	if err != nil || kept.Name != "일기" {
		t.Fatalf("refused rename leaked: %+v err=%v", kept, err)
	}
}

func TestDeleteDetachesPostsAndCountsThemInTheSameTransaction(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	for _, p := range []purpose.Purpose{newPurpose("p1", "alice", "리뷰"), newPurpose("p2", "alice", "일기")} {
		if err := s.Insert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	seedPost(t, handle, "a1", "alice", "p1")
	seedPost(t, handle, "a2", "alice", "p1")
	seedPost(t, handle, "a3", "alice", "")

	detached, err := s.Delete(ctx, "alice", "p1")
	if err != nil || detached != 2 {
		t.Fatalf("detached=%d err=%v", detached, err)
	}
	var posts, assigned int
	if err := handle.Reader.QueryRow(`SELECT count(*), count(purpose_id) FROM posts WHERE user_id='alice'`).Scan(&posts, &assigned); err != nil {
		t.Fatal(err)
	}
	if posts != 3 || assigned != 0 {
		t.Fatalf("delete removed posts or left assignments: posts=%d assigned=%d", posts, assigned)
	}

	if detached, err := s.Delete(ctx, "alice", "p2"); err != nil || detached != 0 {
		t.Fatalf("unreferenced delete: detached=%d err=%v", detached, err)
	}
	if _, err := s.Delete(ctx, "alice", "p2"); !errors.Is(err, purpose.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestListCountsPostsPerPurposeAndStaysWithinTheAccount(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	for _, p := range []purpose.Purpose{
		newPurpose("p1", "alice", "리뷰"), newPurpose("p2", "alice", "일기"), newPurpose("p3", "bob", "리뷰"),
	} {
		if err := s.Insert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	seedPost(t, handle, "a1", "alice", "p1")
	seedPost(t, handle, "b1", "bob", "p3")

	purposes, err := s.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(purposes) != 2 {
		t.Fatalf("list crossed the account boundary: %+v", purposes)
	}
	// Ordered by name then id, as the directory query promises.
	if purposes[0].Name != "리뷰" || purposes[1].Name != "일기" {
		t.Fatalf("unexpected order: %q, %q", purposes[0].Name, purposes[1].Name)
	}
	if purposes[0].PostCount != 1 || purposes[1].PostCount != 0 {
		t.Fatalf("post counts = %d, %d", purposes[0].PostCount, purposes[1].PostCount)
	}
}

func strptr(value string) *string { return &value }
