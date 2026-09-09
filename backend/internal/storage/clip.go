package storage

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
	"time"
)

var _ clip.ObjectStore = (*Bucket)(nil)

func (b *Bucket) PresignSource(ctx context.Context, key, contentType string, ttl time.Duration) (clip.SignedSourcePut, error) {
	request, err := b.presign.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(b.name), Key: aws.String(key), ContentType: aws.String(contentType), IfNoneMatch: aws.String("*")}, s3.WithPresignExpires(ttl))
	if err != nil {
		return clip.SignedSourcePut{}, errors.New("sign clip source upload failed")
	}
	// Conditional PUT prevents an unexpired URL from overwriting a confirmed input.
	headers := map[string]string{"Content-Type": contentType, "If-None-Match": "*"}
	for name, values := range request.SignedHeader {
		if name == "Host" {
			continue
		} // browser owns Host
		if len(values) != 1 {
			return clip.SignedSourcePut{}, errors.New("unexpected signed header values")
		}
		headers[name] = values[0]
	}
	return clip.SignedSourcePut{URL: request.URL, Headers: headers}, nil
}
func (b *Bucket) HeadSource(ctx context.Context, key string) (clip.SourceObjectInfo, error) {
	response, err := b.ops.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return clip.SourceObjectInfo{}, clip.ErrNotFound
		}
		return clip.SourceObjectInfo{}, errors.New("head clip source failed")
	}
	return clip.SourceObjectInfo{Bytes: aws.ToInt64(response.ContentLength), ContentType: aws.ToString(response.ContentType)}, nil
}
func (b *Bucket) ListSourceKeys(ctx context.Context) ([]string, error) {
	objects, err := b.List(ctx, clip.SourcePrefix)
	if err != nil {
		return nil, errors.New("list clip sources failed")
	}
	out := make([]string, 0, len(objects))
	for _, o := range objects {
		out = append(out, o.Key)
	}
	return out, nil
}
