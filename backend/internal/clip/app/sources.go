package app

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type SourceService struct {
	store   clip.SourceStore
	objects clip.ObjectStore
	config  clip.SourceConfig
	now     func() time.Time
}

func NewSourceService(store clip.SourceStore, objects clip.ObjectStore, cfg clip.SourceConfig, clocks ...func() time.Time) *SourceService {
	if cfg.MaxCount <= 0 || cfg.MaxDurationMS <= 0 || cfg.MaxFilenameChars <= 0 || cfg.MaxFileBytes <= 0 || cfg.MaxBatchBytes < cfg.MaxFileBytes || cfg.BatchTTL <= 0 || cfg.PutTTL <= 0 || cfg.PutTTL > cfg.BatchTTL || cfg.RetentionTTL <= 0 || cfg.PlaybackTTL <= 0 || len(cfg.Containers) == 0 {
		panic("clip: invalid source limits")
	}
	now := time.Now
	if len(clocks) > 1 || len(clocks) == 1 && clocks[0] == nil {
		panic("clip: invalid source clock")
	}
	if len(clocks) == 1 {
		now = clocks[0]
	}
	return &SourceService{store: store, objects: objects, config: cfg, now: now}
}

func (s *SourceService) validateManifest(manifest []clip.SourceMetadata) error {
	if len(manifest) == 0 || len(manifest) > s.config.MaxCount {
		return clip.ErrInvalid
	}
	var totalBytes int64
	totalDuration := 0
	seen := map[string]bool{}
	for _, v := range manifest {
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(v.Filename), "."))
		fingerprint, err := hex.DecodeString(v.Fingerprint)
		if !clip.BoundedText(v.Filename, 1, s.config.MaxFilenameChars) || strings.TrimSpace(v.Filename) == "" || strings.ContainsAny(v.Filename, "\x00\r\n") || !slices.Contains(s.config.Containers[ext], v.ContentType) || err != nil || len(fingerprint) != 32 || v.Fingerprint != strings.ToLower(v.Fingerprint) || seen[v.Fingerprint] || v.Bytes <= 0 || v.Bytes > s.config.MaxFileBytes || v.DurationMS <= 0 || v.DurationMS > s.config.MaxDurationMS || v.Width <= 0 || v.Height <= 0 {
			return clip.ErrInvalid
		}
		// Subtraction prevents aggregate overflow even for adversarial integer inputs.
		if v.Bytes > s.config.MaxBatchBytes-totalBytes || v.DurationMS > s.config.MaxDurationMS-totalDuration {
			return clip.ErrInvalid
		}
		totalBytes += v.Bytes
		totalDuration += v.DurationMS
		seen[v.Fingerprint] = true
	}
	return nil
}

func (s *SourceService) Create(ctx context.Context, user, project string, manifest []clip.SourceMetadata) (clip.SourceBatchUpload, error) {
	if err := s.validateManifest(manifest); err != nil {
		return clip.SourceBatchUpload{}, err
	}
	now := s.now().UTC()
	b := clip.SourceBatch{ID: newID(), UserID: user, ProjectID: project, State: "uploading", CreatedAt: now, ExpiresAt: now.Add(s.config.BatchTTL), UploadExpiresAt: now.Add(s.config.BatchTTL), PutExpiresAt: now.Add(s.config.PutTTL)}
	for _, m := range manifest {
		id := newID()
		ext := strings.ToLower(path.Ext(m.Filename))
		b.Sources = append(b.Sources, clip.SourceLease{ID: id, Key: clip.SourcePrefix + url.PathEscape(user) + "/" + b.ID + "/" + id + ext, State: "pending", SourceMetadata: m})
	}
	old, err := s.store.ReplaceSourceBatch(ctx, b)
	if err != nil {
		return clip.SourceBatchUpload{}, err
	}
	// Replacement and cleanup intent are already durable. A storage outage cannot erase
	// the old identities or make the new batch reuse one of their keys.
	for _, previous := range old {
		s.cleanupBestEffort(ctx, previous)
	}
	// The store may preserve canonical IDs for a matching saved plan on reselection.
	b, err = s.store.GetSourceBatch(ctx, user, b.ID)
	if err != nil {
		return clip.SourceBatchUpload{}, err
	}
	if b.AccessDenied || b.State != "uploading" || !s.now().Before(b.UploadExpiresAt) {
		return clip.SourceBatchUpload{}, clip.ErrSourceState
	}
	out := clip.SourceBatchUpload{Batch: b}
	for _, source := range b.Sources {
		signed, err := s.objects.PresignSource(ctx, source.Key, source.ContentType, s.config.PutTTL)
		if err != nil {
			_ = s.Discard(ctx, user, b.ID)
			return clip.SourceBatchUpload{}, errors.New("clip source signing failed") // never log a URL-bearing SDK error
		}
		out.Uploads = append(out.Uploads, clip.SourceUpload{SourceID: source.ID, SignedSourcePut: signed, ExpiresAt: now.Add(s.config.PutTTL)})
	}
	current, err := s.store.GetSourceBatch(ctx, user, b.ID)
	if err != nil {
		return clip.SourceBatchUpload{}, err
	}
	if current.AccessDenied || current.State != "uploading" || !s.now().Before(current.UploadExpiresAt) {
		return clip.SourceBatchUpload{}, clip.ErrSourceState
	}
	return out, nil
}

