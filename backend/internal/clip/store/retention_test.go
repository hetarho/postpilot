package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

type racingSourceSigner struct {
	clip.ObjectStore
	beforePut, beforeGet func()
}

func (o *racingSourceSigner) PresignSource(ctx context.Context, key, mime string, ttl time.Duration) (clip.SignedSourcePut, error) {
	if o.beforePut != nil {
		o.beforePut()
	}
	return o.ObjectStore.PresignSource(ctx, key, mime, ttl)
}

func (o *racingSourceSigner) PresignSourcePlayback(ctx context.Context, key, mime string, ttl time.Duration) (string, error) {
	if o.beforeGet != nil {
		o.beforeGet()
	}
	return o.ObjectStore.PresignSourcePlayback(ctx, key, mime, ttl)
}

func TestOriginalCapabilitiesRejectChangesDuringSigning(t *testing.T) {
	for _, mode := range []string{"put", "get"} {
		for _, change := range []string{"replace", "revoke"} {
			t.Run(mode+"/"+change, func(t *testing.T) {
				projects, store, _ := setup(t)
				_, p := create(t, projects)
				ctx := context.Background()
				objects := fakeSources()
				signer := &racingSourceSigner{ObjectStore: objects}
				cfg := config.ClipSourceLimits(6*time.Hour, 10*time.Minute)
				sources := clip.NewSourceService(store, signer, cfg)
				other := clip.NewSourceService(store, objects, cfg)
				mutate := func() {
					var err error
					if change == "revoke" {
						err = other.RevokeProject(ctx, "alice", p.ID)
					} else {
						_, err = other.Create(ctx, "alice", p.ID, manifest(2))
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if mode == "put" {
					signer.beforePut = mutate
					u, err := sources.Create(ctx, "alice", p.ID, manifest(1))
					if !errors.Is(err, clip.ErrSourceState) || len(u.Uploads) != 0 {
						t.Fatal("obsolete upload capabilities returned", err)
					}
					return
				}
				u, err := sources.Create(ctx, "alice", p.ID, manifest(1))
				if err != nil {
					t.Fatal(err)
				}
				objects.upload(u.Batch)
				v := u.Batch.Sources[0]
				if _, err = sources.Confirm(ctx, "alice", u.Batch.ID, v.ID); err != nil {
					t.Fatal(err)
				}
				signer.beforeGet = mutate
				link, err := sources.Playback(ctx, "alice", p.ID, v.ID, v.Fingerprint)
				if err == nil || link.URL != "" {
					t.Fatal("obsolete playback capability returned", err)
				}
			})
		}
	}
}

func TestOriginalRetentionMutationsAndAttemptReleaseAreNotReadLeases(t *testing.T) {
	projects, store, _ := setup(t)
	_, p := create(t, projects)
	ctx := context.Background()
	now := time.Now().UTC()
	objects := fakeSources()
	sources := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return now })
	upload, err := sources.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(upload.Batch)
	b, err := sources.Confirm(ctx, "alice", upload.Batch.ID, upload.Batch.Sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	expires := now.Add(24 * time.Hour)
	assertExpiry := func(want time.Time) {
		t.Helper()
		got, e := store.GetSourceBatch(ctx, "alice", b.ID)
		if e != nil || !got.ExpiresAt.Equal(want) {
			t.Fatal(got.ExpiresAt, want, e)
		}
	}
	assertExpiry(expires)
	now = now.Add(time.Hour)
	if _, err = sources.Confirm(ctx, "alice", b.ID, b.Sources[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = sources.GetSources(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, b.Sources[0].ID, b.Sources[0].Fingerprint); err != nil {
		t.Fatal(err)
	}
	if _, err = store.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Title: &p.Title}, now); err != nil {
		t.Fatal(err)
	}
	assertExpiry(expires)
	changed := "Changed title"
	if _, err = store.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Title: &changed}, now); err != nil {
		t.Fatal(err)
	}
	expires = now.Add(24 * time.Hour)
	assertExpiry(expires)
	now = now.Add(time.Hour)
	if err = store.LinkSourceJob(ctx, "alice", b.ID, "first", now); err != nil {
		t.Fatal(err)
	}
	expires = now.Add(24 * time.Hour)
	assertExpiry(expires)
	now = expires
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := objects.info[b.Sources[0].Key]; !ok {
		t.Fatal("active original deleted at expiry")
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, b.Sources[0].ID, b.Sources[0].Fingerprint); !errors.Is(err, clip.ErrSourceExpired) {
		t.Fatal(err)
	}
	if err = sources.ReleaseAttempt(ctx, "alice", "first", now); err != nil {
		t.Fatal(err)
	}
	expires = now.Add(24 * time.Hour)
	assertExpiry(expires)
	now = now.Add(time.Hour)
	if err = sources.ReleaseAttempt(ctx, "alice", "first", now); err != nil {
		t.Fatal(err)
	}
	assertExpiry(expires)
	if err = store.LinkSourceJob(ctx, "alice", b.ID, "second", now); err != nil {
		t.Fatal("same-batch retry", err)
	}
	prior, err := store.BatchForJob(ctx, "alice", "first")
	if err != nil || prior.JobID != "first" || prior.ID != b.ID {
		t.Fatal(prior, err)
	}
	if err = sources.ReleaseAttempt(ctx, "alice", "second", now); err != nil {
		t.Fatal(err)
	}
}

