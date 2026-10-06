package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
)

type AnalysisCopyMedia interface {
	WithWorkspace(context.Context, string, func(clip.MediaWorkspace) error) error
	VerifyAnalysisCopy(context.Context, clip.MediaWorkspace, string, clip.AnalysisCopy) (clip.AnalysisCopyVerification, error)
}
type AnalysisCopyDownloads interface {
	Download(context.Context, clip.MediaLeaseCredentials, string, io.Writer, int64) (int64, error)
}
type AnalysisVerifier struct {
	media    AnalysisCopyMedia
	objects  AnalysisCopyDownloads
	cfg      clip.MediaConfig
	progress atomic.Int64
}

func NewAnalysisVerifier(media AnalysisCopyMedia, objects AnalysisCopyDownloads, cfg clip.MediaConfig) *AnalysisVerifier {
	if media == nil || objects == nil {
		panic("clip: analysis verifier needs bounded copies only")
	}
	return &AnalysisVerifier{media: media, objects: objects, cfg: cfg}
}
func (v *AnalysisVerifier) Progress() int { return int(v.progress.Load()) }
func (v *AnalysisVerifier) Execute(ctx context.Context, w clip.MediaWork) (string, error) {
	v.progress.Store(0)
	if w.Operation != clip.MediaVerifyAnalysis || w.ContractVersion != clip.MediaContractVersion || w.RendererVersion != clip.AnalysisVerificationRenderer || w.AssetVersion != clip.AnalysisVerificationAssets || w.InputDigest != clip.MediaPayloadDigest(w.Payload) {
		return "", clip.ErrMediaIncompatible
	}
	var task clip.AnalysisVerificationTask
	if len(w.Payload) > clip.MediaPayloadMaxBytes || clip.StrictJSON(w.Payload, &task) != nil {
		return "", clip.ErrInvalid
	}
	if e := task.Validate(); e != nil {
		return "", e
	}
	result := clip.AnalysisVerificationResult{Version: 1, ProfileVersion: task.ProfileVersion, ManifestDigest: task.ManifestDigest}
	e := v.media.WithWorkspace(ctx, w.Credentials.AttemptID, func(ws clip.MediaWorkspace) error {
		for i, c := range task.Copies {
			if e := ctx.Err(); e != nil {
				return e
			}
			if ws.CheckCapacity != nil {
				if e := ws.CheckCapacity(c.Bytes); e != nil {
					return e
				}
			}
			path := filepath.Join(ws.Path, "copy-"+strconv.Itoa(i)+".mp4")
			e := func() error {
				f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if e != nil {
					return clip.ErrInvalidMedia
				}
				h := sha256.New()
				writer := &analysisCopyWriter{dst: io.MultiWriter(f, h), remaining: c.Bytes}
				n, e := v.objects.Download(ctx, w.Credentials, c.Slot, writer, c.Bytes)
				closeErr := f.Close()
				defer os.Remove(path)
				if e != nil {
					return e
				}
				if closeErr != nil || n != c.Bytes || writer.remaining != 0 || writer.err != nil || hex.EncodeToString(h.Sum(nil)) != c.Digest {
					return clip.ErrInvalidMedia
				}
				measured, e := v.media.VerifyAnalysisCopy(ctx, ws, path, c)
				if e != nil {
					return e
				}
				if e = clip.ValidateAnalysisCopyVerification(c, measured, v.cfg); e != nil {
					return e
				}
				result.Copies = append(result.Copies, measured)
				return nil
			}()
			if e != nil {
				return e
			}
			v.progress.Store(int64((i + 1) * 1000 / len(task.Copies)))
		}
		return nil
	})
	if e != nil {
		return "", e
	}
	if e = clip.ValidateAnalysisVerification(task, result, v.cfg); e != nil {
		return "", e
	}
	data, e := json.Marshal(result)
	if e != nil || len(data) > clip.MediaPayloadMaxBytes {
		return "", clip.ErrInvalid
	}
	return string(data), nil
}

type analysisCopyWriter struct {
	dst       io.Writer
	remaining int64
	err       error
}

func (w *analysisCopyWriter) Write(b []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if int64(len(b)) > w.remaining {
		w.err = clip.ErrInvalidMedia
		return 0, w.err
	}
	n, e := w.dst.Write(b)
	w.remaining -= int64(n)
	w.err = e
	return n, e
}
