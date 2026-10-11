package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
)

// confirmPhoto runs the production confirm: the reservation, then the guarded insert.
func confirmPhoto(t *testing.T, s *store.Store, slug, id string, at time.Time) {
	t.Helper()
	ctx := context.Background()
	key := post.ObjectKey(slug, id)
	if err := s.CreateUpload(ctx, post.Upload{ID: id, PostSlug: slug, Filename: id + ".jpg", Key: key, Kind: post.AttachmentPhoto, ContentType: "image/jpeg", CreatedAt: at, ExpiresAt: at.Add(time.Hour)}); err != nil {
		t.Fatalf("CreateUpload(%s): %v", id, err)
	}
	if err := s.ConfirmUpload(ctx, post.Image{ID: id, PostSlug: slug, Filename: id + ".jpg", Key: key, Width: 1, Height: 1, Bytes: 1, CreatedAt: at}, id); err != nil {
		t.Fatalf("ConfirmUpload(%s): %v", id, err)
	}
}

func listedOrder(t *testing.T, s *store.Store, slug string) []string {
	t.Helper()
	images, err := s.ListImages(context.Background(), slug)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(images))
	for i, image := range images {
		ids[i] = image.ID
	}
	return ids
}

func storedPositions(t *testing.T, handle *db.DB, slug string) map[string]int {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT id, position FROM images WHERE post_slug = ?`, slug)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	positions := map[string]int{}
	for rows.Next() {
		var id string
		var position int
		if err := rows.Scan(&id, &position); err != nil {
			t.Fatal(err)
		}
		positions[id] = position
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return positions
}

// POST-116: a confirmed photo joins after its post's last photo whatever its timestamp, the
// first photo of a post takes 0 and another post counts apart, and a photo after a delete
// still lands last.
func TestConfirmAppendsAfterThePostsLastPhoto(t *testing.T) {
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	seedPost(t, s, "q", "alice", testNow)
	// Confirm order, not the clock: the later confirms carry earlier timestamps.
	confirmPhoto(t, s, "p", "c", testNow.Add(2*time.Minute))
	confirmPhoto(t, s, "p", "a", testNow.Add(time.Minute))
	confirmPhoto(t, s, "p", "b", testNow)
	confirmPhoto(t, s, "q", "z", testNow)
	if got, want := listedOrder(t, s, "p"), []string{"c", "a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if got, want := storedPositions(t, handle, "p"), map[string]int{"c": 0, "a": 1, "b": 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	if got, want := storedPositions(t, handle, "q"), map[string]int{"z": 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("another post's positions = %v, want %v", got, want)
	}

	if deleted, err := s.DeleteImage(context.Background(), "b"); err != nil || !deleted {
		t.Fatalf("DeleteImage = %v, %v", deleted, err)
	}
	if deleted, err := s.DeleteImage(context.Background(), "c"); err != nil || !deleted {
		t.Fatalf("DeleteImage = %v, %v", deleted, err)
	}
	confirmPhoto(t, s, "p", "d", testNow)
	if got, want := listedOrder(t, s, "p"), []string{"a", "d"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order after a delete = %v, want %v", got, want)
	}
	if got := storedPositions(t, handle, "p")["d"]; got != 2 {
		t.Fatalf("the photo after a delete took %d, want 2 (after the last one)", got)
	}
}

// POST-116: a reorder writes the positions 0..n-1 and advances the input revision in one
// transaction, and changes nothing else; an unchanged order writes nothing; a list that is not
// exactly the post's photos is refused with nothing changed; another post keeps its order.
func TestReorderImagesWritesTheOrderInOneTransaction(t *testing.T) {
	ctx := context.Background()
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)
	seedPost(t, s, "q", "alice", testNow)
	for _, id := range []string{"a", "b", "c"} {
		confirmPhoto(t, s, "p", id, testNow)
	}
	confirmPhoto(t, s, "q", "y", testNow)
	confirmPhoto(t, s, "q", "z", testNow)
	if err := s.CreateUpload(ctx, post.Upload{ID: "v1", PostSlug: "p", Filename: "clip.mp4", Key: post.VideoObjectKey("p", "v1", "mp4"), Kind: post.AttachmentVideo, ContentType: "video/mp4", CreatedAt: testNow, ExpiresAt: testNow.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmVideoUpload(ctx, testVideo("v1", "clip.mp4"), "v1"); err != nil {
		t.Fatal(err)
	}
	// A gap a delete left behind is rewritten dense by the next reorder.
	if _, err := handle.Writer.Exec(`UPDATE images SET position = 7 WHERE id = 'c'`); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}

	moved, err := s.ReorderImages(ctx, "p", []string{"c", "a", "b"}, testNow.Add(time.Hour))
	if err != nil || !moved {
		t.Fatalf("ReorderImages = %v, %v", moved, err)
	}
	if got, want := listedOrder(t, s, "p"), []string{"c", "a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if got, want := storedPositions(t, handle, "p"), map[string]int{"c": 0, "a": 1, "b": 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	after, err := s.GetPost(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if after.InputRevision != before.InputRevision+1 || !after.UpdatedAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("input revision %d at %v, want %d at %v", after.InputRevision, after.UpdatedAt, before.InputRevision+1, testNow.Add(time.Hour))
	}
	after.InputRevision, after.UpdatedAt = before.InputRevision, before.UpdatedAt
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("a reorder changed more than the order:\nbefore %+v\nafter  %+v", before, after)
	}
	if got, want := listedOrder(t, s, "q"), []string{"y", "z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("another post's order = %v, want %v", got, want)
	}

	// The same order again writes nothing.
	settled, _ := s.GetPost(ctx, "p")
	if moved, err := s.ReorderImages(ctx, "p", []string{"c", "a", "b"}, testNow.Add(2*time.Hour)); err != nil || moved {
		t.Fatalf("unchanged order = %v, %v", moved, err)
	}
	if again, _ := s.GetPost(ctx, "p"); !reflect.DeepEqual(again, settled) {
		t.Fatalf("an unchanged order wrote the post: %+v", again)
	}

	for name, ids := range map[string][]string{
		"a photo left out":     {"a", "b"},
		"an unknown photo":     {"a", "b", "c", "x"},
		"a video":              {"a", "b", "v1"},
		"a photo twice":        {"a", "a", "b"},
		"another post's photo": {"a", "b", "y"},
	} {
		if moved, err := s.ReorderImages(ctx, "p", ids, testNow.Add(3*time.Hour)); !errors.Is(err, post.ErrPhotoOrderStale) || moved {
			t.Errorf("%s: %v, %v; want post.ErrPhotoOrderStale", name, moved, err)
		}
	}
	if got, want := storedPositions(t, handle, "p"), map[string]int{"c": 0, "a": 1, "b": 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("a refused reorder moved photos: %v", got)
	}
	if again, _ := s.GetPost(ctx, "p"); !reflect.DeepEqual(again, settled) {
		t.Fatalf("a refused reorder wrote the post: %+v", again)
	}
}
