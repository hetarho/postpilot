package clip

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *GenerationService) withSource(ctx context.Context, ws MediaWorkspace, v SourceLease, info MediaInfo, fn func(MediaSource) error) error {
	f, err := os.CreateTemp(ws.Path, "source-*"+filepath.Ext(v.Filename))
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	defer f.Close()
	n, err := s.objects.Download(ctx, v.Key, f, s.cfg.Media.Sources.MaxFileBytes)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("clip source download failed")
	}
	if n != v.Bytes || n != v.ActualBytes {
		return ErrInvalidMedia
	}
	if err = f.Close(); err != nil {
		return err
	}
	return fn(MediaSource{Path: name, SourceID: v.ID, Fingerprint: v.Fingerprint, Info: info})
}
func (s *GenerationService) probeBatch(ctx context.Context, ws MediaWorkspace, b SourceBatch, progress func(int)) ([]AnalysisSource, []MediaInfo, int, error) {
	var sources []AnalysisSource
	var infos []MediaInfo
	var probed []ProbedSource
	for i, v := range b.Sources {
		err := s.withSource(ctx, ws, v, MediaInfo{}, func(source MediaSource) error {
			info, err := s.media.Probe(ctx, ws, source.Path)
			if err != nil {
				return err
			}
			infos = append(infos, info)
			probed = append(probed, ProbedSource{Metadata: v.SourceMetadata, Info: info})
			sources = append(sources, AnalysisSource{RenderSource: RenderSource{ID: v.ID, Fingerprint: v.Fingerprint, Info: info}, Filename: v.Filename})
			return nil
		})
		if err != nil {
			return nil, nil, 0, err
		}
		progress(i + 1)
	}
	count, err := ValidateProbedSources(s.cfg.Media, probed)
	return sources, infos, count, err
}
func (s *GenerationService) uploadPath(ctx context.Context, key, path string, bytes int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != bytes || bytes <= 0 {
		return ErrInvalidMedia
	}
	if err = s.objects.Upload(ctx, key, f, bytes, "video/mp4"); err != nil {
		return errors.New("clip video upload failed")
	}
	return nil
}
func (s *GenerationService) observe(ctx context.Context, b SourceBatch, c AnalysisChunk, input AnalysisSource, model llm.ModelRef) (ChunkAnalysis, error) {
	key := SourcePrefix + url.PathEscape(b.UserID) + "/" + b.ID + "/proxy/" + newID() + ".mp4"
	// Lease precedes upload; a failed/partial upload or crash therefore remains reapable.
	if err := s.store.AddProxy(ctx, b.UserID, b.ID, key); err != nil {
		return ChunkAnalysis{}, err
	}
	defer func() {
		clean, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.CleanupTimeout)
		defer cancel()
		if err := s.objects.Delete(clean, key); err == nil {
			_ = s.store.RemoveProxy(clean, key)
		}
	}()
	info, err := os.Stat(c.Path)
	if err != nil {
		return ChunkAnalysis{}, err
	}
	if err = s.uploadPath(ctx, key, c.Path, info.Size()); err != nil {
		return ChunkAnalysis{}, err
	}
	link, err := s.objects.PresignRead(ctx, key, "", false, s.cfg.ReadTTL)
	if err != nil {
		return ChunkAnalysis{}, errors.New("clip proxy signing failed")
	}
	out, _, err := s.planner.ObserveChunk(ctx, model, ChunkInput{Source: input, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS, URL: link})
	return out, err
}

func (s *GenerationService) Sweep(ctx context.Context) error {
	batches, err := s.store.ListConsumingBatches(ctx)
	if err != nil {
		return err
	}
	for _, b := range batches {
		j, err := s.jobs.Get(ctx, b.UserID, b.JobID)
		if err != nil {
			return err
		}
		if j == nil || (j.Status != "queued" && j.Status != "running") {
			if err = s.sources.Finish(ctx, b.UserID, b.ID); err != nil {
				return err
			}
		}
	}
	if err = s.sources.Sweep(ctx); err != nil {
		return err
	}
	if err = s.media.CleanupStale(ctx, time.Now()); err != nil {
		return err
	}
	keys, err := s.store.DeletionKeys(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, ResultPrefix) {
			return ErrInvalid
		}
		if err = s.objects.Delete(ctx, key); err != nil {
			return errors.New("clip result deletion failed")
		}
		if err = s.store.RemoveDeletion(ctx, key); err != nil {
			return err
		}
	}
	objects, err := s.objects.ListResults(ctx)
	if err != nil {
		return err
	}
	refs, err := s.store.ResultKeys(ctx)
	if err != nil {
		return err
	} // complete reads before deleting any orphan
	keep := map[string]bool{}
	for _, key := range refs {
		keep[key] = true
	}
	cutoff := time.Now().Add(-s.cfg.OrphanMinAge)
	for _, object := range objects {
		if !strings.HasPrefix(object.Key, ResultPrefix) || object.Modified.IsZero() || !object.Modified.Before(cutoff) || keep[object.Key] {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(object.Key, ResultPrefix), "/")
		if len(parts) != 3 {
			continue
		}
		user, e := url.PathUnescape(parts[0])
		if e != nil {
			continue
		}
		project, e := url.PathUnescape(parts[1])
		if e != nil {
			continue
		}
		active, e := s.jobs.Active(ctx, user, project)
		if e != nil {
			return e
		}
		if active != nil {
			continue
		}
		// The job may have saved its result and become terminal since the initial
		// snapshot. Re-read after the active-job check before deleting its output.
		fresh, e := s.store.ResultKeys(ctx)
		if e != nil {
			return e
		}
		for _, key := range fresh {
			keep[key] = true
		}
		if keep[object.Key] {
			continue
		}
		if err = s.objects.Delete(ctx, object.Key); err != nil {
			return errors.New("orphan clip result deletion failed")
		}
	}
	return nil
}
func (s *GenerationService) RunSweep(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.Sweep(ctx); err != nil {
			slog.Warn("clip recovery will retry")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
