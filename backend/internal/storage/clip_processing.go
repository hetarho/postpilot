package storage

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"mime"
	"time"
)

var _ clip.ProcessingObjects = (*Bucket)(nil)

func (b *Bucket) Download(ctx context.Context, key string, dst io.Writer, limit int64) (int64, error) {
	if limit <= 0 {
		return 0, errors.New("invalid download limit")
	}
	out, err := b.ops.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)})
	if err != nil {
		return 0, errors.New("clip object download failed")
	}
	defer out.Body.Close()
	if out.ContentLength == nil || *out.ContentLength <= 0 || *out.ContentLength > limit {
		return 0, clip.ErrInvalidMedia
	}
	n, err := io.Copy(dst, io.LimitReader(out.Body, *out.ContentLength+1))
	if err != nil {
		return n, errors.New("clip object stream failed")
	}
	if n != *out.ContentLength {
		return n, clip.ErrInvalidMedia
	}
	return n, nil
}
func (b *Bucket) Upload(ctx context.Context, key string, body io.ReadSeeker, bytes int64, contentType string) error {
	if bytes <= 0 {
		return clip.ErrInvalidMedia
	}
	// A file-backed ReadSeeker lets SigV4 hash/retry without buffering the video.
	uploader, ok := b.ops.(interface {
		PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	})
	if !ok {
		return errors.New("clip upload unavailable")
	}
	_, err := uploader.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(b.name), Key: aws.String(key), Body: body, ContentLength: aws.Int64(bytes), ContentType: aws.String(contentType), IfNoneMatch: aws.String("*")})
	if err != nil {
		return errors.New("clip object upload failed")
	}
	return nil
}
func (b *Bucket) PresignRead(ctx context.Context, key, filename string, attachment bool, ttl time.Duration) (string, error) {
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	params := map[string]string{}
	if filename != "" {
		params["filename"] = filename
	}
	req, err := b.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.name), Key: aws.String(key), ResponseContentType: aws.String("video/mp4"), ResponseContentDisposition: aws.String(mime.FormatMediaType(disposition, params))}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", errors.New("clip read signing failed")
	}
	return req.URL, nil
}
func (b *Bucket) ListResults(ctx context.Context) ([]clip.StoredObject, error) {
	found, err := b.List(ctx, clip.ResultPrefix)
	if err != nil {
		return nil, errors.New("clip result listing failed")
	}
	out := make([]clip.StoredObject, 0, len(found))
	for _, o := range found {
		out = append(out, clip.StoredObject{Key: o.Key, Modified: o.LastModified})
	}
	return out, nil
}
