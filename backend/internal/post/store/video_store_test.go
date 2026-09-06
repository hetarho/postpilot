package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
)

func testVideo(id, filename string) post.Video {
	return post.Video{
		ID: id, PostSlug: "p", Filename: filename,
		Key:         post.VideoObjectKey("p", id, "mp4"),
		ContentType: "video/mp4", Bytes: 12_345_678, DurationMs: 42_000,
		Width: 1920, Height: 1080, CreatedAt: testNow,
	}
}

func TestVideoRoundTripAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	video := testVideo("v1", "clip.mp4")
	if err := s.CreateVideo(ctx, video); err != nil {
		t.Fatalf("CreateVideo: %v", err)
	}

	got, err := s.GetVideo(ctx, "v1")
	if err != nil {
		t.Fatalf("GetVideo: %v", err)
	}
	if got.Filename != video.Filename || got.Key != video.Key || got.ContentType != "video/mp4" ||
		got.Bytes != video.Bytes || got.DurationMs != 42_000 || got.Width != 1920 || got.Height != 1080 ||
		!got.CreatedAt.Equal(testNow) {
		t.Errorf("video = %+v, want %+v", got, video)
	}

	count, err := s.CountVideos(ctx, "p")
	if err != nil || count != 1 {
		t.Errorf("CountVideos = %d, %v; want 1", count, err)
	}
	taken, err := s.VideoFilenameTaken(ctx, "p", "clip.mp4")
	if err != nil || !taken {
		t.Errorf("VideoFilenameTaken = %v, %v; want true", taken, err)
	}
	inUse, err := s.VideoKeyInUse(ctx, video.Key)
	if err != nil || !inUse {
		t.Errorf("VideoKeyInUse = %v, %v; want true", inUse, err)
	}

	if err := s.DeleteVideo(ctx, "v1"); err != nil {
		t.Fatalf("DeleteVideo: %v", err)
	}
	if _, err := s.GetVideo(ctx, "v1"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// One name per post, enforced by the schema for the video table exactly as for images.
func TestDuplicateVideoFilenameIsRefusedByTheSchema(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	if err := s.CreateVideo(ctx, testVideo("v1", "clip.mp4")); err != nil {
		t.Fatalf("CreateVideo: %v", err)
	}
	if err := s.CreateVideo(ctx, testVideo("v2", "clip.mp4")); !errors.Is(err, post.ErrDuplicateFilename) {
		t.Errorf("err = %v, want ErrDuplicateFilename", err)
	}
}

// A video row cannot outlive its post, and cannot exist without one.
func TestVideoRequiresItsPostAndCascades(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	if err := s.CreateVideo(ctx, post.Video{ID: "orphan", PostSlug: "ghost", Filename: "a.mp4",
		Key: "k", ContentType: "video/mp4", Bytes: 1, DurationMs: 1, Width: 1, Height: 1, CreatedAt: testNow}); err == nil {
		t.Error("a video for an unknown post was accepted — foreign keys are off")
	}

	if err := s.CreateVideo(ctx, testVideo("v1", "clip.mp4")); err != nil {
		t.Fatalf("CreateVideo: %v", err)
	}
	if deleted, err := s.DeletePost(ctx, "p", "alice"); err != nil || !deleted {
		t.Fatalf("DeletePost: %v, %v", deleted, err)
	}
	if _, err := s.GetVideo(ctx, "v1"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("the video row outlived its post: %v", err)
	}
}

// An upload row remembers which kind it reserved and what its PUT was signed for; a row
// written without either reads as the photo it is.
func TestUploadKindAndContentTypeRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	upload := post.Upload{ID: "u1", PostSlug: "p", Filename: "clip.mp4",
		Key: post.VideoObjectKey("p", "u1", "mp4"), Kind: post.AttachmentVideo,
		ContentType: "video/mp4", ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}
	if err := s.CreateUpload(ctx, upload); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	got, err := s.GetUpload(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUpload: %v", err)
	}
	if got.Kind != post.AttachmentVideo || got.ContentType != "video/mp4" {
		t.Errorf("upload = %+v", got)
	}
	byFilename, err := s.GetUploadByFilename(ctx, "p", "clip.mp4")
	if err != nil || byFilename.Kind != post.AttachmentVideo {
		t.Errorf("GetUploadByFilename = %+v, %v", byFilename, err)
	}
	expired, err := s.ListUploadsExpiredBefore(ctx, testNow.Add(time.Hour))
	if err != nil || len(expired) != 1 || expired[0].Kind != post.AttachmentVideo {
		t.Errorf("ListUploadsExpiredBefore = %+v, %v", expired, err)
	}
}

