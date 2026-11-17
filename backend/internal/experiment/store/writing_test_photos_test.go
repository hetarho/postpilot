package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
)

func newTestPhoto(id, user string, expires time.Time) experiment.TestPhoto {
	return experiment.TestPhoto{ID: id, UserID: user, Filename: id + ".jpg", Key: "tests/" + user + "/" + id, ExpiresAt: expires, CreatedAt: time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)}
}
func readyTestPhoto(t *testing.T, store *experimentstore.Store, id, user string, expires time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateTestPhoto(ctx, newTestPhoto(id, user, expires)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmTestPhoto(ctx, user, id, 2048, 1024, 768); err != nil {
		t.Fatal(err)
	}
}
func admitModelTest(t *testing.T, store *experimentstore.Store, key string) experiment.WritingTest {
	t.Helper()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, key)
	return admitWriting(t, store, request, plan)
}
func photoIDs(photos []experiment.TestPhoto) []string {
	ids := make([]string, 0, len(photos))
	for _, photo := range photos {
		ids = append(ids, photo.ID)
	}
	return ids
}

// A photo is created pending, confirmed ready with its measures, and answers only its owner.
func TestTestPhotoLifecycleAndOwnership(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	future := time.Now().UTC().Add(time.Hour)
	if err := store.CreateTestPhoto(ctx, newTestPhoto("p1", "alice", future)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateTestPhoto(ctx, newTestPhoto("p2", "alice", future)); err != nil {
		t.Fatal(err)
	}
	pending, err := store.GetTestPhoto(ctx, "alice", "p1")
	if err != nil || pending.Status != experiment.TestPhotoPending || pending.TestID != "" || pending.Bytes != 0 || pending.Key != "tests/alice/p1" {
		t.Fatalf("pending photo = %+v err=%v", pending, err)
	}
	if ready, err := store.ListReadyTestPhotos(ctx, "alice", []string{"p1", "p2"}); err != nil || len(ready) != 0 {
		t.Fatalf("a pending photo listed as ready: %v %v", ready, err)
	}
	confirmed, err := store.ConfirmTestPhoto(ctx, "alice", "p1", 2048, 1024, 768)
	if err != nil || confirmed.Status != experiment.TestPhotoReady || confirmed.Bytes != 2048 || confirmed.Width != 1024 || confirmed.Height != 768 {
		t.Fatalf("confirmed photo = %+v err=%v", confirmed, err)
	}
	// A lost response may be repeated, and a repeat measures nothing again.
	repeated, err := store.ConfirmTestPhoto(ctx, "alice", "p1", 1, 1, 1)
	if err != nil || repeated.Bytes != 2048 {
		t.Fatalf("repeated confirm changed the photo: %+v err=%v", repeated, err)
	}
	// A foreign owner and an unknown id are the same refusal.
	if _, err = store.GetTestPhoto(ctx, "bob", "p1"); !errors.Is(err, experiment.ErrTestPhotoNotFound) {
		t.Fatalf("foreign get = %v", err)
	}
	if _, err = store.GetTestPhoto(ctx, "alice", "missing"); !errors.Is(err, experiment.ErrTestPhotoNotFound) {
		t.Fatalf("unknown get = %v", err)
	}
	if _, err = store.ConfirmTestPhoto(ctx, "bob", "p2", 1, 1, 1); !errors.Is(err, experiment.ErrTestPhotoNotFound) {
		t.Fatalf("foreign confirm = %v", err)
	}
	if still, _ := store.GetTestPhoto(ctx, "alice", "p2"); still.Status != experiment.TestPhotoPending {
		t.Fatalf("a foreign confirm reached the photo: %+v", still)
	}
	// Only the owner's ready photos are listed, whoever asks.
	if ready, err := store.ListReadyTestPhotos(ctx, "alice", []string{"p1", "p2", "missing"}); err != nil || !slices.Equal(photoIDs(ready), []string{"p1"}) {
		t.Fatalf("ready = %v err=%v", photoIDs(ready), err)
	}
	if ready, err := store.ListReadyTestPhotos(ctx, "bob", []string{"p1"}); err != nil || len(ready) != 0 {
		t.Fatalf("a foreign owner listed %v err=%v", photoIDs(ready), err)
	}
	if err = store.CreateTestPhoto(ctx, newTestPhoto("p1", "bob", future)); err == nil {
		t.Fatal("a photo id was created twice")
	}
	duplicate := newTestPhoto("p3", "alice", future)
	duplicate.Key = "tests/alice/p1"
	if err = store.CreateTestPhoto(ctx, duplicate); err == nil {
		t.Fatal("an object key was claimed twice")
	}
}

// Attaching is all or nothing inside the caller's transaction and refuses a photo that is not
// the owner's, not ready, repeated or already held by a test.
func TestAttachTestPhotos(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	future := time.Now().UTC().Add(time.Hour)
	for _, id := range []string{"p1", "p2", "p3"} {
		readyTestPhoto(t, store, id, "alice", future)
	}
	if err := store.CreateTestPhoto(ctx, newTestPhoto("pending", "alice", future)); err != nil {
		t.Fatal(err)
	}
	readyTestPhoto(t, store, "bobs", "bob", future)
	first := admitModelTest(t, store, "photo-test-1")
	second := admitModelTest(t, store, "photo-test-2")
	attach := func(user, testID string, ids ...string) error {
		return experimentstore.AttachTestPhotosTx(ctx, handle.Writer, user, testID, ids)
	}

	for name, call := range map[string]func() error{
		"a pending photo":        func() error { return attach("alice", first.ID, "p1", "pending") },
		"another owner's photo":  func() error { return attach("alice", first.ID, "p1", "bobs") },
		"an unknown photo":       func() error { return attach("alice", first.ID, "p1", "missing") },
		"a repeated photo":       func() error { return attach("alice", first.ID, "p1", "p1") },
		"an empty id":            func() error { return attach("alice", first.ID, "") },
		"a foreign owner's call": func() error { return attach("bob", first.ID, "p1") },
	} {
		if err := call(); !errors.Is(err, experiment.ErrTestPhotoUnavailable) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	// Every refusal rolled back, so the photos that were attachable stayed unattached.
	if held, err := store.ListTestPhotos(ctx, "alice", first.ID); err != nil || len(held) != 0 {
		t.Fatalf("a refused attach left %v err=%v", photoIDs(held), err)
	}
	if err := attach("alice", first.ID); err != nil {
		t.Fatalf("attaching nothing: %v", err)
	}

	if err := attach("alice", first.ID, "p1", "p2"); err != nil {
		t.Fatal(err)
	}
	held, err := store.ListTestPhotos(ctx, "alice", first.ID)
	if err != nil || !slices.Equal(photoIDs(held), []string{"p1", "p2"}) || held[0].TestID != first.ID {
		t.Fatalf("held = %+v err=%v", held, err)
	}
	// A photo one test holds can join no other test, and the call is not repeatable.
	if err = attach("alice", second.ID, "p2", "p3"); !errors.Is(err, experiment.ErrTestPhotoUnavailable) {
		t.Fatalf("attach to a second test = %v", err)
	}
	if err = attach("alice", first.ID, "p1"); !errors.Is(err, experiment.ErrTestPhotoUnavailable) {
		t.Fatalf("attach again = %v", err)
	}
	if third, _ := store.GetTestPhoto(ctx, "alice", "p3"); third.TestID != "" {
		t.Fatalf("a refused attach moved p3 to %q", third.TestID)
	}
	// A test that does not exist cannot hold a photo.
	if err = attach("alice", "missing-test", "p3"); err == nil {
		t.Fatal("a photo was attached to a missing test")
	}
}

// The sweep sees only unattached rows, a held photo cannot be removed alone, and the keys
// still referenced by any row are listed.
func TestTestPhotoSweepViews(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	readyTestPhoto(t, store, "held", "alice", past)
	readyTestPhoto(t, store, "stale", "alice", past)
	readyTestPhoto(t, store, "fresh", "alice", future)
	if err := store.CreateTestPhoto(ctx, newTestPhoto("abandoned", "bob", past)); err != nil {
		t.Fatal(err)
	}
	found := admitModelTest(t, store, "photo-sweep")
	if err := experimentstore.AttachTestPhotosTx(ctx, handle.Writer, "alice", found.ID, []string{"held"}); err != nil {
		t.Fatal(err)
	}

	expired, err := store.ListExpiredUnattachedTestPhotos(ctx, now, 10)
	if err != nil || !slices.Equal(photoIDs(expired), []string{"abandoned", "stale"}) {
		t.Fatalf("expired = %v err=%v", photoIDs(expired), err)
	}
	if limited, err := store.ListExpiredUnattachedTestPhotos(ctx, now, 1); err != nil || len(limited) != 1 {
		t.Fatalf("limited = %v err=%v", photoIDs(limited), err)
	}
	if count, err := store.CountUnattachedTestPhotos(ctx, "alice"); err != nil || count != 2 {
		t.Fatalf("alice unattached = %d err=%v", count, err)
	}
	if count, err := store.CountUnattachedTestPhotos(ctx, "bob"); err != nil || count != 1 {
		t.Fatalf("bob unattached = %d err=%v", count, err)
	}

	if removed, err := store.DeleteUnattachedTestPhoto(ctx, "alice", "held"); err != nil || removed {
		t.Fatalf("a held photo was removed alone: %v %v", removed, err)
	}
	if removed, err := store.DeleteUnattachedTestPhoto(ctx, "alice", "abandoned"); err != nil || removed {
		t.Fatalf("a foreign photo was removed: %v %v", removed, err)
	}
	if removed, err := store.DeleteUnattachedTestPhoto(ctx, "alice", "stale"); err != nil || !removed {
		t.Fatalf("an unattached photo was kept: %v %v", removed, err)
	}
	keys, err := store.ListTestPhotoKeys(ctx)
	slices.Sort(keys)
	if err != nil || !slices.Equal(keys, []string{"tests/alice/fresh", "tests/alice/held", "tests/bob/abandoned"}) {
		t.Fatalf("keys = %v err=%v", keys, err)
	}
}

// Purging a test's private payload deletes the photo rows it holds with the rest of it, keeps
// the owner's other uploads, and leaves the test itself; the migration's default keeps it a post.
func TestPurgeTestDeletesItsPhotos(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	future := time.Now().UTC().Add(time.Hour)
	readyTestPhoto(t, store, "held-1", "alice", future)
	readyTestPhoto(t, store, "held-2", "alice", future)
	readyTestPhoto(t, store, "waiting", "alice", future)
	found := admitModelTest(t, store, "photo-purge")
	if err := experimentstore.AttachTestPhotosTx(ctx, handle.Writer, "alice", found.ID, []string{"held-1", "held-2"}); err != nil {
		t.Fatal(err)
	}
	var output string
	if err := handle.Reader.QueryRow(`SELECT output FROM writing_tests WHERE id=?`, found.ID).Scan(&output); err != nil || output != "post" {
		t.Fatalf("test output = %q err=%v", output, err)
	}

	if err := store.PurgeWritingTestPost(ctx, "alice", "post-a"); err != nil {
		t.Fatal(err)
	}
	if held, err := store.ListTestPhotos(ctx, "alice", found.ID); err != nil || len(held) != 0 {
		t.Fatalf("purge left %v err=%v", photoIDs(held), err)
	}
	if _, err := store.GetTestPhoto(ctx, "alice", "held-1"); !errors.Is(err, experiment.ErrTestPhotoNotFound) {
		t.Fatalf("a purged photo still reads: %v", err)
	}
	if _, err := store.GetTestPhoto(ctx, "alice", "waiting"); err != nil {
		t.Fatalf("an unattached upload went with the purge: %v", err)
	}
	if err := handle.Reader.QueryRow(`SELECT output FROM writing_tests WHERE id=?`, found.ID).Scan(&output); err != nil || output != "post" {
		t.Fatalf("the purged test row changed: %q err=%v", output, err)
	}
}

// Deleting the account removes its test photos, attached or not, and no one else's.
func TestAccountDeletionCascadesTestPhotos(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	future := time.Now().UTC().Add(time.Hour)
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := handle.Writer.Exec(statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('carol','hash','2026-11-17T00:00:00Z')`)
	exec(`INSERT INTO writing_tests(id,user_id,operation_key,fingerprint,kind,factor,model_stage,count,status,context,common_hash,prompt_version,created_at,updated_at) VALUES('carol-test','carol','request','fp','ab','model','observe',2,'queued','{}','hash','v1','now','now')`)
	readyTestPhoto(t, store, "carol-held", "carol", future)
	readyTestPhoto(t, store, "carol-waiting", "carol", future)
	readyTestPhoto(t, store, "alice-waiting", "alice", future)
	if err := experimentstore.AttachTestPhotosTx(ctx, handle.Writer, "carol", "carol-test", []string{"carol-held"}); err != nil {
		t.Fatal(err)
	}
	exec(`DELETE FROM users WHERE id='carol'`)
	var carol, others int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM writing_test_photos WHERE user_id='carol'`).Scan(&carol); err != nil || carol != 0 {
		t.Fatalf("carol keeps %d photos err=%v", carol, err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM writing_test_photos WHERE user_id='alice'`).Scan(&others); err != nil || others != 1 {
		t.Fatalf("alice keeps %d photos err=%v", others, err)
	}
}
