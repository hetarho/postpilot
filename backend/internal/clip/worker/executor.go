// Package worker executes immutable media tasks through consumer-owned ports.
// It has no database, provider, billing or project publication authority.
package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/localmedia"
	"github.com/postpilot/backend/internal/clip/mediacodec"
)

type Media interface {
	WithWorkspace(context.Context, string, func(clip.MediaWorkspace) error) error
	Probe(context.Context, clip.MediaWorkspace, string) (clip.MediaInfo, error)
	ProbeContainer(context.Context, clip.MediaWorkspace, string) (clip.MediaInfo, error)
	PrepareAnalysisChunksExcept(context.Context, clip.MediaWorkspace, clip.MediaSource, func(int) bool, func(clip.AnalysisChunk) error) (clip.MediaInfo, error)
}
type Artifacts interface {
	Download(context.Context, clip.MediaLeaseCredentials, string, io.Writer, int64) (int64, error)
	Upload(context.Context, clip.MediaLeaseCredentials, clip.MediaOutput, string) error
}
type Executor struct {
	media     Media
	render    clip.Renderer
	artifacts Artifacts
	cfg       clip.MediaConfig
}

func NewExecutor(m Media, r clip.Renderer, a Artifacts, cfg clip.MediaConfig) *Executor {
	return &Executor{m, r, a, cfg}
}
func (e *Executor) fetch(ctx context.Context, ws clip.MediaWorkspace, w clip.MediaWork, s clip.MediaTaskSource) (clip.MediaSource, func() error, error) {
	lease := clip.SourceLease{ID: s.ID, SourceMetadata: s.SourceMetadata, ActualBytes: s.Bytes, Key: "source/" + s.ID}
	return localmedia.Fetch(ctx, ws, lease, s.Info, e.cfg.Sources.MaxFileBytes, true, func(ctx context.Context, slot string, dst io.Writer, n int64) (int64, error) {
		return e.artifacts.Download(ctx, w.Credentials, slot, dst, n)
	})
}
func (e *Executor) Execute(ctx context.Context, w clip.MediaWork) (string, error) {
	if w.ContractVersion != clip.MediaContractVersion || w.RendererVersion != clip.MediaRendererVersion || w.AssetVersion != clip.MediaAssetVersion || clip.MediaPayloadDigest(w.Payload) != w.InputDigest {
		return "", clip.ErrMediaIncompatible
	}
	task, err := mediacodec.DecodeTask(w.Payload)
	if err != nil {
		return "", err
	}
	if err = clip.ValidateMediaTask(w.Operation, task, e.cfg); err != nil {
		return "", err
	}
	result := clip.MediaResult{Version: clip.MediaContractVersion}
	err = e.media.WithWorkspace(ctx, w.Credentials.AttemptID, func(ws clip.MediaWorkspace) error {
		switch w.Operation {
		case clip.MediaPrepare:
			return e.prepare(ctx, ws, w, task, &result)
		case clip.MediaRender:
			return e.renderTask(ctx, ws, w, task, &result)
		case clip.MediaSample:
			return e.sampleTask(ctx, ws, w, task, &result)
		default:
			return clip.ErrMediaIncompatible
		}
	})
	if err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = clip.ValidateMediaResult(w.Operation, task, result, e.cfg); err != nil {
		return "", err
	}
	return mediacodec.EncodeResult(result)
}
func (e *Executor) prepare(ctx context.Context, ws clip.MediaWorkspace, w clip.MediaWork, task clip.MediaTask, result *clip.MediaResult) error {
	var probed []clip.ProbedSource
	var bytes int64
	for _, s := range task.Sources {
		// The API selects only validated compatible observations. If every
		// interval is already observed, their frozen verified source info is
		// sufficient: no original download or decode needs repeating.
		count := (s.Info.DurationMS + e.cfg.ChunkDurationMS - 1) / e.cfg.ChunkDurationMS
		if count > 0 && len(s.ReusedChunks) == count {
			probed = append(probed, clip.ProbedSource{Metadata: s.SourceMetadata, Info: s.Info})
			if _, err := clip.ValidateProbedSources(e.cfg, probed); err != nil {
				return err
			}
			result.Sources = append(result.Sources, clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info})
			continue
		}
		original, drop, err := e.fetch(ctx, ws, w, s)
		if err != nil {
			return err
		}
		err = func() error {
			header, err := e.media.ProbeContainer(ctx, ws, original.Path)
			if err != nil {
				return err
			}
			if _, err = clip.ValidateProbedSources(e.cfg, append(probed, clip.ProbedSource{Metadata: s.SourceMetadata, Info: header})); err != nil {
				return err
			}
			original.Info = header
			info, err := e.media.PrepareAnalysisChunksExcept(ctx, ws, original, func(i int) bool { return slices.Contains(s.ReusedChunks, i) }, func(c clip.AnalysisChunk) error {
				if slices.Contains(s.ReusedChunks, c.Index) {
					return nil
				}
				if c.SourceID != s.ID || c.Fingerprint != s.Fingerprint || c.Bytes > e.cfg.PreparedMaxBytes-bytes {
					return clip.ErrInvalidMedia
				}
				out := clip.MediaOutput{Slot: clip.MediaAnalysisSlot(s.ID, c.Index), SourceID: s.ID, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS, Bytes: c.Bytes, ContentType: "video/mp4", Info: c.Info}
				out.Digest, err = FileDigest(ctx, c.Path, c.Bytes)
				if err != nil {
					return err
				}
				if err = clip.ValidateMediaOutput(w.Operation, task, out, e.cfg); err != nil {
					return err
				}
				if err = e.artifacts.Upload(ctx, w.Credentials, out, c.Path); err != nil {
					return err
				}
				result.Outputs = append(result.Outputs, out)
				bytes += out.Bytes
				return os.Remove(c.Path)
			})
			if err != nil {
				return err
			}
			if (header.DurationMS-1)/e.cfg.ChunkDurationMS != (info.DurationMS-1)/e.cfg.ChunkDurationMS {
				return clip.ErrInvalidMedia
			}
			probed = append(probed, clip.ProbedSource{Metadata: s.SourceMetadata, Info: info})
			if _, err = clip.ValidateProbedSources(e.cfg, probed); err != nil {
				return err
			}
			result.Sources = append(result.Sources, clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: info})
			return nil
		}()
		if err = errors.Join(err, drop()); err != nil {
			return err
		}
	}
	return nil
}
func (e *Executor) renderTask(ctx context.Context, ws clip.MediaWorkspace, w clip.MediaWork, task clip.MediaTask, result *clip.MediaResult) error {
	sources, plan, err := frozenInputs(task)
	if err != nil {
		return err
	}
	// The delivered file is made from these originals, so each is decoded in
	// full and must be the original the task froze. That happens the first time
	// the render loads it: one download serves the check and the cuts.
	verified := make([]bool, len(task.Sources))
	result.Sources = make([]clip.MediaVerifiedSource, len(task.Sources))
	loader, release := e.loader(ws, w, task, func(ctx context.Context, i int, original clip.MediaSource) (clip.MediaSource, error) {
		s := task.Sources[i]
		if !verified[i] {
			actual, err := e.media.Probe(ctx, ws, original.Path)
			if err != nil {
				return original, err
			}
			if !clip.SameMediaOriginal(s.Info, actual) {
				return original, clip.ErrInvalidMedia
			}
			result.Sources[i], verified[i] = clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: actual}, true
		}
		original.Info = result.Sources[i].Info
		return original, nil
	})
	defer release()
	video, err := e.render.Render(ctx, ws, plan, sources, loader)
	if err = errors.Join(err, release()); err != nil {
		return err
	}
	// Every source the task names is reported verified, one no cut drew too.
	for i, s := range task.Sources {
		if verified[i] {
			continue
		}
		if err = loader(ctx, s.ID, func(clip.MediaSource) error { return nil }); err != nil {
			return err
		}
	}
	if err = release(); err != nil {
		return err
	}
	out := clip.MediaOutput{Slot: "result", DurationMS: video.Info.DurationMS, Bytes: video.Bytes, ContentType: "video/mp4", Info: video.Info}
	out.Digest, err = FileDigest(ctx, video.Path, video.Bytes)
	if err != nil {
		return err
	}
	if err = clip.ValidateMediaOutput(w.Operation, task, out, e.cfg); err != nil {
		return err
	}
	if video.Plan != nil {
		result.Plan, err = clip.EncodeEditPlan(*video.Plan)
		if err != nil {
			return err
		}
	}
	if err = e.artifacts.Upload(ctx, w.Credentials, out, video.Path); err != nil {
		return err
	}
	result.Outputs = []clip.MediaOutput{out}
	return nil
}

