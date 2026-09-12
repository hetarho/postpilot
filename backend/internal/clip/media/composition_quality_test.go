package media

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// Explicit, offline review gate. The ordinary smoke remains mandatory for the image;
// this gate additionally exports a corpus for the actual browser draft compositor.
func TestCompositionQualityReview(t *testing.T) {
	root := os.Getenv("CLIP_QUALITY_REVIEW_DIR")
	if root == "" {
		t.Skip("set a writable local review directory in the production media image")
	}
	for _, file := range []string{"memory.max", "cpu.max"} {
		b, err := os.ReadFile("/sys/fs/cgroup/" + file)
		if err != nil {
			t.Fatal(err)
		}
		value := strings.TrimSpace(string(b))
		if file == "memory.max" && value != "536870912" || file == "cpu.max" && strings.Join(strings.Fields(value), " ") != "200000 100000" {
			t.Fatal("review requires 512 MiB and 2 CPUs", file, value)
		}
	}
	t.Cleanup(func() { b, _ := os.ReadFile("/sys/fs/cgroup/memory.peak"); t.Logf("peak memory bytes: %s", b) })
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		paces := []string{"steady", "rapid"}
		if ratio == "vertical" {
			paces = append(paces, "steady-extended")
		}
		for _, variant := range paces {
			t.Run(ratio+"-"+variant, func(t *testing.T) {
				pace := strings.TrimSuffix(variant, "-extended")
				duration := 15000
				if variant == "steady-extended" {
					duration = 15600
				}
				folder := filepath.Join(root, ratio+"-"+variant)
				if err := os.MkdirAll(folder, 0755); err != nil {
					t.Fatal(err)
				}
				cfg := mediaConfig(t)
				cfg.OperationTimeout = 15 * time.Minute
				runner := &qualityReviewRunner{Runner: ExecRunner{StdoutLimit: cfg.StdoutLimit, StderrLimit: cfg.StderrLimit, WaitDelay: cfg.WaitDelay}, root: cfg.WorkRoot}
				a, err := New(cfg, runner)
				if err != nil {
					t.Fatal(err)
				}
				renderer, err := NewRenderer(a, renderConfig(t))
				if err != nil {
					t.Fatal(err)
				}
				err = a.WithWorkspace(t.Context(), "quality", func(ws clip.MediaWorkspace) error {
					var sources []clip.RenderSource
					paths := map[string]string{}
					for i, id := range []string{"a", "b"} {
						path := filepath.Join(ws.Path, "source-"+id+".mp4")
						pattern := filepath.Join(ws.Path, "pattern-"+id+".png")
						if err := writeQualityPattern(pattern, i); err != nil {
							return err
						}
						if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-loop", "1", "-framerate", "30", "-i", pattern, "-t", "8", "-an", "-c:v", "libx264", "-threads", "1", "-preset", "ultrafast", "-pix_fmt", "yuv420p", path); err != nil {
							return err
						}
						info, err := a.Probe(t.Context(), ws, path)
						if err != nil {
							return err
						}
						sources = append(sources, clip.RenderSource{ID: id, Fingerprint: strings.Repeat(id, 64), Info: info})
						paths[id] = path
						if err := copyQualityFile(path, filepath.Join(folder, "source-"+id+".mp4")); err != nil {
							return err
						}
					}
					body := `<clip version="1" styles="simple clean" pace="` + pace + `"><repeat for="scenes"><scene id="scene"><text id="caption" kind="fixed" role="caption" style="simple" position="bottom" basis="cut" start="0" end="7.4">바로 다음</text></scene></repeat><text id="across" kind="fixed" role="info" position="top" basis="output-start" start="1" end="14">두 장면의 기록</text><text id="ending" kind="fixed" role="caption" style="clean" position="upper_mid" basis="output-end" start="-2" end="0">직접 쓴 마무리</text></clip>`
					cuts := []composition.Cut{{ID: "cut-a", SectionID: "scene", SourceID: "a", EndMS: 7600}, {ID: "cut-b", SectionID: "scene", SourceID: "b", EndMS: 7600, TransitionMS: 200}}
					for i := range cuts {
						cuts[i].EndMS = (duration + 200) / 2
					}
					doc, problem := composition.Parse(body, renderer.cfg.Composition)
					if problem != nil {
						return problem
					}
					resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: cuts}, renderer.cfg.Composition, 30000)
					if problem != nil {
						return problem
					}
					plan := clip.EditPlan{Ratio: ratio, DurationMS: duration, Portable: &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: 1, Body: body}, Cuts: cuts}}
					for i, c := range cuts {
						plan.Cuts = append(plan.Cuts, clip.Cut{ID: c.ID, SourceID: c.SourceID, Fingerprint: sources[i].Fingerprint, EndMS: c.EndMS, TransitionMS: c.TransitionMS, Focal: clip.Point{X: []float64{.25, .75}[i], Y: .5}})
					}
					for _, e := range resolved.Elements {
						text := clip.PortableText{Resolved: e, Pace: pace}
						if pace == "rapid" && e.Element.ID == "caption" {
							text.OwnerEdited = true
							text.Phrases = []clip.EditablePhrase{{Text: "바로", StartMS: 0, EndMS: 300}, {Text: "다음", StartMS: 300, EndMS: 600}}
						}
						// Global exact text is independent of the per-cut caption pace.
						if e.Element.ID != "caption" {
							text.Pace = "steady"
						}
						plan.Portable.Elements = append(plan.Portable.Elements, text)
					}
					runner.lossy = 0
					result, err := renderer.Render(t.Context(), ws, plan, sources, func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
						for _, source := range sources {
							if source.ID == id {
								return consume(clip.MediaSource{SourceID: id, Fingerprint: source.Fingerprint, Info: source.Info, Path: paths[id]})
							}
						}
						return fmt.Errorf("unknown review source %s", id)
					})
					if err != nil {
						return err
					}
					if runner.lossy != 1 || result.Info.DurationMS != duration || result.Info.HasAudio || len(result.Elements) != 4 {
						return fmt.Errorf("quality output contract: %d encodes, %+v, %d elements", runner.lossy, result.Info, len(result.Elements))
					}
					if err := copyQualityFile(result.Path, filepath.Join(folder, "result.mp4")); err != nil {
						return err
					}
					prepared, err := renderer.PreparePreview(t.Context(), *result.Plan, sources, nil, 0, clip.PreviewConfig{MaxAssets: 8, MaxAssetBytes: 512 << 10, MaxResponseBytes: 4 << 20, Timeout: 5 * time.Second})
					if err != nil {
						return err
					}
					if prepared.NextOffset != -1 {
						return fmt.Errorf("review asset fixture unexpectedly paged")
					}
					frames := []int{0, 1, 3, 8, 9, 10, 17, 18, 29, 30, 60, 221, 222, 223, 224, 225, 226, 227, 228, 389, 390, 420, 448, 449}
					if duration != 15000 {
						frames = append(frames, 408, 466, 467)
					}
					for _, frame := range frames {
						path := filepath.Join(folder, fmt.Sprintf("export-%03d.png", frame))
						if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-y", "-v", "error", "-threads", "1", "-i", result.Path, "-vf", fmt.Sprintf("trim=start_frame=%d:end_frame=%d", frame, frame+1), "-frames:v", "1", "-threads", "1", path); err != nil {
							return err
						}
					}
					artifact := struct {
						Ratio, Pace        string
						Plan               clip.CorrectionPlan
						Sources            []clip.RenderSource
						Preview            clip.PreparedPreview
						Elements           []clip.CompositionElement
						Frames             []int
						LossyEncodes       int
						PeakWorkspaceBytes int64
					}{ratio, pace, clip.CorrectionFromPlan(*result.Plan), sources, prepared, result.Elements, frames, runner.lossy, runner.peakDisk}
					encoded, err := json.MarshalIndent(artifact, "", "  ")
					if err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(folder, "review.json"), encoded, 0644); err != nil {
						return err
					}
					t.Logf("%s/%s: %d assets, %d frames, %d final lossy encode, peak workspace %d bytes", ratio, pace, len(prepared.Assets), len(frames), runner.lossy, runner.peakDisk)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func copyQualityFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0644)
}

