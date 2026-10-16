package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/postpilot/backend/internal/clip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHeadSourceKeepsCause(t *testing.T) {
	for _, cause := range []error{errors.New("SDK transport failure"), &types.NotFound{}} {
		b := &Bucket{ops: &fakeS3{headErr: fmt.Errorf("SDK request: %w", cause)}}
		_, err := b.HeadSource(t.Context(), "clip-inputs/alice/source")
		if !errors.Is(err, cause) {
			t.Fatalf("SDK cause was discarded: %v", err)
		}
		var missing *types.NotFound
		if errors.As(cause, &missing) && !errors.Is(err, clip.ErrNotFound) {
			t.Fatalf("not-found classification was lost: %v", err)
		}
	}
}

func TestHeadSourceReturnsCallerCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		for _, sdkErr := range []error{nil, errors.New("SDK HEAD interrupted")} {
			ctx, cancel := context.WithCancel(t.Context())
			if deadline {
				cancel()
				ctx, cancel = context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			} else {
				cancel()
			}
			b := &Bucket{ops: &fakeS3{headErr: sdkErr}}
			_, err := b.HeadSource(ctx, "clip-inputs/alice/source")
			if err != ctx.Err() {
				t.Errorf("expected caller cancellation %v, got %v", ctx.Err(), err)
			}
			cancel()
		}
	}
}

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
