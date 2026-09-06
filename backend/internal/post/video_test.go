package post

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// mustAttachVideo runs the whole handshake for one clip, the way the browser does.
func mustAttachVideo(t *testing.T, svc *Service, blobs *fakeBlobs, userID, slug, filename string, durationMs int64) Video {
	t.Helper()
	ctx := context.Background()

	upload, putURL, contentType, err := svc.CreateUpload(ctx, userID, slug, filename, AttachmentVideo)
	if err != nil {
		t.Fatalf("CreateUpload(%s): %v", filename, err)
	}
	if !strings.HasPrefix(putURL, "https://storage.example/") {
		t.Errorf("put url is not on the storage host: %q", putURL)
	}
	blobs.putTyped(upload.Key, 1_000_000, contentType, testNow)

	confirmed, err := svc.ConfirmUpload(ctx, userID, upload.ID, 1920, 1080, durationMs)
	if err != nil {
		t.Fatalf("ConfirmUpload(%s): %v", filename, err)
	}
	if confirmed.Kind != AttachmentVideo {
		t.Fatalf("kind = %q, want %q", confirmed.Kind, AttachmentVideo)
	}
	return confirmed.Video
}

// The video handshake is the photo's, with the container's own type in the signature and
// the key, and the duration the browser read recorded on the row (VIDEO-5, VIDEO-6).
func TestVideoUploadHandshake(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, contentType, err := svc.CreateUpload(ctx, alice, p.Slug, "clip.MOV", AttachmentVideo)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if contentType != "video/quicktime" {
		t.Errorf("content type = %q, want video/quicktime (the extension is case-insensitive)", contentType)
	}
	if want := VideoObjectKey(p.Slug, upload.ID, "mov"); upload.Key != want {
		t.Errorf("key = %q, want %q", upload.Key, want)
	}
	if upload.Kind != AttachmentVideo {
		t.Errorf("upload kind = %q, want %q", upload.Kind, AttachmentVideo)
	}
	// Nothing is a video yet.
	if videos, _ := store.ListVideos(ctx, p.Slug); len(videos) != 0 {
		t.Fatalf("a video row appeared before the object did: %+v", videos)
	}

	blobs.putTyped(upload.Key, 2_000_000, contentType, testNow)
	confirmed, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1920, 1080, 12_000)
	if err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	video := confirmed.Video
	if video.ID != upload.ID || video.Filename != "clip.MOV" || video.Key != upload.Key {
		t.Errorf("video = %+v", video)
	}
	if video.ContentType != "video/quicktime" || video.DurationMs != 12_000 {
		t.Errorf("content type/duration = %q/%d", video.ContentType, video.DurationMs)
	}
	// bytes comes from the HEAD, never from the client.
	if video.Bytes != 2_000_000 {
		t.Errorf("bytes = %d, want 2000000 (from the HEAD)", video.Bytes)
	}
	if video.Width != 1920 || video.Height != 1080 {
		t.Errorf("dimensions = %dx%d", video.Width, video.Height)
	}
	if _, err := store.GetUpload(ctx, upload.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("upload row survived confirm: %v", err)
	}

	// Get mints a fresh view URL for a video exactly as it does for a photo.
	found, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(found.Videos) != 1 || found.Videos[0].ViewURL == "" {
		t.Fatalf("videos = %+v", found.Videos)
	}
}

// A container outside the four accepted ones is refused before anything is reserved.
func TestCreateUploadRefusesAnUnsupportedContainer(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	for _, filename := range []string{"clip.avi", "clip.mkv", "clip", "clip."} {
		if _, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, filename, AttachmentVideo); !errors.Is(err, ErrUnsupportedVideo) {
			t.Errorf("CreateUpload(%q) = %v, want ErrUnsupportedVideo", filename, err)
		}
	}
	if uploads, _ := store.ListUploadsExpiredBefore(ctx, testNow.Add(time.Hour)); len(uploads) != 0 {
		t.Errorf("a refused container still reserved something: %+v", uploads)
	}
}

// The per-post ceiling counts CONFIRMED videos and is separate from the photo one.
func TestCreateUploadRefusesPastTheVideoCeiling(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	for _, filename := range []string{"a.mp4", "b.mp4", "c.mp4"} {
		mustAttachVideo(t, svc, blobs, alice, p.Slug, filename, 5_000)
	}
	if _, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "d.mp4", AttachmentVideo); !errors.Is(err, ErrTooManyVideos) {
		t.Fatalf("CreateUpload past the ceiling = %v, want ErrTooManyVideos", err)
	}
	// The photo ceiling is untouched by a post full of videos.
	if _, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "photo.jpg", AttachmentPhoto); err != nil {
		t.Fatalf("photo upload after the video ceiling: %v", err)
	}
}