// sampleTask measures a browser render's grounds over the render's own frozen
// task: the same originals and plan, the server's own sampler, and no file
// written (CLIP-192). A sample writes nothing a viewer receives, so the facts
// the task froze stand for its originals instead of a full decode, and an
// original is downloaded only when a read needs its frames — the download
// still checks its fingerprint.
func (e *Executor) sampleTask(ctx context.Context, ws clip.MediaWorkspace, w clip.MediaWork, task clip.MediaTask, result *clip.MediaResult) error {
	sampler, ok := e.render.(clip.GroundSampler)
	if !ok {
		return clip.ErrMediaIncompatible
	}
	sources, plan, err := frozenInputs(task)
	if err != nil {
		return err
	}
	loader, release := e.loader(ws, w, task, nil)
	defer release()
	grounds, err := sampler.SampleGrounds(ctx, ws, plan, sources, loader)
	if err = errors.Join(err, release()); err != nil {
		return err
	}
	for _, s := range task.Sources {
		result.Sources = append(result.Sources, clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info})
	}
	result.Grounds = grounds
	return nil
}

// frozenInputs is the plan a render task froze and the originals it names, as
// the task describes them; nothing is downloaded.
func frozenInputs(task clip.MediaTask) ([]clip.RenderSource, clip.EditPlan, error) {
	plan, err := clip.DecodeEditPlan(task.Plan)
	if err != nil {
		return nil, clip.EditPlan{}, err
	}
	plan = task.Render.Apply(plan)
	plan.HideDisclosure = task.HideDisclosure
	sources := make([]clip.RenderSource, len(task.Sources))
	for i, s := range task.Sources {
		sources[i] = clip.RenderSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info}
	}
	return sources, plan, nil
}