// A row inserted the way every upload row was written before videos existed reads as the
// photo it is: the column defaults are what keep a client that predates this change working.
func TestLegacyUploadRowReadsAsAPhoto(t *testing.T) {
	ctx := context.Background()
	s, handle := newStoreWithHandle(t)
	seedPost(t, s, "p", "alice", testNow)

	stamp := testNow.UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO uploads(id, post_slug, filename, r2_key, expires_at, created_at) VALUES(?,?,?,?,?,?)",
		"legacy", "p", "IMG_1.jpg", post.ObjectKey("p", "legacy"), stamp, stamp); err != nil {
		t.Fatalf("insert legacy upload: %v", err)
	}

	got, err := s.GetUpload(ctx, "legacy")
	if err != nil {
		t.Fatalf("GetUpload: %v", err)
	}
	if got.Kind != post.AttachmentPhoto || got.ContentType != "image/jpeg" {
		t.Errorf("legacy upload = %+v, want a photo of type image/jpeg", got)
	}
}

// The video confirm is one transaction, and the key it hands over is named by exactly one
// table afterwards — the invariant the orphan sweep depends on.
func TestConfirmVideoUploadIsAtomicAndReferenced(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	key := post.VideoObjectKey("p", "u1", "mp4")
	if err := s.CreateUpload(ctx, post.Upload{ID: "u1", PostSlug: "p", Filename: "clip.mp4",
		Key: key, Kind: post.AttachmentVideo, ContentType: "video/mp4",
		ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	video := testVideo("u1", "clip.mp4")
	if err := s.ConfirmVideoUpload(ctx, video, "u1"); err != nil {
		t.Fatalf("ConfirmVideoUpload: %v", err)
	}
	if _, err := s.GetVideo(ctx, "u1"); err != nil {
		t.Errorf("the video was not written: %v", err)
	}
	if _, err := s.GetUpload(ctx, "u1"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("the upload row survived the confirm: %v", err)
	}

	keys, err := s.AllReferencedKeys(ctx)
	if err != nil {
		t.Fatalf("AllReferencedKeys: %v", err)
	}
	if _, ok := keys[video.Key]; !ok {
		t.Errorf("%s is missing — the sweep would delete a live video", video.Key)
	}
	if len(keys) != 1 {
		t.Errorf("referenced keys = %v, want exactly one", keys)
	}
}

// A failed confirm leaves both sides untouched, so the upload stays retryable.
func TestConfirmVideoUploadRollsBack(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)

	// A confirmed video already holds the name the pending upload wants.
	if err := s.CreateVideo(ctx, testVideo("taken", "clip.mp4")); err != nil {
		t.Fatalf("CreateVideo: %v", err)
	}
	if err := s.CreateUpload(ctx, post.Upload{ID: "u1", PostSlug: "p", Filename: "clip.mp4",
		Key: post.VideoObjectKey("p", "u1", "mp4"), Kind: post.AttachmentVideo, ContentType: "video/mp4",
		ExpiresAt: testNow.Add(time.Minute), CreatedAt: testNow}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	if err := s.ConfirmVideoUpload(ctx, testVideo("u1", "clip.mp4"), "u1"); !errors.Is(err, post.ErrDuplicateFilename) {
		t.Fatalf("ConfirmVideoUpload = %v, want ErrDuplicateFilename", err)
	}
	if _, err := s.GetUpload(ctx, "u1"); err != nil {
		t.Errorf("the upload row was dropped by a failed confirm: %v", err)
	}
	if _, err := s.GetVideo(ctx, "u1"); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("a video row survived a rolled-back confirm: %v", err)
	}
}
