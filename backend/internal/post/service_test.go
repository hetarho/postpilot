package post

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	alice = "alice"
	bob   = "bob"
)

var testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

const testMaxBytes int64 = 1 << 20

// newTestService returns a service with a frozen clock and predictable ids.
func newTestService(t *testing.T) (*Service, *fakeStore, *fakeBlobs) {
	t.Helper()

	store := newFakeStore()
	blobs := newFakeBlobs()
	svc := NewService(store, blobs, 10*time.Minute, 5*time.Minute, testMaxBytes)
	svc.now = func() time.Time { return testNow }

	n := 0
	svc.newID = func() string {
		n++
		return fmt.Sprintf("img%d", n)
	}

	return svc, store, blobs
}

func mustCreatePost(t *testing.T, svc *Service, userID, title string) Post {
	t.Helper()
	created, err := svc.SaveDraft(context.Background(), userID, "", title, "")
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	return created
}

// --- drafts ---

// TestSaveDraftCreatesThenUpdates is job 03 A9: the first save mints the slug, later
// saves with that slug are idempotent updates.
func TestSaveDraftCreatesThenUpdates(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	created, err := svc.SaveDraft(ctx, alice, "", "Jeju", "first")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Slug != "20260301-jeju" {
		t.Errorf("slug = %q", created.Slug)
	}
	if created.Status != StatusDraft {
		t.Errorf("status = %q, want draft", created.Status)
	}

	updated, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "second")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Slug != created.Slug {
		t.Errorf("slug changed on update: %q → %q", created.Slug, updated.Slug)
	}
	if updated.Memo != "second" {
		t.Errorf("memo = %q, want second", updated.Memo)
	}
}

// TestSaveDraftKeepsTheSlugOnRetitle is the second half of plan 02 AC10. The slug is
// the primary key AND part of every object key, so renaming it would orphan the photos.
func TestSaveDraftKeepsTheSlugOnRetitle(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Jeju")

	renamed, err := svc.SaveDraft(context.Background(), alice, created.Slug, "Something else entirely", "")
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if renamed.Slug != created.Slug {
		t.Errorf("slug = %q, want it unchanged at %q", renamed.Slug, created.Slug)
	}
	if renamed.Title != "Something else entirely" {
		t.Errorf("title = %q", renamed.Title)
	}
}

// TestSaveDraftCollidesOnTitleAndDay is plan 02 AC10's first half, end to end.
func TestSaveDraftCollidesOnTitleAndDay(t *testing.T) {
	svc, _, _ := newTestService(t)

	first := mustCreatePost(t, svc, alice, "성산")
	second := mustCreatePost(t, svc, alice, "성산")

	if first.Slug != "20260301-성산" {
		t.Errorf("first = %q", first.Slug)
	}
	if second.Slug != "20260301-성산-2" {
		t.Errorf("second = %q, want 20260301-성산-2", second.Slug)
	}
}

// A slug taken by ANOTHER user still collides — slugs are globally unique because they
// are the primary key. The second user must not be handed the first user's post.
func TestSaveDraftCollidesAcrossUsers(t *testing.T) {
	svc, _, _ := newTestService(t)

	mine := mustCreatePost(t, svc, alice, "성산")
	theirs := mustCreatePost(t, svc, bob, "성산")

	if theirs.Slug == mine.Slug {
		t.Fatal("two users share a slug")
	}
	if theirs.UserID != bob {
		t.Errorf("owner = %q, want bob", theirs.UserID)
	}
}

// --- ownership ---

// TestOwnership is job 03 A6 / plan 02 AC9.
func TestOwnership(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	mine := mustCreatePost(t, svc, alice, "Mine")

	t.Run("foreign post is forbidden, not hidden", func(t *testing.T) {
		// PRD §7 specifies 403 here. At two users there is nothing to enumerate.
		if _, err := svc.Get(ctx, bob, mine.Slug); !errors.Is(err, ErrForbidden) {
			t.Errorf("Get = %v, want ErrForbidden", err)
		}
		if _, err := svc.SaveDraft(ctx, bob, mine.Slug, "x", "y"); !errors.Is(err, ErrForbidden) {
			t.Errorf("SaveDraft = %v, want ErrForbidden", err)
		}
		if _, _, _, err := svc.CreateUpload(ctx, bob, mine.Slug, "a.jpg"); !errors.Is(err, ErrForbidden) {
			t.Errorf("CreateUpload = %v, want ErrForbidden", err)
		}
	})

	t.Run("unknown slug is not found", func(t *testing.T) {
		if _, err := svc.Get(ctx, alice, "nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get = %v, want ErrNotFound", err)
		}
		if _, err := svc.SaveDraft(ctx, alice, "nope", "x", "y"); !errors.Is(err, ErrNotFound) {
			t.Errorf("SaveDraft = %v, want ErrNotFound", err)
		}
	})
}

