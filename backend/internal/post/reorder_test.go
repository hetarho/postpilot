package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// photoOrder is the post's photos as GetPost lists them.
func photoOrder(t *testing.T, svc *Service, slug string) []string {
	t.Helper()
	found, err := svc.Get(context.Background(), alice, slug)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(found.Images))
	for i, image := range found.Images {
		ids[i] = image.ID
	}
	return ids
}

func inputRevision(t *testing.T, store *fakeStore, slug string) int64 {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.posts[slug].InputRevision
}

// POST-116: photos stand in the order they were confirmed until the owner moves one; a reorder
// is the order every read lists from then on, it advances the input revision and nothing else,
// and the next confirmed photo joins at the end.
func TestReorderImagesSavesTheOwnersOrder(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "제주 3일")
	first := attachPhoto(t, svc, blobs, p.Slug, "IMG_1.jpg")
	second := attachPhoto(t, svc, blobs, p.Slug, "IMG_2.jpg")
	third := attachPhoto(t, svc, blobs, p.Slug, "IMG_3.jpg")
	mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 3000)
	if got, want := photoOrder(t, svc, p.Slug), []string{first.ID, second.ID, third.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("confirm order = %v, want %v", got, want)
	}
	before, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}

	moved := []string{third.ID, first.ID, second.ID}
	if err := svc.ReorderImages(ctx, alice, p.Slug, moved); err != nil {
		t.Fatalf("ReorderImages = %v", err)
	}
	if got := photoOrder(t, svc, p.Slug); !reflect.DeepEqual(got, moved) {
		t.Fatalf("order after the move = %v, want %v", got, moved)
	}
	after, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if after.InputRevision != before.InputRevision+1 {
		t.Fatalf("input revision = %d, want %d", after.InputRevision, before.InputRevision+1)
	}
	if after.Status != before.Status || after.ContentRevision != before.ContentRevision || after.MachineBaselineRevision != before.MachineBaselineRevision || len(after.Videos) != 1 {
		t.Fatalf("a reorder changed more than the order: before %+v after %+v", before, after)
	}

	// An unchanged order is a no-op success: nothing is written and no revision advances.
	if err := svc.ReorderImages(ctx, alice, p.Slug, moved); err != nil {
		t.Fatalf("unchanged order = %v", err)
	}
	if got := inputRevision(t, store, p.Slug); got != after.InputRevision {
		t.Fatalf("an unchanged order advanced the input revision to %d", got)
	}

	fourth := attachPhoto(t, svc, blobs, p.Slug, "IMG_4.jpg")
	if got, want := photoOrder(t, svc, p.Slug), append(moved, fourth.ID); !reflect.DeepEqual(got, want) {
		t.Fatalf("a new photo = %v, want it last in %v", got, want)
	}
}

// POST-116: the list must be exactly the post's photos, each once. A photo added or deleted
// since the client read the post, a video, a repeat or another post's photo is refused and
// nothing moves.
func TestReorderImagesRefusesAListThatIsNotThePostsPhotos(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "제주 3일")
	first := attachPhoto(t, svc, blobs, p.Slug, "IMG_1.jpg")
	second := attachPhoto(t, svc, blobs, p.Slug, "IMG_2.jpg")
	clip := mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 3000)
	other := mustCreatePost(t, svc, alice, "부산")
	foreign := attachPhoto(t, svc, blobs, other.Slug, "IMG_1.jpg")
	order := photoOrder(t, svc, p.Slug)
	revision := inputRevision(t, store, p.Slug)

	for name, ids := range map[string][]string{
		"a photo left out":           {second.ID},
		"an unknown photo":           {second.ID, first.ID, "deleted-photo"},
		"a video":                    {second.ID, first.ID, clip.ID},
		"a video in a photo's place": {second.ID, clip.ID},
		"a photo twice":              {second.ID, second.ID},
		"another post's photo":       {second.ID, foreign.ID},
		"nothing":                    nil,
	} {
		if err := svc.ReorderImages(ctx, alice, p.Slug, ids); !errors.Is(err, ErrPhotoOrderStale) {
			t.Errorf("%s: err = %v, want ErrPhotoOrderStale", name, err)
		}
	}
	if got := photoOrder(t, svc, p.Slug); !reflect.DeepEqual(got, order) {
		t.Fatalf("a refused reorder moved photos: %v, want %v", got, order)
	}
	if got := inputRevision(t, store, p.Slug); got != revision {
		t.Fatalf("a refused reorder advanced the input revision to %d", got)
	}
}

// The post is the caller's: another account and an unknown post are refused before anything
// is read, as every photo write is.
func TestReorderImagesIsTheOwnersOnly(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "제주 3일")
	first := attachPhoto(t, svc, blobs, p.Slug, "IMG_1.jpg")
	second := attachPhoto(t, svc, blobs, p.Slug, "IMG_2.jpg")
	if err := svc.ReorderImages(ctx, bob, p.Slug, []string{second.ID, first.ID}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another account = %v, want ErrForbidden", err)
	}
	if err := svc.ReorderImages(ctx, alice, "nope", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown post = %v, want ErrNotFound", err)
	}
	if got, want := photoOrder(t, svc, p.Slug), []string{first.ID, second.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("a refused reorder moved photos: %v", got)
	}
}

// POST-74: a reorder that passed the service's check and then lost the race to a publish is
// refused by the store's own guard, as the lock, and no photo moves.
func TestAReorderThatLosesTheRaceToAPublishIsRefusedAsLocked(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "제주 3일")
	first := attachPhoto(t, svc, blobs, created.Slug, "IMG_1.jpg")
	second := attachPhoto(t, svc, blobs, created.Slug, "IMG_2.jpg")
	content := PostContent{Title: "제주 3일 기록", Blocks: []Block{{Type: BlockText, Content: "협재 해변은 물빛이 맑았다."}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(ctx, alice, created.Slug, 1); err != nil {
		t.Fatal(err)
	}
	revision := inputRevision(t, store, created.Slug)
	store.beforeGuardedWrite = func(slug string) {
		store.mu.Lock()
		defer store.mu.Unlock()
		store.beforeGuardedWrite = nil
		published := store.posts[slug]
		published.Status, published.PublishedURL = StatusPublished, firstAddress
		store.posts[slug] = published
	}
	if err := svc.ReorderImages(ctx, alice, created.Slug, []string{second.ID, first.ID}); !errors.Is(err, ErrPostPublished) {
		t.Fatalf("err = %v, want ErrPostPublished", err)
	}
	if got, want := photoOrder(t, svc, created.Slug), []string{first.ID, second.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the losing reorder moved photos: %v", got)
	}
	if got := inputRevision(t, store, created.Slug); got != revision {
		t.Fatalf("the losing reorder advanced the input revision to %d", got)
	}
}
