package media

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// The production Dockerfile runs this with real binaries as distroless nonroot.
// No host ffmpeg, network, database, cloud bucket or paid model is used.
func TestMediaSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real media gate runs inside Docker")
	}
	cfg := mediaConfig(t)
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name          string
		seconds       int
		audio, rotate bool
	}{{"silent", 61, false, false}, {"audio", 2, true, false}, {"rotated", 2, false, true}} {
		t.Run(fixture.name, func(t *testing.T) {
			err := a.WithWorkspace(t.Context(), "smoke-"+fixture.name, func(ws clip.MediaWorkspace) error {
				path := filepath.Join(ws.Path, "original.mp4")
				args := []string{"-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30"}
				if fixture.audio {
					args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
				}
				args = append(args, "-t", seconds(fixture.seconds*1000), "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p")
				if fixture.audio {
					args = append(args, "-c:a", "aac")
				}
				args = append(args, path)
				if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, args...); err != nil {
					return err
				}
				if fixture.rotate {
					rotated := filepath.Join(ws.Path, "rotated.mov")
					if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-display_rotation:v:0", "90", "-i", path, "-c", "copy", rotated); err != nil {
						return err
					}
					if err := os.Remove(path); err != nil {
						return err
					}
					path = rotated
				}
				info, err := a.Probe(t.Context(), ws, path)
				if err != nil {
					return err
				}
				if info.HasAudio != fixture.audio {
					t.Fatalf("audio=%v", info.HasAudio)
				}
				width, height := 1280, 720
				if fixture.rotate {
					width, height = 720, 1280
				}
				if info.Width != width || info.Height != height || math.Abs(float64(info.DurationMS-fixture.seconds*1000)) > 50 {
					t.Fatalf("source probe=%+v", info)
				}
				var chunks []clip.AnalysisChunk
				err = a.PrepareAnalysisChunks(t.Context(), ws, clip.MediaSource{Path: path, SourceID: fixture.name, Fingerprint: fixture.name, Info: info}, func(chunk clip.AnalysisChunk) error {
					probe, err := a.Probe(t.Context(), ws, chunk.Path)
					if err != nil {
						return err
					}
					if max(probe.Width, probe.Height) > 720 || probe.FrameRateNumerator != 30*probe.FrameRateDenominator || probe.HasAudio != fixture.audio || math.Abs(float64(probe.DurationMS-chunk.DurationMS)) > 50 {
						t.Fatalf("proxy=%+v chunk=%+v", probe, chunk)
					}
					if probe.Rotation != 0 || (probe.Height > probe.Width) != fixture.rotate {
						t.Fatalf("proxy did not bake in source orientation: %+v", probe)
					}
					for _, s := range probe.Streams {
						if s.Kind == "video" && s.Codec != "h264" || s.Kind == "audio" && s.Codec != "aac" {
							t.Fatalf("codec=%+v", s)
						}
					}
					files, err := os.ReadDir(ws.Path)
					if err != nil {
						return err
					}
					if len(files) != 2 {
						t.Fatalf("disk contains more than source+proxy: %v", files)
					}
					chunks = append(chunks, chunk)
					return nil
				})
				if err != nil {
					return err
				}
				if fixture.seconds == 61 && (len(chunks) != 2 || chunks[0].DurationMS != 60000 || chunks[1].DurationMS != 1000 || chunks[1].OffsetMS != 60000) {
					t.Fatalf("61s split=%+v", chunks)
				}
				files, err := os.ReadDir(ws.Path)
				if err != nil {
					return err
				}
				if len(files) != 1 {
					t.Fatal("proxy survived callback")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := a.WithWorkspace(t.Context(), "corrupt", func(ws clip.MediaWorkspace) error {
		_, err := a.Probe(t.Context(), ws, sourceFile(t, ws))
		if err == nil {
			t.Fatal("corrupt media accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Cancelled admission never opens a workspace or invokes a binary.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := a.WithWorkspace(ctx, "cancelled", func(clip.MediaWorkspace) error { t.Fatal("cancelled callback ran"); return nil }); err == nil {
		t.Fatal("cancel ignored")
	}
	files, err := os.ReadDir(a.cfg.WorkRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), workspacePrefix) {
			t.Fatal("smoke workspace leaked")
		}
	}
}
