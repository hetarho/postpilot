package storage

import (
	"bytes"
	"context"
	"io"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice/spoken"
)

var _ spoken.AudioObjects = (*Bucket)(nil)
var spokenKey = regexp.MustCompile(`^private/spoken/[a-f0-9]{32}\.mp3$`)

func (b *Bucket) PutSpokenAudio(ctx context.Context, key string, data []byte) error {
	if !spokenKey.MatchString(key) || len(data) == 0 || len(data) > llm.SpeechMaxAudioBytes {
		return spoken.ErrInvalid
	}
	uploader, ok := b.ops.(interface {
		PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	})
	if !ok {
		return spoken.ErrMediaUnavailable
	}
	_, err := uploader.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(b.name), Key: aws.String(key), Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data))), ContentType: aws.String("audio/mpeg"), CacheControl: aws.String("private, no-store"), IfNoneMatch: aws.String("*")})
	if err != nil {
		return spoken.ErrMediaUnavailable
	}
	return nil
}
func (b *Bucket) ReadSpokenAudio(ctx context.Context, key string, expected int64) ([]byte, error) {
	if !spokenKey.MatchString(key) || expected <= 0 || expected > llm.SpeechMaxAudioBytes {
		return nil, spoken.ErrInvalid
	}
	r, err := b.ops.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)})
	if err != nil {
		return nil, spoken.ErrMediaUnavailable
	}
	defer r.Body.Close()
	if r.ContentLength == nil || *r.ContentLength != expected {
		return nil, spoken.ErrMediaUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, expected+1))
	if err != nil || int64(len(data)) != expected {
		return nil, spoken.ErrMediaUnavailable
	}
	return data, nil
}
func (b *Bucket) DeleteSpokenAudio(ctx context.Context, key string) error {
	if !spokenKey.MatchString(key) {
		return spoken.ErrInvalid
	}
	_, err := b.ops.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)})
	if err != nil {
		return spoken.ErrMediaUnavailable
	}
	return nil
}
