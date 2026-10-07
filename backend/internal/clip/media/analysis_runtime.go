package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type analysisRuntimeManifest struct {
	Version                                           int
	Role, Profile, OS, Architecture                   string
	AcceptedOperations                                []string
	Tools                                             map[string]runtimeTool
	DecodeThreads, Concurrency, MaxFrames, MaxPackets int
	WorkspaceBytes, FileBytes                         int64
	OperationTimeoutMS                                int64
}

func AnalysisVerificationConfig(cfg clip.MediaConfig) (clip.MediaConfig, error) {
	if cfg.DecodeThreads < 1 || cfg.DecodeThreads > 2 {
		return cfg, errors.New("analysis verification CLIP_DECODE_THREADS must be 1 or 2")
	}
	cfg.WorkspaceMaxBytes = clip.AnalysisVerificationWorkspaceBytes
	cfg.PreparedMaxBytes = cfg.AnalysisMaxBytes
	cfg.OperationTimeout = clip.AnalysisVerificationTimeout
	cfg.StdoutLimit, cfg.StderrLimit = clip.AnalysisVerificationLogBytes, clip.AnalysisVerificationLogBytes
	return cfg, nil
}
func (a *Adapter) AnalysisVerificationProfile(ctx context.Context) (clip.MediaWorkerProfile, error) {
	cfg := a.cfg
	manifest := analysisRuntimeManifest{Version: 1, Role: clip.AnalysisVerificationRole, Profile: clip.AnalysisVerificationProfile, OS: runtime.GOOS, Architecture: runtime.GOARCH, AcceptedOperations: []string{string(clip.MediaVerifyAnalysis)}, Tools: map[string]runtimeTool{}, DecodeThreads: cfg.DecodeThreads, Concurrency: 1, MaxFrames: cfg.FPS * 60, MaxPackets: clip.AnalysisVerificationPacketMax, WorkspaceBytes: cfg.WorkspaceMaxBytes, FileBytes: cfg.AnalysisMaxBytes, OperationTimeoutMS: cfg.OperationTimeout.Milliseconds()}
	runner := ExecRunner{StdoutLimit: clip.AnalysisVerificationLogBytes, StderrLimit: clip.AnalysisVerificationLogBytes, WaitDelay: cfg.WaitDelay}
	for _, tool := range []struct{ name, path string }{{"ffmpeg", cfg.FFmpegPath}, {"ffprobe", cfg.FFprobePath}} {
		out, e := runner.Run(ctx, Command{Binary: tool.path, Args: []string{"-version"}})
		if e != nil {
			return clip.MediaWorkerProfile{}, errors.New("analysis verification binary unavailable")
		}
		line, _, _ := strings.Cut(string(out), "\n")
		if len(line) == 0 || len(line) > 1024 {
			return clip.MediaWorkerProfile{}, clip.ErrInvalid
		}
		f, e := os.Open(tool.path)
		if e != nil {
			return clip.MediaWorkerProfile{}, errors.New("analysis binary unreadable")
		}
		h := sha256.New()
		_, e = io.Copy(h, f)
		f.Close()
		if e != nil {
			return clip.MediaWorkerProfile{}, errors.New("analysis binary digest failed")
		}
		manifest.Tools[tool.name] = runtimeTool{Version: line, SHA256: hex.EncodeToString(h.Sum(nil))}
	}
	e := a.WithWorkspace(ctx, "analysis-capability", func(ws clip.MediaWorkspace) error {
		path := filepath.Join(ws.Path, "copy.mp4")
		_, e := runner.Run(ctx, Command{Binary: cfg.FFmpegPath, Dir: ws.Path, Args: []string{"-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=176x96:r=15", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-threads", "1", "-c:a", "aac", "-ac", "1", "-ar", strconv.Itoa(cfg.AudioRate), "-b:a", strconv.Itoa(cfg.AudioBitrate), "-movflags", "+faststart", "-y", path}})
		if e != nil {
			return errors.New("analysis capability fixture failed")
		}
		stat, e := os.Stat(path)
		if e != nil {
			return e
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		digest := sha256.Sum256(data)
		c := clip.AnalysisCopy{Slot: clip.MediaAnalysisSlot("capability", 0), SourceID: "capability", DurationMS: 1000, Width: 176, Height: 96, HasAudio: true, Bytes: stat.Size(), Digest: hex.EncodeToString(digest[:])}
		_, e = a.VerifyAnalysisCopy(ctx, ws, path, c)
		return e
	})
	if e != nil {
		return clip.MediaWorkerProfile{}, errors.New("analysis verification execution unavailable")
	}
	raw, e := json.Marshal(manifest)
	if e != nil || len(raw) > clip.MediaManifestMaxBytes {
		return clip.MediaWorkerProfile{}, clip.ErrInvalid
	}
	return clip.MediaWorkerProfile{Operation: clip.MediaVerifyAnalysis, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.AnalysisVerificationRenderer, AssetVersion: clip.AnalysisVerificationAssets, Profile: clip.AnalysisVerificationProfile, RuntimeManifest: string(raw)}, nil
}
