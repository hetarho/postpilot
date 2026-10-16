package store_test

import (
	"context"
	"github.com/postpilot/backend/internal/post"
	"testing"
	"time"
)

func TestMediaCaptureTimeRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "p", "alice", testNow)
	at := time.Date(2026, 10, 15, 12, 34, 56, 123000000, time.FixedZone("camera", 9*3600))
	for _, captured := range []*time.Time{&at, nil} {
		id := "captured"
		if captured == nil {
			id = "legacy"
		}
		image := post.Image{ID: id, PostSlug: "p", Filename: id + ".jpg", Key: post.ObjectKey("p", id), Width: 10, Height: 10, Bytes: 100, CreatedAt: testNow, CapturedAt: captured}
		video := testVideo(id, id+".mp4")
		video.CapturedAt = captured
		if err := s.CreateImage(ctx, image); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateVideo(ctx, video); err != nil {
			t.Fatal(err)
		}
		gotImage, err := s.GetImage(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		gotVideo, err := s.GetVideo(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, got := range []*time.Time{gotImage.CapturedAt, gotVideo.CapturedAt} {
			if captured == nil {
				if got != nil {
					t.Fatal("invented legacy capture time", got)
				}
				continue
			}
			if got == nil || !got.Equal(at) || got.Location() != time.UTC {
				t.Fatalf("capture=%v", got)
			}
		}
	}
	images, err := s.ListImages(ctx, "p")
	if err != nil || len(images) != 2 {
		t.Fatal(images, err)
	}
	videos, err := s.ListVideos(ctx, "p")
	if err != nil || len(videos) != 2 {
		t.Fatal(videos, err)
	}
	if images[0].CapturedAt == nil || videos[0].CapturedAt == nil {
		t.Fatal("list dropped captured time")
	}
}
