package post

import (
	"context"
	"testing"
	"time"
)

func TestConfirmUploadCaptureTime(t *testing.T) {
	for _, kind := range []AttachmentKind{AttachmentPhoto, AttachmentVideo} {
		t.Run(string(kind), func(t *testing.T) {
			svc, _, blobs := newTestService(t)
			ctx := context.Background()
			p := mustCreatePost(t, svc, alice, "Capture")
			filename, duration := "camera.jpg", int64(0)
			if kind == AttachmentVideo {
				filename, duration = "camera.mov", 8000
			}
			u, _, mime, err := svc.CreateUpload(ctx, alice, p.Slug, filename, kind)
			if err != nil {
				t.Fatal(err)
			}
			blobs.putTyped(u.Key, 100, mime, testNow)
			at := time.Date(2026, 10, 15, 12, 34, 56, 0, time.FixedZone("camera", 9*3600))
			first, err := svc.ConfirmUpload(ctx, alice, u.ID, 10, 10, duration, &at)
			if err != nil {
				t.Fatal(err)
			}
			later := at.Add(time.Hour)
			retry, err := svc.ConfirmUpload(ctx, alice, u.ID, 10, 10, duration, &later)
			if err != nil {
				t.Fatal(err)
			}
			for _, got := range []Attachment{first, retry} {
				captured := got.Image.CapturedAt
				if kind == AttachmentVideo {
					captured = got.Video.CapturedAt
				}
				if captured == nil || !captured.Equal(at) || captured.Location() != time.UTC {
					t.Fatalf("capture = %v", captured)
				}
			}
		})
	}
}

func TestConfirmVideoIgnoresPlaceholderCaptureTime(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "Capture")
	u, _, mime, err := svc.CreateUpload(ctx, alice, p.Slug, "camera.mp4", AttachmentVideo)
	if err != nil {
		t.Fatal(err)
	}
	blobs.putTyped(u.Key, 100, mime, testNow)
	old := time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC)
	got, err := svc.ConfirmUpload(ctx, alice, u.ID, 10, 10, 8000, &old)
	if err != nil || got.Video.CapturedAt != nil {
		t.Fatalf("got %+v / %v", got, err)
	}
}