func TestListIsScopedToTheCaller(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	mustCreatePost(t, svc, alice, "Mine one")
	mustCreatePost(t, svc, alice, "Mine two")
	mustCreatePost(t, svc, bob, "Theirs")

	mine, err := svc.List(ctx, alice)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("got %d posts, want 2", len(mine))
	}
	for _, s := range mine {
		if strings.Contains(s.Title, "Theirs") {
			t.Errorf("another user's post leaked into the list: %+v", s)
		}
	}
}

type fakeActiveJobs map[string]*ActiveJob

func (f fakeActiveJobs) ActiveForPost(_ context.Context, slug string) (*ActiveJob, error) {
	return f[slug], nil
}

func TestGetAndListPublishTheActiveJobThroughThePort(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Generating")
	svc.jobs = fakeActiveJobs{
		created.Slug: {ID: "job-1", Status: "running", Stage: "observe", ProgressDone: 2, ProgressTotal: 4},
	}

	found, err := svc.Get(context.Background(), alice, created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if found.ActiveJob == nil || found.ActiveJob.ID != "job-1" {
		t.Fatalf("Get active job = %+v", found.ActiveJob)
	}
	listed, err := svc.List(context.Background(), alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ActiveJob == nil || listed[0].ActiveJob.ID != "job-1" {
		t.Fatalf("List active job = %+v", listed)
	}
}

// --- uploads ---

// TestUploadHandshake is job 03 A1 + A2: no bytes touch the API, and the image row
// appears only after storage confirms the object.
func TestUploadHandshake(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, putURL, contentType, err := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if want := ObjectKey(p.Slug, upload.ID); upload.Key != want {
		t.Errorf("key = %q, want %q", upload.Key, want)
	}
	if !strings.HasPrefix(putURL, "https://storage.example/") {
		t.Errorf("put url is not on the storage host: %q", putURL)
	}
	if contentType != "image/jpeg" {
		t.Errorf("content type = %q, want image/jpeg", contentType)
	}
	if want := testNow.Add(10 * time.Minute); !upload.ExpiresAt.Equal(want) {
		t.Errorf("expires_at = %v, want %v", upload.ExpiresAt, want)
	}

	// Nothing is a photo yet.
	if images, _ := store.ListImages(ctx, p.Slug); len(images) != 0 {
		t.Fatalf("an image row appeared before the object did: %+v", images)
	}

	// The browser PUTs; only then does confirm succeed.
	blobs.put(upload.Key, 204_800, testNow)

	image, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1024, 768)
	if err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if image.ID != upload.ID {
		t.Errorf("image id = %q, want the upload id %q", image.ID, upload.ID)
	}
	if image.Filename != "IMG_1.jpg" || image.Key != upload.Key {
		t.Errorf("image = %+v", image)
	}
	if image.Width != 1024 || image.Height != 768 {
		t.Errorf("dimensions = %dx%d, want 1024x768", image.Width, image.Height)
	}
	// bytes comes from storage, not the client — it is the one thing the server can check.
	if image.Bytes != 204_800 {
		t.Errorf("bytes = %d, want 204800 (from the HEAD)", image.Bytes)
	}

	// The uploads row has done its job.
	if _, err := store.GetUpload(ctx, upload.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("upload row survived confirm: %v", err)
	}
}

// TestConfirmWithoutTheObject is job 03 A1's negative: a confirm for an object that
// never landed must not create a photo.
func TestConfirmWithoutTheObject(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1024, 768); !errors.Is(err, ErrObjectMissing) {
		t.Fatalf("ConfirmUpload = %v, want ErrObjectMissing", err)
	}
	if images, _ := store.ListImages(ctx, p.Slug); len(images) != 0 {
		t.Errorf("an image row was created without an object: %+v", images)
	}
	// The row stays so the client can retry the PUT with the same id; the sweep
	// collects it if the client gives up.
	if _, err := store.GetUpload(ctx, upload.ID); err != nil {
		t.Errorf("upload row was dropped, so a retry is impossible: %v", err)
	}
}

func TestConfirmSomeoneElsesUpload(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	blobs.put(upload.Key, 100, testNow)

	if _, err := svc.ConfirmUpload(ctx, bob, upload.ID, 1, 1); !errors.Is(err, ErrForbidden) {
		t.Errorf("ConfirmUpload = %v, want ErrForbidden", err)
	}
}

