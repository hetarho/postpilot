package media

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// perFrameGround is how the sampler read its frames before this task: one seek
// and decode per frame. The batched read has to produce the same measurement.
func perFrameGround(t *testing.T, a *Adapter, r *Rendering, ws clip.MediaWorkspace, canvas clip.Canvas, source clip.MediaSource, cut clip.EditCut, window [2]int, region clip.Region) Luminance {
	t.Helper()
	var frames []image.Image
	var regions []clip.Region
	for i, at := range sampleOffsets(cut, window) {
		path := filepath.Join(ws.Path, fmt.Sprintf("before-%d.png", i))
		args := append(r.baseArgs(), "-threads", strconv.Itoa(a.cfg.DecodeThreads), "-protocol_whitelist", "file,pipe", "-ss", seconds(at), "-i", source.Path, "-frames:v", "1",
			"-vf", coverChain(canvas, cut.Focal), "-c:v", "png", "-threads", "1", path)
		if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, args...); err != nil {
			t.Fatal(err)
		}
		frame, err := readFrame(path)
		_ = os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
		frames, regions = append(frames, frame), append(regions, region)
	}
	return measureFrames(frames, regions)
}

// The frames CDS-44 names now come from one read of the footage. What that read
// measures must be what a seek-and-decode per frame measured — on a bright
// ground, a dark one and one that swings between them (CLIP-125).
func TestSampledGroundsSurviveTheBatchedRead(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	cfg := mediaConfig(t)
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	canvas, _ := clip.ClipCanvas("vertical")
	for _, tc := range []struct{ name, footage string }{
		{"bright", "color=c=white:s=1080x1920:r=30"},
		{"dark", "color=c=black:s=1080x1920:r=30"},
		{"variance", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := a.WithWorkspace(t.Context(), "sampled-"+tc.name, func(ws clip.MediaWorkspace) error {
				footage := filepath.Join(ws.Path, "composed.mp4")
				build := append(r.baseArgs(), "-f", "lavfi", "-i", tc.footage)
				if tc.name == "variance" {
					// A white block crossing a black ground: the three frames of a
					// window land on different amounts of it, so σ is large.
					build = append(r.baseArgs(), "-f", "lavfi", "-i", "color=c=black:s=1080x1920:r=30",
						"-f", "lavfi", "-i", "color=c=white:s=1080x600:r=30",
						"-filter_complex", "[0:v][1:v]overlay=x=0:y='(H-h)*mod(t,4)/4'[v]", "-map", "[v]")
				}
				build = append(build, "-t", "6", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "0", "-threads", "2", "-pix_fmt", "yuv444p", footage)
				if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, build...); err != nil {
					return err
				}
				source := clip.MediaSource{Path: footage, Info: clip.MediaInfo{Width: canvas.Width, Height: canvas.Height, DurationMS: 6000}}
				cut := clip.EditCut{EndMS: 6000, Focal: clip.Point{X: .5, Y: .5}}
				window := declaredSampleWindow(0, 6000, 6000, r.cfg.FPS)
				region := clip.Region{X: 100, Y: 800, Width: 880, Height: 300}
				before := perFrameGround(t, a, r, ws, canvas, source, cut, window, region)
				after, err := r.sample(t.Context(), ws, canvas, source, cut, window, region, 0)
				if err != nil {
					return err
				}
				if before.Mean != after.Mean || before.Sigma != after.Sigma || before.R != after.R || before.G != after.G || before.B != after.B || !slices.Equal(before.Frames, after.Frames) {
					t.Fatalf("the batched read measured %+v, a seek per frame %+v", after, before)
				}
				if before.Scrim() != after.Scrim() || before.AccentWhite() != after.AccentWhite() || before.Hex() != after.Hex() {
					t.Fatalf("the scrim or accent decision moved: %+v vs %+v", before, after)
				}
				if tc.name == "variance" && before.Sigma <= 0 {
					t.Fatalf("the high-variance ground measured no spread: %+v", before)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
