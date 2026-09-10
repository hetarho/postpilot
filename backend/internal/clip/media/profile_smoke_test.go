package media

import (
	"context"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

//go:embed testdata/speech.m4a
var syntheticSpeech []byte

func TestMediaProfileSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real binaries inside the nonroot runtime")
	}
	if os.Getuid() == 0 {
		t.Fatal("media smoke must run as nonroot")
	}
	t.Cleanup(func() {
		for _, name := range []string{"memory.peak", "memory.max", "cpu.max", "cpu.stat"} {
			if data, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", name)); err == nil {
				t.Logf("runtime %s: %s", name, data)
			}
		}
	})
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 5 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, vfr := range []bool{false, true} {
		t.Run(map[bool]string{false: "noisy-60s-speech", true: "variable-frame-rate-speech"}[vfr], func(t *testing.T) {
			start := time.Now()
			var preparation, rendering time.Duration
			stopSampling := sampleProfileDisk(cfg.WorkRoot)
			defer stopSampling()
			err := a.WithWorkspace(t.Context(), "profile", func(ws clip.MediaWorkspace) error {
				// Eight deterministic noisy images loop as a moving high-entropy
				// source. The production binary needs no extra synthetic filters.
				rng := rand.New(rand.NewPCG(84, 20260910))
				for n := 0; n < 8; n++ {
					img := image.NewRGBA(image.Rect(0, 0, 720, 404))
					for y := 0; y < 404; y++ {
						for x := 0; x < 720; x++ {
							img.SetRGBA(x, y, color.RGBA{uint8(rng.Uint32()), uint8(rng.Uint32()), uint8(rng.Uint32()), 255})
						}
					}
					f, err := os.Create(filepath.Join(ws.Path, fmt.Sprintf("frame-%02d.png", n)))
					if err != nil {
						return err
					}
					err = png.Encode(f, img)
					closeErr := f.Close()
					if err != nil {
						return err
					}
					if closeErr != nil {
						return closeErr
					}
				}
				speech := filepath.Join(ws.Path, "speech.m4a")
				if err := os.WriteFile(speech, syntheticSpeech, 0600); err != nil {
					return err
				}
				original := filepath.Join(ws.Path, "original.mp4")
				duration := 60000
				args := []string{"-hide_banner", "-nostdin", "-v", "error", "-filter_threads", "1", "-threads", "1", "-loop", "1", "-framerate", "30", "-i", filepath.Join(ws.Path, "frame-%02d.png"), "-stream_loop", "-1", "-i", speech}
				if vfr {
					duration = 4000
					args = append(args, "-vf", "setpts='if(lt(N,30),N/(15*TB),(2+(N-30)/30)/TB)'", "-fps_mode", "vfr")
				}
				args = append(args, "-t", seconds(duration), "-c:v", "libx264", "-preset", "ultrafast", "-crf", "18", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", "-ar", "48000", "-ac", "2", original)
				if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, args...); err != nil {
					return err
				}
				for n := 0; n < 8; n++ {
					if err := os.Remove(filepath.Join(ws.Path, fmt.Sprintf("frame-%02d.png", n))); err != nil {
						return err
					}
				}
				if err := os.Remove(speech); err != nil {
					return err
				}
				prepareStart := time.Now()
				info, err := a.Probe(t.Context(), ws, original)
				if err != nil {
					return err
				}
				if abs(info.DurationMS-duration) > 22 {
					t.Fatalf("source timing=%+v", info)
				}
				t.Logf("verified source timing: %+v", info)
				if vfr && info.FrameRateNumerator == 30*info.FrameRateDenominator {
					t.Fatal("fixture is not variable rate")
				}
				chunks := 0
				err = a.PrepareAnalysisChunks(t.Context(), ws, clip.MediaSource{Path: original, SourceID: "profile", Fingerprint: "synthetic", Info: info}, func(c clip.AnalysisChunk) error {
					chunks++
					if c.Bytes > 8<<20 || c.Info.ContainerDurationMS > 60000 || abs(c.Info.ContainerDurationMS-duration) > 67 || c.Info.AudioChannels != 1 {
						t.Fatalf("profile=%+v", c)
					}
					positions := []int{700, 3100}
					if !vfr {
						positions = append(positions, 30700, 58700)
					}
					for _, at := range positions {
						sourcePCM, err := speechWindow(t.Context(), a, ws, original, at)
						if err != nil {
							return err
						}
						proxyPCM, err := speechWindow(t.Context(), a, ws, c.Path, at)
						if err != nil {
							return err
						}
						correlation := audioCorrelation(sourcePCM, proxyPCM)
						if correlation < .85 {
							t.Fatalf("speech continuity at %d ms: correlation %.4f", at, correlation)
						}
						t.Logf("speech at %d ms: correlation %.4f", at, correlation)
					}
					t.Logf("verified proxy: %d bytes, %d ms, %dx%d, 15 FPS, mono AAC", c.Bytes, c.Info.ContainerDurationMS, c.Info.Width, c.Info.Height)
					return nil
				})
				if err != nil {
					return err
				}
				if chunks != 1 {
					t.Fatal("padding created an extra paid chunk", chunks)
				}
				preparation = time.Since(prepareStart)
				if !vfr {
					r, err := NewRenderer(a, renderConfig(t))
					if err != nil {
						return err
					}
					renderStart := time.Now()
					plan := clip.EditPlan{Ratio: "horizontal", DurationMS: 15000, Cuts: []clip.Cut{{ID: "profile-cut", SourceID: "profile", Fingerprint: "synthetic", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Style: "clean", Position: "bottom"}}}}
					result, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{{ID: "profile", Fingerprint: "synthetic", Info: info}}, func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
						return consume(clip.MediaSource{Path: original, SourceID: id, Fingerprint: "synthetic", Info: info})
					})
					if err != nil {
						return err
					}
					rendering = time.Since(renderStart)
					if result.Info.Width != 1920 || result.Info.Height != 1080 || abs(result.Info.DurationMS-15000) > 34 || !result.Info.HasAudio {
						t.Fatalf("noisy original render=%+v", result.Info)
					}
					for _, at := range []int{700, 3100, 10700} {
						sourcePCM, err := speechWindow(t.Context(), a, ws, original, at)
						if err != nil {
							return err
						}
						outputPCM, err := speechWindow(t.Context(), a, ws, result.Path, at)
						if err != nil {
							return err
						}
						correlation := audioCorrelation(sourcePCM, outputPCM)
						if correlation < .85 {
							t.Fatalf("rendered speech at %d ms correlation %.4f", at, correlation)
						}
						t.Logf("rendered speech at %d ms: correlation %.4f", at, correlation)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("elapsed: %s preparation: %s render: %s disk_peak_bytes: %d", time.Since(start), preparation, rendering, stopSampling())
		})
	}
}