func TestPartialUploadExpiryDoesNotShortenConfirmedOriginals(t *testing.T) {
	projects, store, _ := setup(t)
	_, p := create(t, projects)
	ctx := context.Background()
	now := time.Now().UTC()
	objects := fakeSources()
	sources := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return now })
	u, err := sources.Create(ctx, "alice", p.ID, manifest(2))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	b, err := sources.Confirm(ctx, "alice", u.Batch.ID, u.Batch.Sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(6 * time.Hour)
	if _, err = sources.Confirm(ctx, "alice", b.ID, b.Sources[1].ID); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal(err)
	}
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := objects.info[b.Sources[0].Key]; !ok {
		t.Fatal("confirmed original followed incomplete-upload TTL")
	}
	if _, ok := objects.info[b.Sources[1].Key]; ok {
		t.Fatal("unconfirmed bytes survived incomplete-upload TTL")
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, b.Sources[0].ID, b.Sources[0].Fingerprint); err != nil {
		t.Fatal(err)
	}
	if _, err = sources.Confirm(ctx, "alice", b.ID, b.Sources[0].ID); err != nil {
		t.Fatal("confirmed replay must remain readable", err)
	}
	now = now.Add(18 * time.Hour)
	changed := "Text without pixels"
	if _, err = store.UpdateProject(ctx, "alice", p.ID, clip.ProjectPatch{Title: &changed}, now); err != nil {
		t.Fatal(err)
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, b.Sources[0].ID, b.Sources[0].Fingerprint); !errors.Is(err, clip.ErrSourceExpired) {
		t.Fatal("expired original revived by text save", err)
	}
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if len(objects.info) != 0 {
		t.Fatal("expired objects retained", objects.info)
	}
}

func TestOriginalTombstoneOutlivesAnIssuedPutAndPermanentDeletion(t *testing.T) {
	projects, store, _ := setup(t)
	_, p := create(t, projects)
	ctx := context.Background()
	now := time.Now().UTC()
	objects := fakeSources()
	sources := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return now })
	u, err := sources.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	if err = sources.PrepareProjectDelete(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = sources.GetSources(ctx, "alice", p.ID); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal("deletion fence lost", err)
	}
	if err = store.DeleteProject(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	b, err := store.GetSourceBatch(ctx, "alice", u.Batch.ID)
	if err != nil || b.State != "cleanup_pending" {
		t.Fatal("PUT tombstone missing", b, err)
	}
	objects.upload(u.Batch)
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if len(objects.info) != 0 {
		t.Fatal("late PUT escaped retry")
	}
	now = now.Add(10 * time.Minute)
	objects.upload(u.Batch)
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetSourceBatch(ctx, "alice", u.Batch.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("finished tombstone retained", err)
	}
	objects.upload(u.Batch) // A PUT started before expiry can finish after it.
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if len(objects.info) != 0 {
		t.Fatal("late orphan escaped retry")
	}
}

