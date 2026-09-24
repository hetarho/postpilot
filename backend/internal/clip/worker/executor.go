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
	var sources []clip.RenderSource
	for _, s := range task.Sources {
		original, drop, err := e.fetch(ctx, ws, w, s)
		if err != nil {
			return err
		}
		actual, err := e.media.Probe(ctx, ws, original.Path)
		if err = errors.Join(err, drop()); err != nil {
			return err
		}
		if !localmedia.SameIdentity(s.Info, actual) || s.Info.CadenceVerified && !actual.CadenceVerified {
			return clip.ErrInvalidMedia
		}
		sources = append(sources, clip.RenderSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: actual})
		result.Sources = append(result.Sources, clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: actual})
	}
	loader, release := localmedia.Loader(func(ctx context.Context, id string) (clip.MediaSource, func() error, error) {
		for i, s := range task.Sources {
			if s.ID == id {
				s.Info = sources[i].Info
				return e.fetch(ctx, ws, w, s)
			}
		}
		return clip.MediaSource{}, nil, clip.ErrInvalidMedia
	})
	defer release()
	plan, err := clip.DecodeEditPlan(task.Plan)
	if err != nil {
		return err
	}
	plan = task.Render.Apply(plan)
	plan.HideDisclosure = task.HideDisclosure
	video, err := e.render.Render(ctx, ws, plan, sources, loader)
	if err = errors.Join(err, release()); err != nil {
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