func (s *SourceService) Confirm(ctx context.Context, user, batchID, sourceID string) (clip.SourceBatch, error) {
	b, err := s.store.GetSourceBatch(ctx, user, batchID)
	if err != nil {
		return clip.SourceBatch{}, err
	}
	if b.AccessDenied || b.State != "uploading" && b.State != "ready" {
		return clip.SourceBatch{}, clip.ErrSourceState
	}
	for _, source := range b.Sources {
		if source.ID != sourceID {
			continue
		}
		if source.State == "ready" {
			if source.CleanupPending || !s.now().Before(source.ExpiresAt) {
				return clip.SourceBatch{}, clip.ErrSourceState
			}
			return s.store.ConfirmSourceLease(ctx, user, batchID, sourceID, source.ActualBytes, s.now())
		}
		if !s.now().Before(b.UploadExpiresAt) {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		info, err := s.objects.HeadSource(ctx, source.Key)
		if err != nil {
			return clip.SourceBatch{}, err
		}
		if info.Bytes != source.Bytes || info.ContentType != source.ContentType || info.Bytes > s.config.MaxFileBytes {
			if err := s.Discard(ctx, user, batchID); err != nil {
				return clip.SourceBatch{}, err
			}
			return clip.SourceBatch{}, clip.ErrInvalid
		}
		return s.store.ConfirmSourceLease(ctx, user, batchID, sourceID, info.Bytes, s.now())
	}
	return clip.SourceBatch{}, clip.ErrNotFound
}

func (s *SourceService) Discard(ctx context.Context, user, id string) error {
	b, err := s.store.MarkSourceCleanup(ctx, user, id, false)
	if errors.Is(err, clip.ErrNotFound) {
		return nil
	} // unknown and foreign are equally absent
	if err != nil {
		return err
	}
	s.cleanupBestEffort(ctx, b)
	return nil // deletion intent is durable; storage retries no longer need the client
}

// ReleaseAttempt is called only after the owning queue persisted a terminal outcome.
// The timestamp is the durable outcome time, never the current sweep time.
func (s *SourceService) ReleaseAttempt(ctx context.Context, user, job string, terminal time.Time) error {
	if terminal.IsZero() {
		return clip.ErrSourceState
	}
	return s.store.ReleaseSourceAttempt(ctx, user, job, terminal)
}

func (s *SourceService) PrepareProjectDelete(ctx context.Context, user, project string) error {
	batches, err := s.store.BeginProjectSourceCleanup(ctx, user, project)
	if err != nil {
		return err
	}
	for _, b := range batches {
		s.cleanupBestEffort(ctx, b)
	}
	return nil
}

// RevokeProject permanently denies new original access while active bindings protect
// pixels until their durable terminal release. Finalization uses this without deleting results.
func (s *SourceService) RevokeProject(ctx context.Context, user, project string) error {
	batches, err := s.store.RevokeProjectSources(ctx, user, project, s.now())
	if err != nil {
		return err
	}
	for _, b := range batches {
		s.cleanupBestEffort(ctx, b)
	}
	return nil
}

func (s *SourceService) cleanup(ctx context.Context, b clip.SourceBatch) error {
	partial := b.State != "cleanup_pending"
	var errs []error
	for _, key := range b.ProxyKeys {
		if err := s.objects.Delete(ctx, key); err != nil {
			errs = append(errs, errors.New("delete clip proxy failed"))
		} else if err := s.store.RemoveProxy(ctx, key); err != nil {
			errs = append(errs, err)
		}
	}
	for _, source := range b.Sources {
		if partial && !source.CleanupPending {
			continue
		}
		if err := s.objects.Delete(ctx, source.Key); err != nil {
			errs = append(errs, fmt.Errorf("delete source %s", source.ID))
		}
	}
	if len(errs) != 0 {
		return errors.Join(errs...)
	}
	if partial || s.now().Before(b.PutExpiresAt) {
		return nil
	}
	return s.store.RemoveSourceBatch(ctx, b.UserID, b.ID, s.now())
}

func (s *SourceService) cleanupBestEffort(ctx context.Context, b clip.SourceBatch) {
	if err := s.cleanup(ctx, b); err != nil {
		slog.Warn("clip source cleanup queued for retry", "batch_id", b.ID)
	}
}

func (s *SourceService) Sweep(ctx context.Context) error {
	batches, err := s.store.ReapSourceBatches(ctx, s.now())
	if err != nil {
		return err
	}
	var errs []error
	for _, b := range batches {
		if err := s.cleanup(ctx, b); err != nil {
			errs = append(errs, err)
		}
	}
	keys, err := s.objects.ListSourceKeys(ctx)
	if err != nil {
		return errors.Join(append(errs, errors.New("list clip source objects failed"))...)
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, clip.SourcePrefix) {
			continue
		}
		// Query after listing. A lease is inserted BEFORE a URL is signed, so no live
		// upload can race this absence check; source identities are never reused.
		exists, err := s.store.SourceKeyExists(ctx, key)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !exists {
			if err := s.objects.Delete(ctx, key); err != nil {
				errs = append(errs, errors.New("delete orphan clip source failed"))
			}
		}
	}
	return errors.Join(errs...)
}

func (s *SourceService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.Sweep(ctx); err != nil {
			slog.Warn("clip source sweep will retry")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