// Sample while the real binaries write, including fixture construction and
// intermediate render files; a successful final file alone is not a disk bound.
func sampleProfileDisk(root string) func() int64 {
	var peak atomic.Int64
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var size int64
				_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
					if err == nil && !d.IsDir() {
						if info, e := d.Info(); e == nil {
							size += info.Size()
						}
					}
					return nil
				})
				for old := peak.Load(); size > old && !peak.CompareAndSwap(old, size); old = peak.Load() {
				}
			}
		}
	}()
	var stopped atomic.Bool
	return func() int64 {
		if stopped.CompareAndSwap(false, true) {
			close(stop)
		}
		<-done
		return peak.Load()
	}
}

func speechWindow(ctx context.Context, a *Adapter, ws clip.MediaWorkspace, path string, at int) ([]byte, error) {
	return a.run(ctx, ws, a.cfg.FFmpegPath, "-hide_banner", "-nostdin", "-v", "error", "-threads", "1", "-ss", seconds(at), "-i", path, "-t", "0.200", "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
}
func audioCorrelation(a, b []byte) float64 {
	n := min(len(a), len(b)) / 2
	if n < 2000 {
		return 0
	}
	var aa, bb, ab float64
	for i := 0; i < n; i++ {
		x, y := float64(int16(binary.LittleEndian.Uint16(a[i*2:]))), float64(int16(binary.LittleEndian.Uint16(b[i*2:])))
		aa += x * x
		bb += y * y
		ab += x * y
	}
	if aa < 1 || bb < 1 {
		return 0
	}
	return ab / math.Sqrt(aa*bb)
}
