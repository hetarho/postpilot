package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/localmedia"
	"github.com/postpilot/backend/internal/llm"
)

func (s *GenerationService) withSource(ctx context.Context, ws clip.MediaWorkspace, v clip.SourceLease, info clip.MediaInfo, fn func(clip.MediaSource) error) error {
	source, release, err := s.fetchSource(ctx, ws, v, info)
	if err != nil {
		return err
	}
	return errors.Join(fn(source), release())
}

func (s *GenerationService) fetchSource(ctx context.Context, ws clip.MediaWorkspace, v clip.SourceLease, info clip.MediaInfo) (clip.MediaSource, func() error, error) {
	return localmedia.Fetch(ctx, ws, v, info, s.cfg.Media.Sources.MaxFileBytes, false, s.objects.Download)
}

func (s *GenerationService) renderLoader(ws clip.MediaWorkspace, resolve func(string) (clip.SourceLease, clip.MediaInfo, bool), verifyRetained ...bool) (clip.RenderSourceLoader, func() error) {
	return localmedia.Loader(func(ctx context.Context, id string) (clip.MediaSource, func() error, error) {
		lease, info, ok := resolve(id)
		if !ok {
			return clip.MediaSource{}, nil, clip.ErrNotFound
		}
		source, drop, err := s.fetchSource(ctx, ws, lease, info)
		if err != nil {
			return source, nil, err
		}
		if len(verifyRetained) > 0 && verifyRetained[0] {
			actual, err := s.media.Probe(ctx, ws, source.Path)
			if err == nil && !localmedia.SameIdentity(info, actual) {
				err = clip.ErrInvalidMedia
			}
			if err != nil {
				return clip.MediaSource{}, nil, errors.Join(err, drop())
			}
			source.Info = actual
		}
		return source, drop, nil
	})
}

func (s *GenerationService) probeBatch(ctx context.Context, ws clip.MediaWorkspace, b clip.SourceBatch, progress func(int)) ([]clip.AnalysisSource, []clip.MediaInfo, int, error) {
	var sources []clip.AnalysisSource
	var infos []clip.MediaInfo
	var probed []clip.ProbedSource
	for i, v := range b.Sources {
		err := s.withSource(ctx, ws, v, clip.MediaInfo{}, func(source clip.MediaSource) error {
			info, err := s.media.Probe(ctx, ws, source.Path)
			if err != nil {
				return err
			}
			infos = append(infos, info)
			probed = append(probed, clip.ProbedSource{Metadata: v.SourceMetadata, Info: info})
			sources = append(sources, clip.AnalysisSource{RenderSource: clip.RenderSource{ID: v.ID, Fingerprint: v.Fingerprint, Info: info}, Filename: v.Filename})
			return nil
		})
		if err != nil {
			return nil, nil, 0, err
		}
		progress(i + 1)
	}
	count, err := clip.ValidateProbedSources(s.cfg.Media, probed)
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
		return clip.ErrInvalidMedia
	}
	if err = s.objects.Upload(ctx, key, f, bytes, "video/mp4"); err != nil {
		return errors.New("clip video upload failed")
	}
	return nil
}

func (s *GenerationService) observe(ctx context.Context, c clip.AnalysisChunk, input clip.AnalysisSource, policy llm.CallPolicy, language string) (out clip.ChunkAnalysis, err error) {
	defer func() { err = errors.Join(err, os.Remove(c.Path)) }()
	video := llm.InlineVideo{MIME: "video/mp4", Size: c.Bytes, DurationMS: int64(c.Info.ContainerDurationMS), Sampling: llm.VideoSamplingFixed, Open: func(ctx context.Context) (io.ReadCloser, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := os.Open(c.Path)
		if err != nil {
			return nil, clip.ErrInvalidMedia
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() != c.Bytes {
			_ = f.Close()
			return nil, clip.ErrInvalidMedia
		}
		return f, nil
	}}
	out, _, err = s.planner.ObserveChunk(ctx, policy.Ref, clip.ChunkInput{Source: input, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS, Video: video, Policy: policy, Language: language})
	return out, err
}

func (s *GenerationService) Sweep(ctx context.Context) error {
	if s.finisher != nil {
		if err := s.finisher.Recover(ctx); err != nil {
			return err
		}
	}
	batches, err := s.store.ListConsumingBatches(ctx)
	if err != nil {
		return err
	}
	for _, b := range batches {
		j, err := s.jobs.Get(ctx, b.UserID, b.JobID)
		if err != nil {
			return err
		}
		if j != nil && j.FinishedAt != nil && (j.Status != "queued" && j.Status != "running") {
			if err = s.sources.ReleaseAttempt(ctx, b.UserID, b.JobID, *j.FinishedAt); err != nil {
				return err
			}
		}
	}
	if err = s.sources.Sweep(ctx); err != nil {
		return err
	}
	if err = s.media.CleanupStale(ctx, s.now()); err != nil {
		return err
	}
	keys, err := s.store.DeletionKeys(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, clip.ResultPrefix) {
			return clip.ErrInvalid
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
	cutoff := s.now().Add(-s.cfg.OrphanMinAge)
	for _, object := range objects {
		if !strings.HasPrefix(object.Key, clip.ResultPrefix) || object.Modified.IsZero() || !object.Modified.Before(cutoff) || keep[object.Key] {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(object.Key, clip.ResultPrefix), "/")
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
		if id, browser := strings.CutPrefix(parts[2], "browser-"); browser && strings.HasSuffix(id, ".mp4") {
			renders, ok := s.store.(clip.BrowserUploadStore)
			if !ok {
				return clip.ErrRenderUnavailable
			}
			id = strings.TrimSuffix(id, ".mp4")
			r, err := renders.GetBrowserRender(ctx, user, id)
			if err != nil && !errors.Is(err, clip.ErrNotFound) {
				return err
			}
			if err == nil {
				if r.ResultKey() != object.Key {
					continue
				}
				// Claim deletion using the same writer as completion. Whichever
				// wins fences the other, even after the reference snapshot above.
				cancelled, err := renders.CancelBrowserRender(ctx, user, id, s.now())
				if err != nil && !errors.Is(err, clip.ErrNotFound) {
					return err
				}
				if err == nil && !cancelled {
					// Completion won. It can never promote this identity again,
					// so a fresh reference protects its current file without
					// retaining a superseded file recreated by a delayed PUT.
					refs, err := s.store.ResultKeys(ctx)
					if err != nil {
						return err
					}
					live := false
					for _, key := range refs {
						live = live || key == object.Key
					}
					if live {
						continue
					}
				}
			}
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