// TestDuplicateFilename is job 03 A8. The filename is how the model and the exporters
// refer to a photo, so two photos cannot share one within a post.
func TestDuplicateFilename(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	first, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	// Still only in flight: that is the retry case, so the pending upload is REPLACED
	// rather than refused — every photo has a retry button that restarts from here.
	blobs.put(first.Key, 100, testNow)
	retry, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	if err != nil {
		t.Fatalf("retry CreateUpload: %v", err)
	}
	if retry.ID == first.ID {
		t.Error("the retry reused the abandoned upload id")
	}
	if blobs.has(first.Key) {
		t.Error("the replaced upload's object was left behind")
	}

	first = retry
	blobs.put(first.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, first.ID, 1, 1); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if _, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg"); !errors.Is(err, ErrDuplicateFilename) {
		t.Errorf("after confirm = %v, want ErrDuplicateFilename", err)
	}

	// The same name under a DIFFERENT post is fine.
	other := mustCreatePost(t, svc, alice, "Busan")
	if _, _, _, err := svc.CreateUpload(ctx, alice, other.Slug, "IMG_1.jpg"); err != nil {
		t.Errorf("same filename on another post: %v", err)
	}
}

// --- reading ---

// TestGetMintsFreshViewURLs is job 03 A5's server half: a URL per read, never stored.
func TestGetMintsFreshViewURLs(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1, 1); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	got, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Images) != 1 {
		t.Fatalf("got %d images, want 1", len(got.Images))
	}
	if !strings.Contains(got.Images[0].ViewURL, "get") {
		t.Errorf("view url = %q, want a presigned GET", got.Images[0].ViewURL)
	}

	// The stored row must not carry a URL — a persisted one is either long-lived (a
	// durable capability to a private object) or already expired.
	stored, err := store.GetImage(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if stored.ViewURL != "" {
		t.Errorf("a view url was persisted: %q", stored.ViewURL)
	}
}

// --- deletion ---

// TestDeleteImage is job 03 A3: the row AND the object go.
func TestDeleteImage(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1, 1); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	if err := svc.DeleteImage(ctx, alice, upload.ID); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if blobs.has(upload.Key) {
		t.Error("the object survived the delete")
	}
	if _, err := store.GetImage(ctx, upload.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the image row survived the delete: %v", err)
	}
}

func TestDeleteImageOwnership(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1, 1); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	if err := svc.DeleteImage(ctx, bob, upload.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("DeleteImage = %v, want ErrForbidden", err)
	}
	if !blobs.has(upload.Key) {
		t.Error("another user's photo was deleted from storage")
	}
	if err := svc.DeleteImage(ctx, alice, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown image = %v, want ErrNotFound", err)
	}
}

// A failed storage delete must keep the row: the row is the only record of the key, so
// dropping it would turn a retryable failure into bytes nobody can name.
func TestDeleteImageKeepsTheRowWhenStorageFails(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1, 1); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}

	blobs.failDelete = true
	if err := svc.DeleteImage(ctx, alice, upload.ID); err == nil {
		t.Fatal("DeleteImage reported success while storage failed")
	}
	if _, err := store.GetImage(ctx, upload.ID); err != nil {
		t.Errorf("the row was dropped even though the object is still there: %v", err)
	}
}

type recordingContentPurger struct {
	calls []string
	err   error
}

func (p *recordingContentPurger) PurgePost(_ context.Context, userID, slug string) error {
	p.calls = append(p.calls, userID+"/"+slug)
	return p.err
}

func TestDeletePostPurgesExperimentContentBeforeRemovingSource(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	found := mustCreatePost(t, svc, alice, "Jeju")
	upload, _, _, _ := svc.CreateUpload(ctx, alice, found.Slug, "IMG_1.jpg")
	blobs.put(upload.Key, 100, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1, 1); err != nil {
		t.Fatal(err)
	}
	purger := &recordingContentPurger{}
	svc.SetExperimentContentPurger(purger)

	if err := svc.DeletePost(ctx, alice, found.Slug); err != nil {
		t.Fatal(err)
	}
	if len(purger.calls) != 1 || purger.calls[0] != alice+"/"+found.Slug {
		t.Fatalf("purge calls = %v", purger.calls)
	}
	if _, err := store.GetPost(ctx, found.Slug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("post survived delete: %v", err)
	}
	if blobs.has(upload.Key) {
		t.Fatal("post image survived delete")
	}
}

func TestDeletePostStopsBeforeDeleteWhenExperimentPurgeFails(t *testing.T) {
	svc, store, _ := newTestService(t)
	found := mustCreatePost(t, svc, alice, "Jeju")
	svc.SetExperimentContentPurger(&recordingContentPurger{err: errors.New("database unavailable")})

	if err := svc.DeletePost(context.Background(), alice, found.Slug); err == nil {
		t.Fatal("delete succeeded without purging experiment content")
	}
	if _, err := store.GetPost(context.Background(), found.Slug); err != nil {
		t.Fatalf("post was deleted after purge failure: %v", err)
	}
}

