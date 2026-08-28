package post

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// fakeStore is an in-memory post.Store. These tests are about the context's rules —
// ownership, the upload handshake, what the sweep may delete — not about SQL.
type fakeStore struct {
	mu      sync.Mutex
	posts   map[string]Post
	images  map[string]Image
	uploads map[string]Upload

	// slugTaken lets a test simulate another request claiming a slug between the
	// existence check and the insert — the race the retry loop exists for.
	slugTaken func(slug string)
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		posts:   map[string]Post{},
		images:  map[string]Image{},
		uploads: map[string]Upload{},
	}
}

func (f *fakeStore) CreatePost(_ context.Context, p Post) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.posts[p.Slug]; exists {
		return ErrDuplicateSlug
	}
	f.posts[p.Slug] = p
	return nil
}

func (f *fakeStore) UpdateDraft(_ context.Context, slug, userID, title, memo string, updatedAt time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.posts[slug]
	if !ok || existing.UserID != userID {
		return false, nil
	}
	existing.Title = title
	existing.Memo = memo
	existing.UpdatedAt = updatedAt
	f.posts[slug] = existing
	return true, nil
}

func (f *fakeStore) GetPost(_ context.Context, slug string) (Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.posts[slug]
	if !ok {
		return Post{}, ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) SlugExists(_ context.Context, slug string) (bool, error) {
	f.mu.Lock()
	_, ok := f.posts[slug]
	hook := f.slugTaken
	f.mu.Unlock()

	if !ok && hook != nil {
		hook(slug)
	}
	return ok, nil
}

func (f *fakeStore) ListPosts(_ context.Context, userID string) ([]Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Summary
	for _, p := range f.posts {
		if p.UserID != userID {
			continue
		}
		out = append(out, Summary{Slug: p.Slug, Title: p.Title, Status: p.Status, UpdatedAt: p.UpdatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (f *fakeStore) CreateImage(_ context.Context, img Image) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createImageLocked(img)
}

// createImageLocked mirrors the real schema's constraints, so a test sees the same
// failures production would.
func (f *fakeStore) createImageLocked(img Image) error {
	if _, exists := f.images[img.ID]; exists {
		return ErrDuplicateFilename
	}
	for _, existing := range f.images {
		if existing.PostSlug == img.PostSlug && existing.Filename == img.Filename {
			return ErrDuplicateFilename
		}
	}
	f.images[img.ID] = img
	return nil
}

// ConfirmUpload is atomic here too — a fake that let the two writes come apart would
// hide the very bug the real transaction exists to prevent.
func (f *fakeStore) ConfirmUpload(_ context.Context, img Image, uploadID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.createImageLocked(img); err != nil {
		return err
	}
	delete(f.uploads, uploadID)
	return nil
}

func (f *fakeStore) ListImages(_ context.Context, postSlug string) ([]Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Image
	for _, img := range f.images {
		if img.PostSlug == postSlug {
			out = append(out, img)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) GetImage(_ context.Context, id string) (Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	img, ok := f.images[id]
	if !ok {
		return Image{}, ErrNotFound
	}
	return img, nil
}

func (f *fakeStore) DeleteImage(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.images, id)
	return nil
}

func (f *fakeStore) ImageFilenameTaken(_ context.Context, postSlug, filename string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, img := range f.images {
		if img.PostSlug == postSlug && img.Filename == filename {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) ImageKeyInUse(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, img := range f.images {
		if img.Key == key {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) CreateUpload(_ context.Context, u Upload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// UNIQUE(post_slug, filename), as in the schema.
	for _, existing := range f.uploads {
		if existing.PostSlug == u.PostSlug && existing.Filename == u.Filename {
			return ErrDuplicateFilename
		}
	}
	f.uploads[u.ID] = u
	return nil
}

func (f *fakeStore) GetUploadByFilename(_ context.Context, postSlug, filename string) (Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.uploads {
		if u.PostSlug == postSlug && u.Filename == filename {
			return u, nil
		}
	}
	return Upload{}, ErrNotFound
}

func (f *fakeStore) GetUpload(_ context.Context, id string) (Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[id]
	if !ok {
		return Upload{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) DeleteUpload(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.uploads, id)
	return nil
}

func (f *fakeStore) ListUploadsExpiredBefore(_ context.Context, t time.Time) ([]Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Upload
	for _, u := range f.uploads {
		if u.ExpiresAt.Before(t) {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) AllReferencedKeys(_ context.Context) (map[string]struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := map[string]struct{}{}
	for _, img := range f.images {
		keys[img.Key] = struct{}{}
	}
	for _, u := range f.uploads {
		keys[u.Key] = struct{}{}
	}
	return keys, nil
}

// fakeBlobs is an in-memory ObjectStore. It records deletes so a test can assert that
// storage was reached, which is the half of DeleteImage a database cannot show.
type fakeBlobs struct {
	mu      sync.Mutex
	objects map[string]fakeObject
	deleted []string

	// failDelete makes every Delete fail, for the "storage is down" paths.
	failDelete bool
	// failList makes List fail, so the sweep can be shown to delete nothing.
	failList bool
}

type fakeObject struct {
	size         int64
	lastModified time.Time
}

func newFakeBlobs() *fakeBlobs {
	return &fakeBlobs{objects: map[string]fakeObject{}}
}

func (f *fakeBlobs) put(key string, size int64, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = fakeObject{size: size, lastModified: at}
}

func (f *fakeBlobs) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *fakeBlobs) PresignPut(_ context.Context, key, contentType string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("https://storage.example/%s?put&ct=%s&ttl=%s", key, contentType, ttl), nil
}

func (f *fakeBlobs) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("https://storage.example/%s?get&ttl=%s", key, ttl), nil
}

func (f *fakeBlobs) Head(_ context.Context, key string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[key]
	if !ok {
		return 0, ErrObjectNotFound
	}
	return obj.size, nil
}

func (f *fakeBlobs) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failDelete {
		return fmt.Errorf("storage unavailable")
	}
	f.deleted = append(f.deleted, key)
	delete(f.objects, key)
	return nil
}

func (f *fakeBlobs) List(_ context.Context, prefix string) ([]Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failList {
		return nil, fmt.Errorf("storage unavailable")
	}
	var out []Object
	for key, obj := range f.objects {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, Object{Key: key, Size: obj.size, LastModified: obj.lastModified})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
