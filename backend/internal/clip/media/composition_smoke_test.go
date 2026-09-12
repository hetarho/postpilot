package media

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// Production binaries, fonts and filters: source transitions happen before the
// independent output overlay, including frame-exact output-relative boundaries.
func TestRenderSmokeComposition(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	for _, variant := range []string{"vertical", "horizontal", "square", "converted"} {
		t.Run(variant, func(t *testing.T) {
			ratio := variant
			if variant == "converted" {
				ratio = "vertical"
			}
			cfg := mediaConfig(t)
			cfg.OperationTimeout = 15 * time.Minute
			a, err := New(cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			r, err := NewRenderer(a, renderConfig(t))
			if err != nil {
				t.Fatal(err)
			}
			err = a.WithWorkspace(t.Context(), "declared-frames", func(ws clip.MediaWorkspace) error {
				path := filepath.Join(ws.Path, "source.mp4")
				if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "16", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", path); err != nil {
					return err
				}
				info, err := a.Probe(t.Context(), ws, path)
				if err != nil {
					return err
				}
				body := `<clip version="1" styles="clean bold"><text id="disclosure" kind="fixed" role="badge" position="header" basis="output-start" start="1" end="14">제작비 일부 지원</text><text id="first" kind="fixed" role="info" position="bottom" basis="output-start" start="2" end="7">A 9,900원</text><text id="second" kind="fixed" role="info" position="bottom" basis="output-start" start="8" end="13">B 12,000원</text><text id="throughout" kind="fixed" role="caption" style="bold" position="upper_mid" basis="whole">끝까지 표시</text></clip>`
				plan := declaredPlan(t, body, ratio)
				plan.Disclosure, plan.Hook, plan.Preset, plan.CTA = "ad", "must not appear", "restaurant", "profile"
				plan.Cuts = []clip.Cut{{ID: "a", SourceID: "source", Fingerprint: "fp", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}}, {ID: "b", SourceID: "source", Fingerprint: "fp", StartMS: 7600, EndMS: 15200, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}}}
				plan.Portable.Cuts = []composition.Cut{{ID: "a", SourceID: "source", EndMS: 7600}, {ID: "b", SourceID: "source", StartMS: 7600, EndMS: 15200, TransitionMS: 200}}
				if variant == "converted" {
					plan.Portable = nil
					plan.Hook, plan.CTA, plan.Preset = "", "", ""
					plan.Cuts[0].Copies = []clip.Copy{{Text: "이전에 저장한 문장", Style: "clean", Anchor: "bottom", Align: "center", StartMS: 1000, EndMS: 6000}}
					plan.Portable, err = clip.FreezeLegacyPlan(clip.Project{Disclosure: "ad"}, plan, clip.Recipe{CopyStyles: []string{"clean"}}, r.cfg.Composition)
					if err != nil {
						return err
					}
				}
				result, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: info}}, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
					return consume(clip.MediaSource{SourceID: "source", Fingerprint: "fp", Info: info, Path: path})
				})
				if err != nil {
					return err
				}
				if result.Plan == nil || result.Info.DurationMS != 15000 {
					return fmt.Errorf("unexpected native result: %+v", result)
				}
				if variant == "converted" {
					found := false
					region := clip.Region{}
					for _, e := range result.Elements {
						if e.Text == "이전에 저장한 문장" && e.StartMS == 1000 && e.EndMS == 6000 {
							found = true
							region = e.Region
						}
					}
					if !found || len(result.Elements) != 2 {
						return fmt.Errorf("converted content changed: %+v", result.Elements)
					}
					for _, second := range []int{0, 2, 7} {
						output := filepath.Join(ws.Path, fmt.Sprintf("converted-%d.png", second))
						if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-ss", fmt.Sprint(second), "-i", result.Path, "-frames:v", "1", "-threads", "1", output); err != nil {
							return err
						}
						img, err := readPNG(output)
						if err != nil {
							return err
						}
						visible := scan(img, region, func(red, green, blue, alpha uint32) bool { return red > 50000 && green > 50000 && blue > 50000 })
						if visible != (second == 2) {
							return fmt.Errorf("converted caption exposure at %d s", second)
						}
					}
					return nil
				}
				if len(result.Elements) != 4 {
					return fmt.Errorf("unexpected hidden elements: %d", len(result.Elements))
				}
				badge := result.Elements[0].Region
				platePoint := image.Pt(int(badge.X+badge.Width/2), int(badge.Y+2))
				var middle uint32
				for _, frame := range []int{29, 30, 150, 223, 224, 225, 226, 227, 419, 420} {
					out := filepath.Join(ws.Path, fmt.Sprintf("frame-%d.png", frame))
					// Decode by integer frame, avoiding input-seek rounding in the assertion.
					if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-threads", "1", "-i", result.Path, "-vf", fmt.Sprintf("trim=start_frame=%d:end_frame=%d", frame, frame+1), "-frames:v", "1", "-threads", "1", out); err != nil {
						return err
					}
					img, err := readPNG(out)
					if err != nil {
						return err
					}
					red, _, blue, _ := img.At(platePoint.X, platePoint.Y).RGBA()
					visible := frame >= 30 && frame < 420
					if visible && blue > 30000 || !visible && (blue < 55000 || red > 3000) {
						return fmt.Errorf("badge boundary frame %d: red=%d blue=%d", frame, red, blue)
					}
					if frame == 150 {
						middle = blue
					}
					if frame >= 223 && frame <= 227 && math.Abs(float64(blue)-float64(middle)) > 2500 {
						return fmt.Errorf("persistent badge opacity changed at cut transition: %d/%d", blue, middle)
					}
					if err := os.Remove(out); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
