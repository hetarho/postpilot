package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeS3 struct {
	headOutput *s3.HeadObjectOutput
	listPages  []*s3.ListObjectsV2Output
	listTokens []string
}

func (f *fakeS3) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return &s3.DeleteObjectOutput{}, nil
}
func (f *fakeS3) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(""))}, nil
}
func (f *fakeS3) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if f.headOutput == nil {
		return &s3.HeadObjectOutput{}, nil
	}
	return f.headOutput, nil
}
func (f *fakeS3) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.listTokens = append(f.listTokens, aws.ToString(input.ContinuationToken))
	if len(f.listPages) == 0 {
		return nil, errors.New("unexpected list page")
	}
	page := f.listPages[0]
	f.listPages = f.listPages[1:]
	return page, nil
}

func TestReadAtMost(t *testing.T) {
	for _, size := range []int{0, 4} {
		got, err := readAtMost(bytes.NewBuffer(bytes.Repeat([]byte{'x'}, size)), 4)
		if err != nil || len(got) != size {
			t.Fatalf("size %d: got=%d err=%v", size, len(got), err)
		}
	}
	_, err := readAtMost(strings.NewReader("12345"), 4)
	if err == nil {
		t.Fatal("oversized stream was accepted")
	}
}

func TestListFollowsEveryPage(t *testing.T) {
	firstTime := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	secondTime := firstTime.Add(time.Hour)
	ops := &fakeS3{listPages: []*s3.ListObjectsV2Output{
		{Contents: []types.Object{{Key: aws.String("posts/a"), LastModified: aws.Time(firstTime)}}, IsTruncated: aws.Bool(true), NextContinuationToken: aws.String("next")},
		{Contents: []types.Object{{Key: aws.String("posts/b"), LastModified: aws.Time(secondTime)}}},
	}}
	bucket := &Bucket{ops: ops, name: "photos"}

	objects, err := bucket.List(context.Background(), "posts/")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 2 || objects[0].Key != "posts/a" || objects[1].Key != "posts/b" {
		t.Fatalf("objects=%#v", objects)
	}
	if len(ops.listTokens) != 2 || ops.listTokens[0] != "" || ops.listTokens[1] != "next" {
		t.Fatalf("tokens=%v", ops.listTokens)
	}
}

func TestPresignGetUsesRequestedTTL(t *testing.T) {
	bucket, err := New(context.Background(), Config{Endpoint: "https://internal.example", PublicEndpoint: "https://public.example", AccessKeyID: "key", SecretAccessKey: "secret", Bucket: "photos", MaxReadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := bucket.PresignGet(context.Background(), "posts/alice/photo.jpg", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := neturl.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "public.example" || parsed.Query().Get("X-Amz-Expires") != "600" {
		t.Fatalf("signed URL=%s", signed)
	}
}
