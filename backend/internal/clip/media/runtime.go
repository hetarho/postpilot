package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type runtimeTool struct{ Version, SHA256 string }
type runtimeManifest struct {
	Profile, OS, Architecture, Fingerprint    string
	Tools                                     map[string]runtimeTool
	Fonts                                     map[string]string
	Overlays                                  string
	EncodeThreads, DecodeThreads, Concurrency int
	WorkspaceBytes, PreparedBytes, FileBytes  int64
	OperationTimeoutMS                        int64
}

// RuntimeProfile validates actual capabilities, not merely the presence of an
// executable. No NVIDIA command is called on the validated CPU path.
func (r *Rendering) RuntimeProfile(ctx context.Context, accel string) (clip.MediaWorkerProfile, error) {
	if accel == "nvenc" {
		return clip.MediaWorkerProfile{}, errors.New("nvenc profile is not approved; use MEDIA_ACCEL=cpu or auto")
	}
	if accel != "cpu" && accel != "auto" {
		return clip.MediaWorkerProfile{}, clip.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cfg := r.media.cfg
	runner := ExecRunner{StdoutLimit: 1 << 20, StderrLimit: 16 << 10, WaitDelay: cfg.WaitDelay}
	manifest := runtimeManifest{Profile: clip.MediaCPUProfile, OS: runtime.GOOS, Architecture: runtime.GOARCH, Tools: map[string]runtimeTool{}, Fonts: map[string]string{}, Overlays: r.overlays.Digest(), EncodeThreads: cfg.EncodeThreads, DecodeThreads: cfg.DecodeThreads, Concurrency: 1, WorkspaceBytes: cfg.WorkspaceMaxBytes, PreparedBytes: cfg.PreparedMaxBytes, FileBytes: cfg.Sources.MaxFileBytes, OperationTimeoutMS: cfg.OperationTimeout.Milliseconds()}
	for _, tool := range []struct{ name, path, flag string }{{"ffmpeg", cfg.FFmpegPath, "-version"}, {"ffprobe", cfg.FFprobePath, "-version"}, {"resvg", r.cfg.ResvgPath, "--version"}} {
		version, err := runner.Run(ctx, Command{Binary: tool.path, Args: []string{tool.flag}})
		if err != nil {
			return clip.MediaWorkerProfile{}, errors.New("media binary version check failed")
		}
		line, _, _ := strings.Cut(string(version), "\n")
		if len(line) == 0 || len(line) > 1024 {
			return clip.MediaWorkerProfile{}, clip.ErrInvalid
		}
		f, err := os.Open(tool.path)
		if err != nil {
			return clip.MediaWorkerProfile{}, errors.New("media binary unreadable")
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return clip.MediaWorkerProfile{}, errors.New("media binary digest failed")
		}
		manifest.Tools[tool.name] = runtimeTool{Version: line, SHA256: hex.EncodeToString(h.Sum(nil))}
	}
	for _, listing := range []struct {
		flag     string
		required []string
	}{{"-filters", requiredFilters}, {"-encoders", requiredEncoders}, {"-decoders", requiredDecoders}, {"-muxers", requiredMuxers}} {
		out, err := runner.Run(ctx, Command{Binary: cfg.FFmpegPath, Args: []string{"-hide_banner", listing.flag}})
		if err != nil {
			return clip.MediaWorkerProfile{}, errors.New("media capabilities unavailable")
		}
		names := parseListing(string(out))
		for _, name := range listing.required {
			if !names[name] {
				return clip.MediaWorkerProfile{}, errors.New("media binary lacks required CPU capability")
			}
		}
	}
	if err := r.checkCPUExecution(ctx, runner); err != nil {
		return clip.MediaWorkerProfile{}, err
	}
	for _, font := range bundledFonts {
		manifest.Fonts[font.Key] = font.SHA256
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return clip.MediaWorkerProfile{}, err
	}
	sum := sha256.Sum256(raw)
	manifest.Fingerprint = hex.EncodeToString(sum[:])
	raw, err = json.Marshal(manifest)
	if err != nil || len(raw) > clip.MediaManifestMaxBytes {
		return clip.MediaWorkerProfile{}, clip.ErrInvalid
	}
	return clip.MediaWorkerProfile{ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profile: clip.MediaCPUProfile, RuntimeManifest: string(raw)}, nil
}

// An executable listing is insufficient: exercise the encoder and decoder with
// three synthetic frames inside the same bounded, disposable workspace.
func (r *Rendering) checkCPUExecution(ctx context.Context, runner ExecRunner) error {
	return r.media.WithWorkspace(ctx, "cpu-capability", func(ws clip.MediaWorkspace) error {
		cfg := r.media.cfg
		path := filepath.Join(ws.Path, "cpu-probe.mp4")
		_, err := runner.Run(ctx, Command{Binary: cfg.FFmpegPath, Dir: ws.Path, Args: []string{
			"-hide_banner", "-nostdin", "-v", "error", "-filter_threads", "1", "-f", "lavfi", "-i", "color=c=blue:s=176x96:r=30", "-frames:v", "3", "-an",
			"-c:v", "libx264", "-preset", "ultrafast", "-crf", "28", "-pix_fmt", "yuv420p", "-threads", strconv.Itoa(cfg.EncodeThreads), "-y", path,
		}})
		if err != nil {
			return errors.New("CPU H.264 execution check failed")
		}
		out, err := runner.Run(ctx, Command{Binary: cfg.FFmpegPath, Dir: ws.Path, Args: []string{
			"-hide_banner", "-nostdin", "-v", "error", "-filter_threads", "1", "-threads", strconv.Itoa(cfg.DecodeThreads), "-i", path, "-frames:v", "1", "-an", "-threads", "1", "-c:v", "png", "-f", "image2pipe", "pipe:1",
		}})
		if err != nil {
			return errors.New("CPU H.264 decode check failed")
		}
		frame, err := png.Decode(bytes.NewReader(out))
		if err != nil || frame.Bounds().Dx() != 176 || frame.Bounds().Dy() != 96 {
			return errors.New("CPU H.264 decode check failed")
		}
		return nil
	})
}
