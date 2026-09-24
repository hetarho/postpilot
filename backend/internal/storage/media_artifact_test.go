package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
)

func TestMediaSigningUsesWorkerEndpointAndExactConditionalHeaders(t *testing.T) {
	b, err := New(t.Context(), Config{Endpoint: "http://api-storage:9000", PublicEndpoint: "http://browser-storage:9000", MediaEndpoint: "http://worker-storage:9000", AccessKeyID: "example", SecretAccessKey: "example-secret", Bucket: "private", MaxReadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	put, err := b.PresignMediaWrite(t.Context(), "clip-media/attempt/output.mp4", "video/mp4", 17, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(put.URL)
	if err != nil {
		t.Fatal("invalid signed URL")
	}
	if u.Host != "worker-storage:9000" || put.Headers["If-None-Match"] != "*" || put.Headers["Content-Type"] != "video/mp4" || put.Bytes != 17 {
		t.Fatal("worker signing lost endpoint/constraints")
	}
	signed := u.Query().Get("X-Amz-SignedHeaders")
	for _, name := range []string{"content-length", "content-type", "if-none-match"} {
		if !strings.Contains(signed, name) {
			t.Fatalf("required %s is not signed", name)
		}
	}
	get, err := b.PresignMediaRead(t.Context(), "clip-media/attempt/output.mp4", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err = url.Parse(get.URL)
	if err != nil || u.Host != "worker-storage:9000" {
		t.Fatal("read signed for browser")
	}
}

// Uses only a unique local bucket and the published development credentials.
func TestMediaArtifactDirectLocalStorage(t *testing.T) {
	endpoint := os.Getenv("CLIP_SOURCE_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("CLIP_SOURCE_TEST_ENDPOINT enables local MinIO")
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		t.Fatal("integration endpoint must be localhost")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("media-artifact-test-%d", time.Now().UnixNano())
	cfg := Config{Endpoint: endpoint, PublicEndpoint: "http://browser.invalid", MediaEndpoint: endpoint, AccessKeyID: "postpilot", SecretAccessKey: "postpilot-dev-secret", Bucket: name, MaxReadBytes: 1024}
	client, err := newClient(ctx, cfg, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)}); err != nil {
		t.Fatal("create local test bucket failed")
	}
	key := "clip-media/alice/project/attempt/output.mp4"
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(name), Key: aws.String(key)}); err != nil {
			t.Error("test object cleanup failed")
		}
		if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(name)}); err != nil {
			t.Error("test bucket cleanup failed")
		}
	})
	b, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	data := "media-test-bytes"
	put, err := b.PresignMediaWrite(ctx, key, "video/mp4", int64(len(data)), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method string, access clip.MediaArtifactAccess, body, contentType string) (int, string) {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, access.URL, strings.NewReader(body))
		if err != nil {
			t.Fatal("request creation failed")
		}
		for name, value := range access.Headers {
			r.Header.Set(name, value)
		}
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal("direct object request failed")
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return response.StatusCode, string(raw)
	}
	if code, _ := request(http.MethodPut, put, data, "text/plain"); code != http.StatusForbidden {
		t.Fatal("wrong content type accepted", code)
	}
	if code, _ := request(http.MethodPut, put, data+"extra", "video/mp4"); code == http.StatusOK {
		t.Fatal("wrong byte length accepted")
	}
	if code, _ := request(http.MethodPut, put, data, "video/mp4"); code != http.StatusOK {
		t.Fatal("valid conditional PUT failed", code)
	}
	if code, _ := request(http.MethodPut, put, data, "video/mp4"); code != http.StatusPreconditionFailed {
		t.Fatal("overwrite was allowed", code)
	}
	head, err := b.HeadMediaArtifact(ctx, key)
	if err != nil || head.Bytes != int64(len(data)) || head.ContentType != "video/mp4" {
		t.Fatal("HEAD mismatch")
	}
	get, err := b.PresignMediaRead(ctx, key, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if code, raw := request(http.MethodGet, get, "", ""); code != http.StatusOK || raw != data {
		t.Fatal("signed GET failed", code)
	}
	unsigned := get
	rawURL, _ := url.Parse(unsigned.URL)
	rawURL.RawQuery = ""
	unsigned.URL = rawURL.String()
	if code, _ := request(http.MethodGet, unsigned, "", ""); code != http.StatusForbidden {
		t.Fatal("private artifact is public", code)
	}
	missing, err := b.PresignMediaRead(ctx, "clip-media/missing.mp4", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := request(http.MethodGet, missing, "", ""); code != http.StatusNotFound {
		t.Fatal("missing object", code)
	}
	expired, err := b.PresignMediaRead(ctx, key, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-time.After(2100 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal("integration deadline")
	}
	if code, _ := request(http.MethodGet, expired, "", ""); code != http.StatusForbidden {
		t.Fatal("expired URL accepted", code)
	}
}