// loader holds one of the task's originals at a time, downloading each only
// when it is asked for. A check, when given, sees every download before a cut
// reads it.
func (e *Executor) loader(ws clip.MediaWorkspace, w clip.MediaWork, task clip.MediaTask, check func(context.Context, int, clip.MediaSource) (clip.MediaSource, error)) (clip.RenderSourceLoader, func() error) {
	return localmedia.Loader(func(ctx context.Context, id string) (clip.MediaSource, func() error, error) {
		i := slices.IndexFunc(task.Sources, func(s clip.MediaTaskSource) bool { return s.ID == id })
		if i < 0 {
			return clip.MediaSource{}, nil, clip.ErrInvalidMedia
		}
		original, drop, err := e.fetch(ctx, ws, w, task.Sources[i])
		if err != nil || check == nil {
			return original, drop, err
		}
		if original, err = check(ctx, i, original); err != nil {
			return clip.MediaSource{}, nil, errors.Join(err, drop())
		}
		return original, drop, nil
	})
}

// FileDigest never buffers the media file and refuses a non-regular/mis-sized file.
func FileDigest(ctx context.Context, path string, size int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", clip.ErrInvalidMedia
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() != size || size <= 0 {
		return "", clip.ErrInvalidMedia
	}
	h := sha256.New()
	b := make([]byte, 64<<10)
	var n int64
	for {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		count, read := f.Read(b)
		n += int64(count)
		if n > size {
			return "", clip.ErrInvalidMedia
		}
		h.Write(b[:count])
		if read == io.EOF {
			break
		}
		if read != nil {
			return "", clip.ErrInvalidMedia
		}
	}
	if n != size {
		return "", clip.ErrInvalidMedia
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