func TestWinnerApplicationIsIdempotentAtPostBoundary(t *testing.T) {
	svc, store, _ := newTestService(t)
	found := mustCreatePost(t, svc, alice, "Jeju")
	content := PostContent{Title: "generated", Blocks: []Block{{Type: BlockText, Content: "body"}}}
	if err := svc.SetGeneratedContent(context.Background(), alice, found.Slug, content); err != nil {
		t.Fatal(err)
	}
	first, _ := store.GetPost(context.Background(), found.Slug)
	svc.now = func() time.Time { return testNow.Add(time.Hour) }
	if err := svc.SetGeneratedContent(context.Background(), alice, found.Slug, content); err != nil {
		t.Fatal(err)
	}
	second, _ := store.GetPost(context.Background(), found.Slug)
	if !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("idempotent apply changed updated_at: %v -> %v", first.UpdatedAt, second.UpdatedAt)
	}
}

// --- regressions ---

// A confirm the client never saw the answer to must return the photo, not a
// primary-key failure.
func TestConfirmUploadIsIdempotent(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	blobs.put(upload.Key, 100, testNow)

	first, err := svc.ConfirmUpload(ctx, alice, upload.ID, 10, 10)
	if err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	second, err := svc.ConfirmUpload(ctx, alice, upload.ID, 10, 10)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if second.ID != first.ID || second.Filename != first.Filename {
		t.Errorf("retry returned a different photo: %+v vs %+v", second, first)
	}

	// A genuinely unknown id is still not found, and another user's photo is not
	// reachable through the retry path.
	if _, err := svc.ConfirmUpload(ctx, alice, "never-existed", 10, 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id = %v, want ErrNotFound", err)
	}
	if _, err := svc.ConfirmUpload(ctx, bob, upload.ID, 10, 10); !errors.Is(err, ErrForbidden) {
		t.Errorf("foreign retry = %v, want ErrForbidden", err)
	}
}

// The browser's size cap is advice; a presigned PUT is a URL an authenticated client
// can use however it likes, so the limit is enforced where it can be trusted.
func TestConfirmUploadRejectsImplausibleObjects(t *testing.T) {
	ctx := context.Background()

	cases := map[string]int64{
		"empty":   0,
		"too big": testMaxBytes + 1,
	}
	for name, size := range cases {
		t.Run(name, func(t *testing.T) {
			svc, store, blobs := newTestService(t)
			p := mustCreatePost(t, svc, alice, "Jeju")
			upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
			blobs.put(upload.Key, size, testNow)

			if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 10, 10); !errors.Is(err, ErrInvalidImage) {
				t.Fatalf("err = %v, want ErrInvalidImage", err)
			}
			// The object is dropped rather than left for the sweep, which would keep
			// paying for it for an hour.
			if blobs.has(upload.Key) {
				t.Error("the rejected object was left in storage")
			}
			if images, _ := store.ListImages(ctx, p.Slug); len(images) != 0 {
				t.Errorf("a photo row was created: %+v", images)
			}
		})
	}
}

func TestConfirmUploadRejectsImplausibleDimensions(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")
	upload, _, _, _ := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg")
	blobs.put(upload.Key, 100, testNow)

	for _, wh := range [][2]int32{{0, 10}, {10, 0}, {-1, 10}, {10, -1}, {maxImageDimension + 1, 10}} {
		if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, wh[0], wh[1]); !errors.Is(err, ErrInvalidImage) {
			t.Errorf("%dx%d = %v, want ErrInvalidImage", wh[0], wh[1], err)
		}
	}
}

// Two first-saves of the same title on the same day can mint the same candidate. The
// loser must get the next serial, not an error.
func TestCreatePostRetriesWhenTheSlugIsTakenMidFlight(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()

	// Stand in for the race: the slug appears between the existence check and the
	// insert, exactly as a concurrent request would make it.
	original := store.slugTaken
	store.slugTaken = func(slug string) {
		if slug == "20260301-jeju" {
			_ = store.CreatePost(ctx, Post{Slug: slug, UserID: bob, Status: StatusDraft,
				CreatedAt: testNow, UpdatedAt: testNow})
			store.slugTaken = original
		}
	}

	created, err := svc.SaveDraft(ctx, alice, "", "Jeju", "")
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if created.Slug != "20260301-jeju-2" {
		t.Errorf("slug = %q, want 20260301-jeju-2", created.Slug)
	}
	if created.UserID != alice {
		t.Errorf("owner = %q, want alice", created.UserID)
	}
}
