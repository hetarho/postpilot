package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestRapidLayersHaveUniquePathsCroppedExtentsAndNoMotion(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		if filepath.Base(c.Binary) != "resvg" {
			return nil, fmt.Errorf("simple style sampled footage: %s", c.Binary)
		}
		return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("png"), 0600)
	}}
	a := newAdapter(t, runner)
	r := testRenderer(t, a)
	canvas, _ := clip.ClipCanvas("vertical")
	copies, _ := clip.SplitRapid(clip.Caption{Text: "오늘은 구로디지털단지에 와보았는데요", Style: "simple", Anchor: "bottom", Align: "center"}, 120, 1420)
	cut := clip.EditCut{EndMS: 3000, Copies: copies}
	c := composed{plan: clip.EditPlan{Cuts: []clip.EditCut{cut}}, layouts: [][]copyLayout{make([]copyLayout, len(copies))}, grounds: [][]Luminance{make([]Luminance, len(copies))}}
	if err := a.WithWorkspace(t.Context(), "rapid", func(ws clip.MediaWorkspace) error {
		seen := map[string]bool{}
		l := layers{}
		for j, copy := range copies {
			layout, err := fitCopy(canvas, copy, [][]string{{copy.Text}}, map[string]clip.Region{copy.Text: {Y: -80, Width: 500, Height: 100}})
			if err != nil {
				return err
			}
			c.layouts[0][j] = layout
			path, err := r.copyLayer(t.Context(), ws, canvas, &c, 0, j, clip.MediaSource{})
			if err != nil {
				return err
			}
			if seen[path] {
				return fmt.Errorf("caption plate overwritten: %s", path)
			}
			seen[path] = true
			region := copyCrop(canvas, copy, layout, Luminance{})
			if region.Height >= float64(canvas.Height)/4 || region.X <= 0 || region.Y <= 0 {
				t.Fatal(region)
			}
			if !design.Legible(layout.Elements(0, j, copy, copy.StartMS, copy.EndMS)) {
				t.Fatal("outline contrast failed")
			}
			l.Copies = append(l.Copies, path)
			l.CopyRegions = append(l.CopyRegions, region)
		}
		graph := cutGraph(r.cfg, canvas, cut, clip.MediaInfo{}, 90, l, false)
		if strings.Contains(graph, "fade=") || strings.Contains(graph, "pow(") {
			t.Fatal(graph)
		}
		for _, window := range []string{"gte(t,0.120)*lt(t,0.420)", "gte(t,0.420)*lt(t,0.920)", "gte(t,0.920)*lt(t,1.420)"} {
			if !strings.Contains(graph, window) {
				t.Fatal(graph)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Production binary/font gate. The separately constrained local run also checks
// that the admitted 24-cue limit fits the existing 512 MiB VPS render envelope.
func TestRenderSmokeRapidCaptions(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "rapid-stress", func(ws clip.MediaWorkspace) error {
		source := filepath.Join(ws.Path, "source.mp4")
		if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30", "-t", "15", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p", source); err != nil {
			return err
		}
		info, err := a.Probe(t.Context(), ws, source)
		if err != nil {
			return err
		}
		cut := clip.Cut{ID: "one", SourceID: "source", Fingerprint: "source", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}
		for i := 0; i < 24; i++ {
			text := "오늘은"
			if i%2 == 1 {
				text = "철판 요리를 먹어요"
			}
			cut.Copies = append(cut.Copies, clip.Caption{Pace: "rapid", Text: text, Style: "simple", Anchor: "bottom", Align: "center", StartMS: i * 300, EndMS: (i + 1) * 300})
		}
		plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Styles: []string{"clean", "simple"}, Cuts: []clip.Cut{cut}}
		sources := []clip.RenderSource{{ID: "source", Fingerprint: "source", Info: info}}
		result, err := r.Render(t.Context(), ws, plan, sources, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
			return consume(clip.MediaSource{SourceID: "source", Fingerprint: "source", Path: source, Info: info})
		})
		if err != nil {
			return err
		}
		if result.Info.DurationMS != 15000 {
			return fmt.Errorf("wrong duration: %+v", result.Info)
		}
		if err := design.VerifyApproved(result.Manifest, "vertical", plan.Styles); err != nil {
			return err
		}
		// Adjacent frame 8 / 9 crosses precisely 300 ms. A later frame of the
		// second cue must retain its ink instead of fading in over several frames.
		for _, frame := range []int{8, 9, 10} {
			path := filepath.Join(ws.Path, fmt.Sprintf("frame-%d.png", frame))
			if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-threads", "2", "-ss", frameSeconds(frame, 30), "-i", result.Path, "-vf", "crop=900:170:50:1230", "-frames:v", "1", "-threads", "1", path); err != nil {
				return err
			}
		}
		count := func(frame int) (int, error) {
			img, err := readPNG(filepath.Join(ws.Path, fmt.Sprintf("frame-%d.png", frame)))
			if err != nil {
				return 0, err
			}
			n := 0
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					if r > 0xc000 && g > 0xc000 && b > 0xc000 {
						n++
					}
				}
			}
			return n, nil
		}
		before, err := count(8)
		if err != nil {
			return err
		}
		after, err := count(9)
		if err != nil {
			return err
		}
		next, err := count(10)
		if err != nil {
			return err
		}
		if before < 100 || after < before*2 || next < after*9/10 || next > after*11/10 {
			return fmt.Errorf("cue boundary/fade wrong: white pixels %d/%d/%d", before, after, next)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory.peak", "memory.events"} {
		data, _ := os.ReadFile("/sys/fs/cgroup/" + name)
		t.Logf("%s: %s", name, data)
	}
}
