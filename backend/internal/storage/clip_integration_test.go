package storage

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in local integration: creates and removes only a unique test bucket. It cannot
// address production, and uses the public development credentials from .env.example.
func TestClipDirectUploadLocalStorage(t *testing.T) {
	endpoint := os.Getenv("CLIP_SOURCE_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("CLIP_SOURCE_TEST_ENDPOINT enables the local MinIO integration")
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		t.Fatal("integration endpoint must be localhost")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	name := fmt.Sprintf("clip-test-%d", time.Now().UnixNano())
	cfg := Config{Endpoint: endpoint, PublicEndpoint: endpoint, AccessKeyID: "postpilot", SecretAccessKey: "postpilot-dev-secret", Bucket: name, MaxReadBytes: 1024}
	client, err := newClient(ctx, cfg, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)}); err != nil {
		t.Fatal(err)
	}
	key := "clip-inputs/test/batch/source.mp4"
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := client.DeleteObject(cleanup, &s3.DeleteObjectInput{Bucket: aws.String(name), Key: aws.String(key)}); err != nil {
			t.Error(err)
		}
		if _, err := client.DeleteBucket(cleanup, &s3.DeleteBucketInput{Bucket: aws.String(name)}); err != nil {
			t.Error(err)
		}
	}()
	b, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	put, err := b.PresignSource(ctx, key, "video/mp4", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request := func(contentType string) int {
		r, err := http.NewRequestWithContext(ctx, http.MethodPut, put.URL, strings.NewReader("source-data"))
		if err != nil {
			t.Fatal("build upload request")
		}
		for k, v := range put.Headers {
			r.Header.Set(k, v)
		}
		r.Header.Set("Content-Type", contentType)
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal("direct upload failed")
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		return response.StatusCode
	}
	if status := request("video/webm"); status != http.StatusForbidden {
		t.Fatalf("unsigned content type accepted: %d", status)
	}
	if status := request("video/mp4"); status != http.StatusOK {
		t.Fatalf("upload status: %d", status)
	}
	if status := request("video/mp4"); status != http.StatusPreconditionFailed {
		t.Fatalf("confirmed object overwrite status: %d", status)
	}
	info, err := b.HeadSource(ctx, key)
	if err != nil || info.Bytes != 11 || info.ContentType != "video/mp4" {
		t.Fatal(info, err)
	}
	keys, err := b.ListSourceKeys(ctx)
	if err != nil || len(keys) != 1 || keys[0] != key {
		t.Fatal(keys, err)
	}
	for range 2 {
		if err := b.Delete(ctx, key); err != nil {
			t.Fatal(err)
		}
	}
}
