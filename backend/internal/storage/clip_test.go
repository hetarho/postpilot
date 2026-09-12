package storage

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClipPresignProtectsTypeAndPreventsOverwrite(t *testing.T) {
	b, err := New(context.Background(), Config{Endpoint: "http://storage.internal", PublicEndpoint: "http://storage.browser", AccessKeyID: "test", SecretAccessKey: "test", Bucket: "private", MaxReadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	put, err := b.PresignSource(context.Background(), "clip-inputs/alice/batch/source.mp4", "video/mp4", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(put.URL)
	if err != nil {
		t.Fatal(err)
	}
	signed := u.Query().Get("X-Amz-SignedHeaders")
	if u.Host != "storage.browser" || u.Query().Get("X-Amz-Expires") != "600" || !strings.Contains(signed, "content-type") || !strings.Contains(signed, "if-none-match") || put.Headers["If-None-Match"] != "*" || put.Headers["Content-Type"] != "video/mp4" {
		t.Fatalf("host=%s signed=%s headers=%v", u.Host, signed, put.Headers)
	}
}
func TestClipHeadReportsOnlyActualMetadata(t *testing.T) {
	b := &Bucket{ops: &fakeS3{headOutput: &s3.HeadObjectOutput{ContentLength: aws.Int64(42), ContentType: aws.String("video/mp4")}}}
	got, err := b.HeadSource(context.Background(), "clip-inputs/a/b/c.mp4")
	if err != nil || got != (clip.SourceObjectInfo{Bytes: 42, ContentType: "video/mp4"}) {
		t.Fatal(got, err)
	}
}

func TestClipPlaybackSignsContainerTypeAndLeavesRangeToTheBrowser(t *testing.T) {
	b, err := New(context.Background(), Config{Endpoint: "http://storage.internal", PublicEndpoint: "http://storage.browser", AccessKeyID: "test", SecretAccessKey: "test", Bucket: "private", MaxReadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	for _, mime := range []string{"video/mp4", "video/quicktime", "video/x-m4v", "video/webm"} {
		signed, err := b.PresignSourcePlayback(context.Background(), "clip-inputs/alice/b/source", mime, 30*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(signed)
		if err != nil {
			t.Fatal("invalid signed playback URL")
		}
		q := u.Query()
		if u.Host != "storage.browser" || q.Get("X-Amz-Expires") != "30" || q.Get("response-content-type") != mime || q.Get("response-cache-control") != "private, no-store" || strings.Contains(q.Get("X-Amz-SignedHeaders"), "range") {
			t.Fatal("playback signing did not preserve the bounded range capability")
		}
	}
}
