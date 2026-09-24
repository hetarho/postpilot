package storage

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
)

func mediaSignedHeaders(signed http.Header) (map[string]string, error) {
	headers := map[string]string{}
	for name, values := range signed {
		if name == "Host" {
			continue
		}
		if len(values) != 1 {
			return nil, errors.New("invalid media signed headers")
		}
		headers[name] = values[0]
	}
	return headers, nil
}

func (b *Bucket) PresignMediaRead(ctx context.Context, key string, ttl time.Duration) (clip.MediaArtifactAccess, error) {
	r, err := b.mediaPresign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)}, s3.WithPresignExpires(ttl))
	if err != nil {
		return clip.MediaArtifactAccess{}, errors.New("media read signing failed")
	}
	headers, err := mediaSignedHeaders(r.SignedHeader)
	if err != nil {
		return clip.MediaArtifactAccess{}, err
	}
	return clip.MediaArtifactAccess{URL: r.URL, Headers: headers, ExpiresAfter: ttl}, nil
}

func (b *Bucket) PresignMediaWrite(ctx context.Context, key, contentType string, bytes int64, ttl time.Duration) (clip.MediaArtifactAccess, error) {
	if bytes <= 0 {
		return clip.MediaArtifactAccess{}, clip.ErrInvalid
	}
	r, err := b.mediaPresign.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(b.name), Key: aws.String(key), ContentType: aws.String(contentType), ContentLength: aws.Int64(bytes), IfNoneMatch: aws.String("*")}, s3.WithPresignExpires(ttl))
	if err != nil {
		return clip.MediaArtifactAccess{}, errors.New("media upload signing failed")
	}
	headers, err := mediaSignedHeaders(r.SignedHeader)
	if err != nil {
		return clip.MediaArtifactAccess{}, err
	}
	headers["Content-Type"] = contentType
	headers["If-None-Match"] = "*"
	return clip.MediaArtifactAccess{URL: r.URL, Headers: headers, Bytes: bytes, ContentType: contentType, ExpiresAfter: ttl}, nil
}

func (b *Bucket) HeadMediaArtifact(ctx context.Context, key string) (clip.SourceObjectInfo, error) {
	return b.HeadSource(ctx, key)
}
