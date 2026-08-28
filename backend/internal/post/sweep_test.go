package post

import (
	"context"
	"testing"
	"time"
)

const testMinAge = time.Hour

func newTestSweeper(store Store, blobs ObjectStore) *Sweeper {
	s := NewSweeper(store, blobs, testMinAge)
	s.now = func() time.Time { return testNow }
	return s
}

// TestSweepExpiredUploads is job 03 A4: an upload whose confirm never arrived is
// removed with its object.
func TestSweepExpiredUploads(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	blobs := newFakeBlobs()

	// Long past its window and past the grace period.
	dead := Upload{ID: "dead", PostSlug: "p", Filename: "a.jpg", Key: "posts/p/dead.jpg",
		ExpiresAt: testNow.Add(-3 * time.Hour), CreatedAt: testNow.Add(-4 * time.Hour)}
	// Expired, but inside the grace period — a client that PUT just before the URL died
	// may still be sending its confirm.
	recent := Upload{ID: "recent", PostSlug: "p", Filename: "b.jpg", Key: "posts/p/recent.jpg",
		ExpiresAt: testNow.Add(-time.Minute), CreatedAt: testNow.Add(-11 * time.Minute)}
	// Still valid.
	live := Upload{ID: "live", PostSlug: "p", Filename: "c.jpg", Key: "posts/p/live.jpg",
		ExpiresAt: testNow.Add(5 * time.Minute), CreatedAt: testNow}

	for _, u := range []Upload{dead, recent, live} {
		if err := store.CreateUpload(ctx, u); err != nil {
			t.Fatalf("CreateUpload: %v", err)
		}
		blobs.put(u.Key, 100, u.CreatedAt)
	}

	newTestSweeper(store, blobs).SweepOnce(ctx)

	if _, err := store.GetUpload(ctx, "dead"); err == nil {
		t.Error("the expired upload row survived")
	}
	if blobs.has(dead.Key) {
		t.Error("the expired upload's object survived")
	}
	for _, id := range []string{"recent", "live"} {
		if _, err := store.GetUpload(ctx, id); err != nil {
			t.Errorf("upload %q was swept too early: %v", id, err)
		}
	}
}

// A storage failure must leave the row alone: the row is the only record of the key.
func TestSweepKeepsTheRowWhenStorageFails(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	blobs := newFakeBlobs()
	blobs.failDelete = true

	upload := Upload{ID: "dead", PostSlug: "p", Filename: "a.jpg", Key: "posts/p/dead.jpg",
		ExpiresAt: testNow.Add(-3 * time.Hour), CreatedAt: testNow.Add(-4 * time.Hour)}
	if err := store.CreateUpload(ctx, upload); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	newTestSweeper(store, blobs).SweepOnce(ctx)

	if _, err := store.GetUpload(ctx, "dead"); err != nil {
		t.Errorf("the row was dropped while its object is still there: %v", err)
	}
}

// TestSweepStrayObjects covers the leak no row can describe: bytes in the bucket that
// nothing points at.
func TestSweepStrayObjects(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	blobs := newFakeBlobs()

	// Referenced by an image row.
	if err := store.CreateImage(ctx, Image{ID: "kept", PostSlug: "p", Filename: "a.jpg",
		Key: "posts/p/kept.jpg", CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	blobs.put("posts/p/kept.jpg", 100, testNow.Add(-24*time.Hour))

	// Referenced by an in-flight upload.
	if err := store.CreateUpload(ctx, Upload{ID: "pending", PostSlug: "p", Filename: "b.jpg",
		Key: "posts/p/pending.jpg", ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	blobs.put("posts/p/pending.jpg", 100, testNow.Add(-24*time.Hour))

	// Referenced by nothing, and old enough to be certain.
	blobs.put("posts/p/stray.jpg", 100, testNow.Add(-24*time.Hour))
	// Referenced by nothing, but fresh — this is what an upload in flight looks like
	// before its row is written, so deleting it would race the user's own PUT.
	blobs.put("posts/p/fresh.jpg", 100, testNow.Add(-time.Minute))

	newTestSweeper(store, blobs).SweepOnce(ctx)

	if blobs.has("posts/p/stray.jpg") {
		t.Error("the stray object survived")
	}
	for _, key := range []string{"posts/p/kept.jpg", "posts/p/pending.jpg", "posts/p/fresh.jpg"} {
		if !blobs.has(key) {
			t.Errorf("%s was deleted but should have been kept", key)
		}
	}
}

// If the listing fails, the sweep must delete NOTHING. A short listing would look like
// "these objects are gone" and take live photos with it.
func TestSweepDeletesNothingWhenTheListingFails(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	blobs := newFakeBlobs()
	blobs.put("posts/p/stray.jpg", 100, testNow.Add(-24*time.Hour))
	blobs.failList = true

	newTestSweeper(store, blobs).SweepOnce(ctx)

	if !blobs.has("posts/p/stray.jpg") {
		t.Error("an object was deleted on the strength of a failed listing")
	}
}

func TestSweepRunStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sweeper := newTestSweeper(newFakeStore(), newFakeBlobs())

	done := make(chan struct{})
	go func() {
		sweeper.Run(ctx, time.Hour)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

// TestSweepNeverDeletesAConfirmedPhotosObject is the regression for the worst bug the
// review found: a confirm whose upload row outlived the move (a crash, a cancelled
// context) leaves a row naming the LIVE photo's object. Deleting it destroys the bytes
// while the photo row goes on looking healthy — silent and permanent.
func TestSweepNeverDeletesAConfirmedPhotosObject(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	blobs := newFakeBlobs()

	key := ObjectKey("p", "u1")
	if err := store.CreateImage(ctx, Image{ID: "u1", PostSlug: "p", Filename: "a.jpg",
		Key: key, Width: 1, Height: 1, Bytes: 1, CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	// The row that should have gone with the confirm, now long expired.
	if err := store.CreateUpload(ctx, Upload{ID: "u1", PostSlug: "p", Filename: "a.jpg",
		Key: key, ExpiresAt: testNow.Add(-3 * time.Hour), CreatedAt: testNow.Add(-4 * time.Hour)}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	blobs.put(key, 100, testNow.Add(-4*time.Hour))

	newTestSweeper(store, blobs).SweepOnce(ctx)

	if !blobs.has(key) {
		t.Fatal("a confirmed photo's object was deleted")
	}
	// The stale row itself should go — it is the thing that was wrong.
	if _, err := store.GetUpload(ctx, "u1"); err == nil {
		t.Error("the stale upload row survived")
	}
}
