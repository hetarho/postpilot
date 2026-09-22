package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Opt-in local MinIO proof for the temporary retirement bridge. It creates a
// unique bucket and proves a full publishing-prefix sweep cannot touch source
// post or clip objects in the same bucket.
func TestPublishingRetirementLocalStorage(t *testing.T) {
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
	name := fmt.Sprintf("publishing-retirement-test-%d", time.Now().UnixNano())
	cfg := Config{Endpoint: endpoint, PublicEndpoint: endpoint, AccessKeyID: "postpilot", SecretAccessKey: "postpilot-dev-secret", Bucket: name, MaxReadBytes: 1024}
	client, err := newClient(ctx, cfg, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)}); err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{
		"publishing/job/referenced.jpg": "copy",
		"publishing/orphan.jpg":         "orphan",
		"posts/alice/source.jpg":        "source-photo",
		"clip-inputs/alice/source.mp4":  "source-video",
	}
	defer func() {
		for key := range keys {
			_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(name), Key: aws.String(key)})
		}
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(name)})
	}()
	for key, body := range keys {
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(name), Key: aws.String(key), Body: bytes.NewReader([]byte(body))}); err != nil {
			t.Fatal(err)
		}
	}
	bucket, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := bucket.ListStaged(ctx, "publishing/")
	if err != nil || len(objects) != 2 {
		t.Fatalf("publishing inventory = %v err=%v", objects, err)
	}
	for _, object := range objects {
		if err := bucket.Delete(ctx, object.Key); err != nil {
			t.Fatal(err)
		}
	}
	if remaining, err := bucket.ListStaged(ctx, "publishing/"); err != nil || len(remaining) != 0 {
		t.Fatalf("remaining publishing objects = %v err=%v", remaining, err)
	}
	for _, key := range []string{"posts/alice/source.jpg", "clip-inputs/alice/source.mp4"} {
		out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(name), Key: aws.String(key)})
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(out.Body)
		out.Body.Close()
		if readErr != nil || sha256.Sum256(body) != sha256.Sum256([]byte(keys[key])) {
			t.Fatalf("source checksum changed for %s: %v", key, readErr)
		}
	}
	// S3/R2 deletion is idempotent, which makes an interrupted retry safe.
	if err := bucket.Delete(ctx, "publishing/orphan.jpg"); err != nil {
		t.Fatal(err)
	}
}
