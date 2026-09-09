package store_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
	"strings"
	"sync"
	"testing"
	"time"
)

type sourceObjects struct {
	mu         sync.Mutex
	info       map[string]clip.SourceObjectInfo
	failDelete map[string]bool
	signFails  bool
	deleted    []string
}

func fakeSources() *sourceObjects {
	return &sourceObjects{info: map[string]clip.SourceObjectInfo{}, failDelete: map[string]bool{}}
}
func (o *sourceObjects) PresignSource(_ context.Context, key, mime string, _ time.Duration) (clip.SignedSourcePut, error) {
	if o.signFails {
		return clip.SignedSourcePut{}, errors.New("signing failed")
	}
	return clip.SignedSourcePut{URL: "https://upload.example/" + key, Headers: map[string]string{"Content-Type": mime, "If-None-Match": "*"}}, nil
}
func (o *sourceObjects) HeadSource(_ context.Context, key string) (clip.SourceObjectInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	info, ok := o.info[key]
	if !ok {
		return info, clip.ErrNotFound
	}
	return info, nil
}
func (o *sourceObjects) Delete(_ context.Context, key string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failDelete[key] {
		return errors.New("storage unavailable")
	}
	o.deleted = append(o.deleted, key)
	delete(o.info, key)
	return nil
}
func (o *sourceObjects) ListSourceKeys(context.Context) ([]string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var keys []string
	for key := range o.info {
		keys = append(keys, key)
	}
	return keys, nil
}
func (o *sourceObjects) upload(b clip.SourceBatch) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, v := range b.Sources {
		o.info[v.Key] = clip.SourceObjectInfo{Bytes: v.Bytes, ContentType: v.ContentType}
	}
}
func manifest(count int) []clip.SourceMetadata {
	out := make([]clip.SourceMetadata, count)
	for i := range out {
		out[i] = clip.SourceMetadata{Filename: fmt.Sprintf("original private title %d.mp4", i), ContentType: "video/mp4", Bytes: 100, DurationMS: 1000, Width: 1920, Height: 1080, Fingerprint: fmt.Sprintf("%064x", i+1)}
	}
	return out
}
func TestSourceManifestLimits(t *testing.T) {
	s, store, _ := setup(t)
	_, p := create(t, s)
	src := clip.NewSourceService(store, fakeSources(), config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	tests := map[string]func() []clip.SourceMetadata{
		"none":      func() []clip.SourceMetadata { return nil },
		"count":     func() []clip.SourceMetadata { return manifest(21) },
		"duplicate": func() []clip.SourceMetadata { v := manifest(2); v[1].Fingerprint = v[0].Fingerprint; return v },
		"aggregate duration": func() []clip.SourceMetadata {
			v := manifest(2)
			v[0].DurationMS = config.ClipSourceDurationMS
			return v
		},
		"aggregate bytes": func() []clip.SourceMetadata {
			v := manifest(5)
			for i := range v {
				v[i].Bytes = config.ClipSourceFileBytes
			}
			return v
		},
	}
	changes := map[string]func(*clip.SourceMetadata){
		"fingerprint":           func(v *clip.SourceMetadata) { v.Fingerprint = "" },
		"malformed fingerprint": func(v *clip.SourceMetadata) { v.Fingerprint = strings.Repeat("z", 64) },
		"zero bytes":            func(v *clip.SourceMetadata) { v.Bytes = 0 },
		"file bytes":            func(v *clip.SourceMetadata) { v.Bytes = config.ClipSourceFileBytes + 1 },
		"zero duration":         func(v *clip.SourceMetadata) { v.DurationMS = 0 },
		"duration":              func(v *clip.SourceMetadata) { v.DurationMS = config.ClipSourceDurationMS + 1 },
		"width":                 func(v *clip.SourceMetadata) { v.Width = 0 },
		"height":                func(v *clip.SourceMetadata) { v.Height = -1 },
		"extension":             func(v *clip.SourceMetadata) { v.Filename = "video.avi" },
		"type mismatch":         func(v *clip.SourceMetadata) { v.ContentType = "video/webm" },
	}
	for name, change := range changes {
		tests[name] = func() []clip.SourceMetadata { v := manifest(1); change(&v[0]); return v }
	}
	for name, makeManifest := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := src.Create(context.Background(), "alice", p.ID, makeManifest()); !errors.Is(err, clip.ErrInvalid) {
				t.Fatalf("got %v", err)
			}
		})
	}
	for _, count := range []int{1, 20} {
		if _, err := src.Create(context.Background(), "alice", p.ID, manifest(count)); err != nil {
			t.Fatal(err)
		}
	}
	for ext, types := range config.ClipSourceLimits(6*time.Hour, 10*time.Minute).Containers {
		for _, mime := range types {
			v := manifest(1)
			v[0].Filename = "VIDEO." + strings.ToUpper(ext)
			v[0].ContentType = mime
			v[0].Bytes = config.ClipSourceFileBytes
			v[0].DurationMS = config.ClipSourceDurationMS
			if _, err := src.Create(context.Background(), "alice", p.ID, v); err != nil {
				t.Fatal(ext, mime, err)
			}
		}
	}
	v := manifest(4)
	for i := range v {
		v[i].Bytes = config.ClipSourceFileBytes
		v[i].DurationMS = config.ClipSourceDurationMS / 4
	}
	if _, err := src.Create(context.Background(), "alice", p.ID, v); err != nil {
		t.Fatal(err)
	}
}
func TestSourceOwnedCreateConfirmReplaceAndDiscard(t *testing.T) {
	s, store, _ := setup(t)
	_, p := create(t, s)
	objects := fakeSources()
	src := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	ctx := context.Background()
	for _, project := range []string{p.ID, "unknown"} {
		if _, err := src.Create(ctx, "bob", project, manifest(1)); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal(err)
		}
	}
	u, err := src.Create(ctx, "alice", p.ID, manifest(2))
	if err != nil {
		t.Fatal(err)
	}
	if u.Batch.State != "uploading" || len(u.Uploads) != 2 || u.Batch.ExpiresAt.Sub(u.Batch.CreatedAt) != 6*time.Hour || u.Uploads[0].ExpiresAt.Sub(u.Batch.CreatedAt) != 10*time.Minute {
		t.Fatalf("%+v", u)
	}
	for _, v := range u.Batch.Sources {
		if !strings.HasPrefix(v.Key, "clip-inputs/alice/"+u.Batch.ID+"/"+v.ID) || strings.Contains(v.Key, "private") {
			t.Fatal(v.Key)
		}
	}
	if err := src.Discard(ctx, "bob", u.Batch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Confirm(ctx, "bob", u.Batch.ID, u.Batch.Sources[0].ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	b, err := src.Confirm(ctx, "alice", u.Batch.ID, u.Batch.Sources[0].ID)
	if err != nil || b.State != "uploading" || b.Sources[0].ActualBytes != 100 {
		t.Fatal(b, err)
	}
	b, err = src.Confirm(ctx, "alice", u.Batch.ID, u.Batch.Sources[1].ID)
	if err != nil || b.State != "ready" {
		t.Fatal(b, err)
	}
	if _, err := src.Confirm(ctx, "alice", u.Batch.ID, u.Batch.Sources[1].ID); err != nil {
		t.Fatal(err)
	}
	newBatch, err := src.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSourceBatch(ctx, "alice", u.Batch.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if len(objects.deleted) != 2 {
		t.Fatal(objects.deleted)
	}
	for range 2 {
		if err := src.Discard(ctx, "alice", newBatch.Batch.ID); err != nil {
			t.Fatal(err)
		}
	}
	// A request can arrive after cleanup won the race; it must never resurrect readiness.
	if _, err := store.ConfirmSourceLease(ctx, "alice", newBatch.Batch.ID, newBatch.Batch.Sources[0].ID, 100, time.Now()); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestSourceMismatchAndPartialCleanupRemainRetryable(t *testing.T) {
	for _, mismatch := range []string{"size", "type"} {
		t.Run(mismatch, func(t *testing.T) {
			s, store, _ := setup(t)
			_, p := create(t, s)
			objects := fakeSources()
			src := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
			ctx := context.Background()
			u, err := src.Create(ctx, "alice", p.ID, manifest(2))
			if err != nil {
				t.Fatal(err)
			}
			objects.upload(u.Batch)
			key := u.Batch.Sources[0].Key
			v := objects.info[key]
			if mismatch == "size" {
				v.Bytes++
			} else {
				v.ContentType = "text/html"
			}
			objects.info[key] = v
			failedKey := u.Batch.Sources[1].Key
			objects.failDelete[failedKey] = true
			if _, err := src.Confirm(ctx, "alice", u.Batch.ID, u.Batch.Sources[0].ID); !errors.Is(err, clip.ErrInvalid) {
				t.Fatal(err)
			}
			b, err := store.GetSourceBatch(ctx, "alice", u.Batch.ID)
			if err != nil || b.State != "cleanup_pending" || len(b.Sources) != 2 {
				t.Fatal(b, err)
			}
			if _, err := src.Confirm(ctx, "alice", b.ID, b.Sources[1].ID); !errors.Is(err, clip.ErrSourceState) {
				t.Fatal(err)
			}
			if err := src.Sweep(ctx); err == nil {
				t.Fatal("failed delete was hidden")
			}
			objects.failDelete[failedKey] = false
			if err := src.Sweep(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := store.GetSourceBatch(ctx, "alice", b.ID); !errors.Is(err, clip.ErrNotFound) {
				t.Fatal(err)
			}
			// Late PUT after removal is recovered by the same prefix-limited sweep.
			objects.info[key] = v
			objects.info["posts/do-not-touch.jpg"] = v
			if err := src.Sweep(ctx); err != nil {
				t.Fatal(err)
			}
			if len(objects.info) != 1 {
				t.Fatal(objects.info)
			}
		})
	}
}
func TestSourceExpiryConsumptionAndProjectDeletionFence(t *testing.T) {
	s, store, d := setup(t)
	_, p := create(t, s)
	objects := fakeSources()
	src := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	s.SetSources(src)
	ctx := context.Background()
	u, err := src.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	if _, err := d.Writer.Exec("UPDATE clip_source_batches SET state='consuming',expires_at=? WHERE id=?", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), u.Batch.ID); err != nil {
		t.Fatal(err)
	}
	if err := src.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if len(objects.info) != 1 {
		t.Fatal("consuming bytes were swept")
	}
	if _, err := src.Create(ctx, "alice", p.ID, manifest(1)); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal(err)
	}
	if err := src.Discard(ctx, "alice", u.Batch.ID); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal(err)
	}
	if err := s.DeleteProject(ctx, "alice", p.ID); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal(err)
	}
	if _, err := d.Writer.Exec("UPDATE clip_source_batches SET state='ready' WHERE id=?", u.Batch.ID); err != nil {
		t.Fatal(err)
	}
	if err := src.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if len(objects.info) != 0 {
		t.Fatal("expired ready input survived")
	}
	u, err = src.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	objects.failDelete[u.Batch.Sources[0].Key] = true
	if err := s.DeleteProject(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetProject(ctx, "alice", p.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	b, err := store.GetSourceBatch(ctx, "alice", u.Batch.ID)
	if err != nil || b.State != "cleanup_pending" {
		t.Fatal(b, err)
	}
	if _, err := src.Create(ctx, "alice", p.ID, manifest(1)); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	objects.failDelete[u.Batch.Sources[0].Key] = false
	if err := src.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestSourceDeleteFailsClosedWhenCleanupIntentCannotBeSaved(t *testing.T) {
	s, store, d := setup(t)
	_, p := create(t, s)
	objects := fakeSources()
	src := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	s.SetSources(src)
	ctx := context.Background()
	u, err := src.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	if _, err := d.Writer.Exec("CREATE TRIGGER refuse_cleanup BEFORE UPDATE OF state ON clip_source_batches BEGIN SELECT RAISE(ABORT,'cleanup unavailable'); END"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject(ctx, "alice", p.ID); err == nil {
		t.Fatal("delete must fail closed")
	}
	if _, err := s.GetProject(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	if len(objects.deleted) != 0 {
		t.Fatal("deleted before durable intent")
	}
}
func TestSourceBootSweepAndSigningFailure(t *testing.T) {
	s, store, d := setup(t)
	_, p := create(t, s)
	objects := fakeSources()
	src := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	ctx := context.Background()
	objects.signFails = true
	if _, err := src.Create(ctx, "alice", p.ID, manifest(1)); err == nil {
		t.Fatal("signing failure hidden")
	}
	objects.signFails = false
	u, err := src.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	if _, err := d.Writer.Exec("UPDATE clip_source_batches SET expires_at=?", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() { src.Run(runCtx, time.Hour); close(done) }()
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("boot sweep did not run")
		case <-ticker.C:
			if _, err := store.GetSourceBatch(ctx, "alice", u.Batch.ID); errors.Is(err, clip.ErrNotFound) {
				cancel()
				<-done
				return
			}
		}
	}
}

func TestSourceSchemaStoresMetadataOnlyAndNoSignedURLs(t *testing.T) {
	s, store, d := setup(t)
	_, p := create(t, s)
	src := clip.NewSourceService(store, fakeSources(), config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	if _, err := src.Create(context.Background(), "alice", p.ID, manifest(1)); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"clip_source_batches", "clip_source_leases"} {
		rows, err := d.Reader.Query("SELECT name,type FROM pragma_table_info(?)", table)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var name, kind string
			if err := rows.Scan(&name, &kind); err != nil {
				t.Fatal(err)
			}
			if kind == "BLOB" || strings.Contains(name, "url") || strings.Contains(name, "payload") {
				t.Fatalf("forbidden retained data: %s.%s %s", table, name, kind)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	var key string
	if err := d.Reader.QueryRow("SELECT object_key FROM clip_source_leases").Scan(&key); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(key, "://") || strings.Contains(key, "?") {
		t.Fatal("signed URL persisted")
	}
}
