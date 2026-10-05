package storage

import (
	"bytes"
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"io"
	"regexp"
)

var clipSpeechKey = regexp.MustCompile(`^private/clip-speech/[a-f0-9]{32}\.mp3$`)

// Project speech has its own private namespace, outside reusable-voice orphan cleanup.
func (b *Bucket) PutClipSpeechAudio(ctx context.Context, key string, data []byte) error {
	if !clipSpeechKey.MatchString(key) || len(data) == 0 || len(data) > llm.SpeechMaxAudioBytes {
		return clip.ErrInvalid
	}
	uploader, ok := b.ops.(interface {
		PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	})
	if !ok {
		return clip.ErrSourceMissing
	}
	_, e := uploader.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(b.name), Key: aws.String(key), Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data))), ContentType: aws.String("audio/mpeg"), CacheControl: aws.String("private, no-store"), IfNoneMatch: aws.String("*")})
	return e
}
func (b *Bucket) ReadClipSpeechAudio(ctx context.Context, key string, expected int64) ([]byte, error) {
	if !clipSpeechKey.MatchString(key) || expected <= 0 || expected > llm.SpeechMaxAudioBytes {
		return nil, clip.ErrInvalid
	}
	r, e := b.ops.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)})
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.ContentLength == nil || *r.ContentLength != expected {
		return nil, clip.ErrSourceMissing
	}
	data, e := io.ReadAll(io.LimitReader(r.Body, expected+1))
	if e != nil {
		return nil, e
	}
	if int64(len(data)) != expected {
		return nil, clip.ErrSourceMissing
	}
	return data, nil
}
func (b *Bucket) DeleteClipSpeechAudio(ctx context.Context, key string) error {
	if !clipSpeechKey.MatchString(key) {
		return clip.ErrInvalid
	}
	_, e := b.ops.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(b.name), Key: aws.String(key)})
	return e
}
