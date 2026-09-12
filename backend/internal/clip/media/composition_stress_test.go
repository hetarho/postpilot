package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// At the actual 90 s / 100-cut ceiling, each 900 ms cut can admit two 300 ms
// phrases after the reading insets. 2,400 simultaneous inputs are separately
// covered by the graph-bound test: they cannot be 2,400 sequential readable
// phrases inside a 90 s output.
func TestRenderCompositionStress(t *testing.T) {
	if os.Getenv("CLIP_COMPOSITION_STRESS") != "1" {
		t.Skip("bounded production resource gate")
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
	for _, name := range []string{"memory.peak", "memory.events"} {
		name := name
		t.Cleanup(func() { b, _ := os.ReadFile("/sys/fs/cgroup/" + name); t.Logf("%s: %s", name, b) })
	}
	err = a.WithWorkspace(t.Context(), "composition-stress", func(ws clip.MediaWorkspace) error {
		path := filepath.Join(ws.Path, "source.mp4")
		if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30", "-t", "1", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", path); err != nil {
			return err
		}
		info, err := a.Probe(t.Context(), ws, path)
		if err != nil {
			return err
		}
		body := `<clip version="1" pace="rapid" styles="simple"><repeat for="scenes"><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></repeat></clip>`
		doc, problem := composition.Parse(body, r.cfg.Composition)
		if problem != nil {
			return problem
		}
		plan := clip.EditPlan{Ratio: "vertical", DurationMS: 90000, Portable: &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: 1, Body: body}}}
		for i := 0; i < r.cfg.MaxCuts; i++ {
			id := fmt.Sprintf("cut-%03d", i)
			plan.Cuts = append(plan.Cuts, clip.Cut{ID: id, SourceID: "source", Fingerprint: "fp", EndMS: 900, Focal: clip.Point{X: .5, Y: .5}})
			plan.Portable.Cuts = append(plan.Portable.Cuts, composition.Cut{ID: id, SectionID: "scene", SourceID: "source", EndMS: 900})
		}
		resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: plan.Portable.Cuts}, r.cfg.Composition, 30000)
		if problem != nil {
			return problem
		}
		for _, element := range resolved.Elements {
			element.Text = "오늘은 장면"
			plan.Portable.Elements = append(plan.Portable.Elements, clip.PortableText{Resolved: element, Pace: "rapid"})
		}
		result, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: info}}, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
			return consume(clip.MediaSource{SourceID: "source", Fingerprint: "fp", Info: info, Path: path})
		})
		if err != nil {
			return err
		}
		if result.Info.DurationMS != 90000 || len(result.Elements) != r.cfg.MaxCuts {
			return fmt.Errorf("lost stress timeline: %d ms / %d elements", result.Info.DurationMS, len(result.Elements))
		}
		for _, element := range result.Elements {
			if len(element.Cues) != 2 {
				return fmt.Errorf("lost rapid phrases: %+v", element)
			}
		}
		// The first instant of phrase two is ceil(420ms*30)=frame 13;
		// phrase one is still visible at frame 12. A later partition must
		// preserve its local clock (cut 9 starts at 8.1 s, frame 243).
		for _, frame := range []int{3, 4, 12, 13, 22, 24, 246, 247, 255, 256, 2673, 2674, 2682, 2683, 2694} {
			output := filepath.Join(ws.Path, fmt.Sprintf("stress-frame-%d.png", frame))
			if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-threads", "1", "-ss", fmt.Sprint(frame/30), "-i", result.Path, "-vf", fmt.Sprintf("trim=start_frame=%d:end_frame=%d", frame%30, frame%30+1), "-frames:v", "1", "-threads", "1", output); err != nil {
				return err
			}
			img, err := readPNG(output)
			if err != nil {
				return err
			}
			white := 0
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x += 2 {
					red, green, blue, _ := img.At(x, y).RGBA()
					if red > 50000 && green > 50000 && blue > 50000 {
						white++
					}
				}
			}
			local := frame % 27
			visible := local >= 4 && local < 22
			if visible && white < 20 || !visible && white > 5 {
				return fmt.Errorf("rapid boundary at frame %d: %d white pixels", frame, white)
			}
			if err := os.Remove(output); err != nil {
				return err
			}
		}
		entries, err := os.ReadDir(ws.Path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "bare-") || strings.HasPrefix(entry.Name(), "overlay-") || strings.HasPrefix(entry.Name(), "compose-") {
				return fmt.Errorf("left render intermediate %s", entry.Name())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
