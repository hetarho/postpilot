package clip

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *GenerationService) withSource(ctx context.Context, ws MediaWorkspace, v SourceLease, info MediaInfo, fn func(MediaSource) error) (err error) {
	if v.Bytes <= 0 || v.Bytes > s.cfg.Media.Sources.MaxFileBytes || v.ActualBytes != v.Bytes {
		return ErrInvalidMedia
	}
	if ws.CheckCapacity == nil {
		return ErrWorkspaceLimit
	}
	if err := ws.CheckCapacity(v.Bytes); err != nil {
		return err
	}
	f, err := os.CreateTemp(ws.Path, "source-*"+filepath.Ext(v.Filename))
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() {
		if remove := os.Remove(name); remove != nil && !os.IsNotExist(remove) {
			err = errors.Join(err, remove)
		}
	}()
	defer f.Close()
	w := &sourceWriter{ctx: ctx, file: f, remaining: v.Bytes, capacity: ws.CheckCapacity}
	n, err := s.objects.Download(ctx, v.Key, w, v.Bytes)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if w.err != nil {
			return w.err
		}
		return errors.New("clip source download failed")
	}
	if w.err != nil {
		return w.err
	}
	if n != v.Bytes || w.remaining != 0 {
		return ErrInvalidMedia
	}
	if err = f.Close(); err != nil {
		return err
	}
	return fn(MediaSource{Path: name, SourceID: v.ID, Fingerprint: v.Fingerprint, Info: info})
}

type sourceWriter struct {
	ctx       context.Context
	file      *os.File
	remaining int64
	capacity  func(int64) error
	err       error
}

func (w *sourceWriter) Write(p []byte) (int, error) {
	if w.err == nil {
		w.err = w.ctx.Err()
	}
	if w.err == nil && int64(len(p)) > w.remaining {
		w.err = ErrInvalidMedia
	}
	if w.err == nil {
		w.err = w.capacity(int64(len(p)))
	}
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.file.Write(p)
	w.remaining -= int64(n)
	w.err = err
	return n, err
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
func (s *GenerationService) observe(ctx context.Context, c AnalysisChunk, input AnalysisSource, policy llm.CallPolicy) (out ChunkAnalysis, err error) {
	defer func() { err = errors.Join(err, os.Remove(c.Path)) }()
	video := llm.InlineVideo{MIME: "video/mp4", Size: c.Bytes, DurationMS: int64(c.Info.ContainerDurationMS), Sampling: llm.VideoSamplingFixed, Open: func(ctx context.Context) (io.ReadCloser, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := os.Open(c.Path)
		if err != nil {
			return nil, ErrInvalidMedia
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() != c.Bytes {
			_ = f.Close()
			return nil, ErrInvalidMedia
		}
		return f, nil
	}}
	out, _, err = s.planner.ObserveChunk(ctx, policy.Ref, ChunkInput{Source: input, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS, Video: video, Policy: policy})
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
