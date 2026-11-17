package main

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/storage"
)

// experimentObjects is the private bucket as the experiment context's own TestPhotoObjects
// port (ARCH-6): the same bucket and presign rules as a post photo, with the storage types
// translated here.
type experimentObjects struct{ bucket *storage.Bucket }

func (a experimentObjects) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	return a.bucket.PresignPut(ctx, key, contentType, ttl)
}

func (a experimentObjects) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return a.bucket.PresignGet(ctx, key, ttl)
}

func (a experimentObjects) Head(ctx context.Context, key string) (int64, error) {
	head, err := a.bucket.Head(ctx, key)
	if errors.Is(err, post.ErrObjectNotFound) {
		return 0, experiment.ErrTestObjectNotFound
	}
	if err != nil {
		return 0, err
	}
	return head.Size, nil
}

func (a experimentObjects) Delete(ctx context.Context, key string) error {
	return a.bucket.Delete(ctx, key)
}

func (a experimentObjects) List(ctx context.Context, prefix string) ([]experiment.TestPhotoObject, error) {
	objects, err := a.bucket.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]experiment.TestPhotoObject, 0, len(objects))
	for _, object := range objects {
		out = append(out, experiment.TestPhotoObject{Key: object.Key, LastModified: object.LastModified})
	}
	return out, nil
}
