package storage

import (
	"bytes"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"
)

type clipS3 struct {
	fakeS3
	body *trackedBody
	size int64
	put  *s3.PutObjectInput
}
type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }
func (f *clipS3) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{ContentLength: aws.Int64(f.size), Body: f.body}, nil
}
func (f *clipS3) PutObject(_ context.Context, p *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.put = p
	_, err := io.Copy(io.Discard, p.Body)
	return &s3.PutObjectOutput{}, err
}
func TestClipStreamsAreBoundedAndClosed(t *testing.T) {
	for _, tc := range []struct {
		name, data  string
		size, limit int64
		bad         bool
	}{{"ok", "1234", 4, 4, false}, {"declared limit", "12345", 5, 4, true}, {"short", "123", 4, 4, true}, {"long", "12345", 4, 4, true}} {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tc.data)}
			f := &clipS3{body: body, size: tc.size}
			b := &Bucket{ops: f}
			var dst bytes.Buffer
			n, err := b.Download(context.Background(), "key", &dst, tc.limit)
			if (err != nil) != tc.bad || !body.closed || n > tc.limit+1 {
				t.Fatal(n, err, body.closed)
			}
			if err != nil && !errors.Is(err, clip.ErrInvalidMedia) {
				t.Fatal(err)
			}
		})
	}
}
func TestClipUploadUsesFileStreamAndConditionalWrite(t *testing.T) {
	f := &clipS3{}
	b := &Bucket{ops: f, name: "private"}
	body := strings.NewReader("video")
	if err := b.Upload(context.Background(), "result", body, 5, "video/mp4"); err != nil {
		t.Fatal(err)
	}
	if f.put.Body != body || aws.ToInt64(f.put.ContentLength) != 5 || aws.ToString(f.put.IfNoneMatch) != "*" {
		t.Fatal(f.put)
	}
}
func TestClipReadURLsSignInlineAndAttachmentDisposition(t *testing.T) {
	b, err := New(context.Background(), Config{Endpoint: "https://private.example", AccessKeyID: "test", SecretAccessKey: "test", Bucket: "private", MaxReadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	for _, attachment := range []bool{false, true} {
		link, err := b.PresignRead(context.Background(), "clip-results/alice/p/r.mp4", "서울 clip.mp4", attachment, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(link)
		d := u.Query().Get("response-content-disposition")
		kind := "inline"
		if attachment {
			kind = "attachment"
		}
		if !strings.HasPrefix(d, kind) || u.Query().Get("response-content-type") != "video/mp4" || u.Query().Get("X-Amz-Expires") != "60" {
			t.Fatal(d, u.Query())
		}
	}
}