func writeQualityPattern(path string, variant int) error {
	img := image.NewNRGBA(image.Rect(0, 0, 1280, 720))
	background := []color.RGBA{{0x22, 0x44, 0x66, 255}, {0x66, 0x44, 0x22, 255}}[variant]
	draw.Draw(img, img.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)
	for _, mark := range []struct {
		rect  image.Rectangle
		color color.RGBA
	}{
		{image.Rect(0, 0, 320, 720), color.RGBA{0x88, 0x33, 0x44, 255}},
		{image.Rect(960, 0, 1280, 720), color.RGBA{0x33, 0x88, 0x44, 255}},
		{image.Rect(560, 320, 720, 400), color.RGBA{0x99, 0x88, 0x22, 255}},
	} {
		draw.Draw(img, mark.rect, &image.Uniform{C: mark.color}, image.Point{}, draw.Src)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, img)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

type qualityReviewRunner struct {
	Runner
	mu       sync.Mutex
	root     string
	lossy    int
	peakDisk int64
}

func (r *qualityReviewRunner) Run(ctx context.Context, c Command) ([]byte, error) {
	// The composition tree deliberately uses lossless H.264 4:4:4 nodes;
	// only the final positive-CRF encode discards image information.
	for i, arg := range c.Args {
		if arg == "-crf" && i+1 < len(c.Args) && c.Args[i+1] != "0" {
			r.mu.Lock()
			r.lossy++
			r.mu.Unlock()
		}
	}
	out, err := r.Runner.Run(ctx, c)
	var size int64
	_ = filepath.WalkDir(r.root, func(path string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				size += info.Size()
			}
		}
		return nil
	})
	r.mu.Lock()
	r.peakDisk = max(r.peakDisk, size)
	r.mu.Unlock()
	return out, err
}