func TestOriginalPlaybackOwnershipExpiryMissingAndStaleSelection(t *testing.T) {
	projects, store, _ := setup(t)
	_, p := create(t, projects)
	ctx := context.Background()
	now := time.Now().UTC()
	objects := fakeSources()
	sources := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return now })
	u, err := sources.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	b, err := sources.Confirm(ctx, "alice", u.Batch.ID, u.Batch.Sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	v := b.Sources[0]
	if _, err = sources.Playback(ctx, "bob", p.ID, v.ID, v.Fingerprint); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, v.ID, "wrong"); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	delete(objects.info, v.Key)
	if _, err = sources.AvailableBatch(ctx, "alice", b.ID); !errors.Is(err, clip.ErrSourceMissing) {
		t.Fatal("missing originals admitted", err)
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, v.ID, v.Fingerprint); !errors.Is(err, clip.ErrSourceMissing) {
		t.Fatal(err)
	}
	objects.upload(b)
	now = b.ExpiresAt.Add(-30 * time.Second)
	link, err := sources.Playback(ctx, "alice", p.ID, v.ID, v.Fingerprint)
	if err != nil || !link.ExpiresAt.Equal(b.ExpiresAt) {
		t.Fatal(link, err)
	}
	next, err := sources.Create(ctx, "alice", p.ID, manifest(2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, v.ID, v.Fingerprint); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("stale selection admitted", err)
	}
	if _, ok := objects.info[v.Key]; !ok {
		t.Fatal("replacement destroyed live originals")
	}
	if next.Batch.ID == b.ID {
		t.Fatal("selection mutated an immutable batch")
	}
}

func TestOriginalRerenderReusesFullManifestWithoutUnusedMissingPixels(t *testing.T) {
	h, p, _ := completedClip(t)
	ctx := context.Background()
	unused := h.batch.Sources[1].Key
	delete(h.objects.info, unused)
	if _, err := h.service.StartRender(ctx, "alice", p.ID, h.batch.ID, p.EditPlanRevision); err != nil {
		t.Fatal(err)
	}
	if err := runRender(t, h); err != nil {
		t.Fatal(err)
	}
	if h.objects.downloads[unused] != 0 {
		t.Fatal("rerender fetched an unused original")
	}
	b, err := h.store.GetSourceBatch(ctx, "alice", h.batch.ID)
	if err != nil || b.State != "ready" || len(b.Sources) != 2 {
		t.Fatal("rerender mutated the reusable manifest", b, err)
	}
}

func TestOriginalPermanentFenceProtectsActivePixelsAndDefeatsReleaseRenewal(t *testing.T) {
	projects, store, _ := setup(t)
	_, p := create(t, projects)
	ctx := context.Background()
	now := time.Now().UTC()
	objects := fakeSources()
	sources := clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return now })
	u, err := sources.Create(ctx, "alice", p.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(u.Batch)
	b, err := sources.Confirm(ctx, "alice", u.Batch.ID, u.Batch.Sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.LinkSourceJob(ctx, "alice", b.ID, "bound", now); err != nil {
		t.Fatal(err)
	}
	if err = sources.RevokeProject(ctx, "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := objects.info[b.Sources[0].Key]; !ok {
		t.Fatal("fence deleted actively bound pixels")
	}
	if _, err = sources.Playback(ctx, "alice", p.ID, b.Sources[0].ID, b.Sources[0].Fingerprint); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal("fence allowed new access", err)
	}
	now = now.Add(25 * time.Hour)
	if err = sources.ReleaseAttempt(ctx, "alice", "bound", now); err != nil {
		t.Fatal(err)
	}
	objects.failDelete[b.Sources[0].Key] = true
	if err = sources.Sweep(ctx); err == nil {
		t.Fatal("storage error hidden")
	}
	got, err := store.GetSourceBatch(ctx, "alice", b.ID)
	if err != nil || !got.Sources[0].ExpiresAt.Equal(b.ExpiresAt) || got.State != "cleanup_pending" {
		t.Fatal("terminal release revived fenced originals", got, err)
	}
	// Restart creates a fresh service; durable cleanup and access denial remain.
	sources = clip.NewSourceService(store, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return now })
	objects.failDelete[b.Sources[0].Key] = false
	if err = sources.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if len(objects.info) != 0 {
		t.Fatal("restart lost cleanup")
	}
	if _, err = sources.Create(ctx, "alice", p.ID, manifest(1)); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal("reselection bypassed permanent fence", err)
	}
}