// One filename namespace per post: a name held by a confirmed attachment of EITHER kind is
// a conflict, whichever kind asks for it (VIDEO-5).
func TestCreateUploadRefusesAFilenameHeldByTheOtherKind(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	mustAttachVideo(t, svc, blobs, alice, p.Slug, "shared.mp4", 5_000)
	if _, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "shared.mp4", AttachmentPhoto); !errors.Is(err, ErrDuplicateFilename) {
		t.Errorf("photo taking a video's name = %v, want ErrDuplicateFilename", err)
	}

	photo, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "shared.jpg", AttachmentPhoto)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	blobs.put(photo.Key, 1000, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, photo.ID, 10, 10, 0); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if _, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "shared.jpg", AttachmentVideo); !errors.Is(err, ErrUnsupportedVideo) {
		t.Errorf("a .jpg asked for as a video = %v, want ErrUnsupportedVideo", err)
	}
}

// A PENDING upload under this name is a retry, not a conflict — including one of the other
// kind, because the name is what is being retried.
func TestCreateUploadReplacesAPendingUploadOfEitherKind(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	first, _, contentType, err := svc.CreateUpload(ctx, alice, p.Slug, "clip.mp4", AttachmentVideo)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	blobs.putTyped(first.Key, 500, contentType, testNow)

	retry, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "clip.mp4", AttachmentVideo)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retry.ID == first.ID {
		t.Error("the retry reused the abandoned id")
	}
	if _, err := store.GetUpload(ctx, first.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the replaced upload row survived: %v", err)
	}
	if blobs.has(first.Key) {
		t.Error("the replaced upload's object survived")
	}
}

// The confirm's own refusals. Each one drops the object rather than leaving the sweep to
// pay for it for an hour.
func TestConfirmVideoRefusals(t *testing.T) {
	cases := map[string]struct {
		size        int64
		contentType string
		durationMs  int64
		want        error
	}{
		"zero duration":     {size: 1000, contentType: "video/mp4", durationMs: 0, want: ErrInvalidVideo},
		"past the ceiling":  {size: 1000, contentType: "video/mp4", durationMs: testMaxVideoDurationMs + 1, want: ErrInvalidVideo},
		"empty object":      {size: 0, contentType: "video/mp4", durationMs: 5_000, want: ErrInvalidVideo},
		"too big":           {size: testMaxVideoBytes + 1, contentType: "video/mp4", durationMs: 5_000, want: ErrInvalidVideo},
		"wrong stored type": {size: 1000, contentType: "image/jpeg", durationMs: 5_000, want: ErrInvalidVideo},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, store, blobs := newTestService(t)
			ctx := context.Background()
			p := mustCreatePost(t, svc, alice, "Jeju")

			upload, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "clip.mp4", AttachmentVideo)
			if err != nil {
				t.Fatalf("CreateUpload: %v", err)
			}
			blobs.putTyped(upload.Key, tc.size, tc.contentType, testNow)

			if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1920, 1080, tc.durationMs); !errors.Is(err, tc.want) {
				t.Fatalf("ConfirmUpload = %v, want %v", err, tc.want)
			}
			if videos, _ := store.ListVideos(ctx, p.Slug); len(videos) != 0 {
				t.Errorf("a refused confirm wrote a row: %+v", videos)
			}
			// A duration the server can judge without storage is refused before the HEAD,
			// so only the object-level refusals clean up.
			if tc.want == ErrInvalidVideo && tc.durationMs > 0 && tc.durationMs <= testMaxVideoDurationMs {
				if blobs.has(upload.Key) {
					t.Error("the refused object was left in storage")
				}
				if _, err := store.GetUpload(ctx, upload.ID); !errors.Is(err, ErrNotFound) {
					t.Errorf("the refused upload row survived: %v", err)
				}
			}
		})
	}
}

// A confirm whose object never landed keeps the row, so the client can retry the PUT.
func TestConfirmVideoWithoutTheObject(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "clip.mp4", AttachmentVideo)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if _, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1920, 1080, 5_000); !errors.Is(err, ErrObjectMissing) {
		t.Fatalf("ConfirmUpload = %v, want ErrObjectMissing", err)
	}
	if _, err := store.GetUpload(ctx, upload.ID); err != nil {
		t.Errorf("the upload row was dropped, so a retry is impossible: %v", err)
	}
}

// A retry whose upload row is already gone has to find the VIDEO, not report the id
// unknown — the row that knew which kind it was is the one the first confirm deleted.
func TestConfirmVideoIsIdempotent(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	upload, _, contentType, err := svc.CreateUpload(ctx, alice, p.Slug, "clip.mp4", AttachmentVideo)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	blobs.putTyped(upload.Key, 1000, contentType, testNow)

	first, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1920, 1080, 5_000)
	if err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	second, err := svc.ConfirmUpload(ctx, alice, upload.ID, 1920, 1080, 5_000)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if second.Kind != AttachmentVideo || second.Video.ID != first.Video.ID {
		t.Errorf("retry returned %+v, want the same video as %+v", second, first)
	}
	// Another user cannot reach it through the retry path either.
	if _, err := svc.ConfirmUpload(ctx, bob, upload.ID, 1920, 1080, 5_000); !errors.Is(err, ErrForbidden) {
		t.Errorf("foreign retry = %v, want ErrForbidden", err)
	}
}

// DeleteVideo removes the object BEFORE the row, and takes the filename's observation
// entry with it (VIDEO-12) — a leftover entry would become eyesight for whatever is
// uploaded under that name next.
func TestDeleteVideoDropsTheObjectThenTheRowAndItsObservation(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	video := mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 5_000)
	if err := svc.SetObservations(ctx, alice, p.Slug, []Observation{
		{File: "clip.mp4", Scene: "바다", Events: []string{"파도가 친다"}, Speech: "좋다"},
		{File: "keep.jpg", Scene: "숙소"},
	}); err != nil {
		t.Fatalf("SetObservations: %v", err)
	}

	if err := svc.DeleteVideo(ctx, alice, video.ID); err != nil {
		t.Fatalf("DeleteVideo: %v", err)
	}
	if blobs.has(video.Key) {
		t.Error("the object survived the delete")
	}
	if len(blobs.deleted) == 0 || blobs.deleted[len(blobs.deleted)-1] != video.Key {
		t.Errorf("storage was not reached: %v", blobs.deleted)
	}
	if _, err := store.GetVideo(ctx, video.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the row survived: %v", err)
	}
	found, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(found.Observations) != 1 || found.Observations[0].File != "keep.jpg" {
		t.Errorf("observations = %+v, want only the photo's entry", found.Observations)
	}
}

// A foreign or unknown video is not deletable, and nothing is touched.
func TestDeleteVideoRefusesAForeignOrUnknownID(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")
	video := mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 5_000)

	if err := svc.DeleteVideo(ctx, bob, video.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("foreign delete = %v, want ErrForbidden", err)
	}
	if err := svc.DeleteVideo(ctx, alice, "never-existed"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown delete = %v, want ErrNotFound", err)
	}
	if !blobs.has(video.Key) {
		t.Error("a refused delete still reached storage")
	}
}

// Deleting the post takes every video's object with it, as it does every photo's.
func TestDeletePostRemovesVideoObjects(t *testing.T) {
	svc, store, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")
	svc.SetExperimentContentPurger(&recordingContentPurger{})

	video := mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 5_000)
	if err := svc.DeletePost(ctx, alice, p.Slug); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}
	if blobs.has(video.Key) {
		t.Error("the video's object outlived its post")
	}
	if videos, _ := store.ListVideos(ctx, p.Slug); len(videos) != 0 {
		t.Errorf("video rows outlived the post: %+v", videos)
	}
}

// The generation-facing snapshot carries videos beside photos, photos first, each
// attachment naming its kind, content type and duration.
func TestAttachedImagesCarriesVideosBesidePhotos(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")

	photo, _, _, err := svc.CreateUpload(ctx, alice, p.Slug, "IMG_1.jpg", AttachmentPhoto)
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	blobs.put(photo.Key, 1000, testNow)
	if _, err := svc.ConfirmUpload(ctx, alice, photo.ID, 10, 10, 0); err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 7_500)

	found, err := svc.AttachedImages(ctx, alice, p.Slug)
	if err != nil {
		t.Fatalf("AttachedImages: %v", err)
	}
	if len(found.Images) != 1 || found.Images[0].Filename != "IMG_1.jpg" {
		t.Errorf("images = %+v", found.Images)
	}
	if len(found.Videos) != 1 {
		t.Fatalf("videos = %+v", found.Videos)
	}
	clip := found.Videos[0]
	if clip.Filename != "clip.mp4" || clip.ContentType != "video/mp4" || clip.DurationMs != 7_500 || clip.Key == "" {
		t.Errorf("video = %+v", clip)
	}
	// No browser URLs here: the keys stay backend-only.
	if clip.ViewURL != "" {
		t.Errorf("AttachedImages presigned a view url: %q", clip.ViewURL)
	}
}

// A VIDEO block naming an attached clip saves; one naming nothing does not.
func TestSaveContentAcceptsVideoBlocksForAttachedClips(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Jeju")
	mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 5_000)

	content := PostContent{Title: "제목", Blocks: []Block{
		{Type: BlockText, Content: "문단"},
		{Type: BlockVideo, File: "clip.mp4", Caption: "파도"},
	}}
	saved, err := svc.SaveContent(ctx, alice, p.Slug, content, 0)
	if err != nil {
		t.Fatalf("SaveContent: %v", err)
	}
	if saved.Content == nil || len(saved.Content.Blocks) != 2 {
		t.Fatalf("content = %+v", saved.Content)
	}

	stray := PostContent{Title: "제목", Blocks: []Block{{Type: BlockVideo, File: "gone.mp4"}}}
	if _, err := svc.SaveContent(ctx, alice, p.Slug, stray, saved.ContentRevision); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("SaveContent with an unattached video = %v, want ErrInvalidContent", err)
	}
}
