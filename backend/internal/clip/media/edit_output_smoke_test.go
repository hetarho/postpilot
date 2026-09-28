package media

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// T456 on the real renderer (CDS-52, CDS-65, CLIP-188, CLIP-191): the edited clip — the
// project's intro and outro slots, a blank slot among them, an owner caption in a sequence style
// outside the AI set at the owner's size and place, a writer's caption crossing a cut and a cut
// no caption covers — rendered by the server into a file whose frames draw exactly that: the
// intro over its opening, nothing at all over the uncaptioned cut, the outro over its close.
// The provider side is a fixture; the drawing, the encode and the decode are real.
func TestRenderSmokeEditedRegionsAndOwnerCaption(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	t.Parallel()
	for _, c := range []struct{ ratio, pace string }{{"vertical", "steady"}, {"horizontal", "rapid"}} {
		t.Run(c.ratio+"/"+c.pace, func(t *testing.T) {
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
			err = a.WithWorkspace(t.Context(), "edited-output", func(ws clip.MediaWorkspace) error {
				path := filepath.Join(ws.Path, "source.mp4")
				if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30", "-t", "16", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", path); err != nil {
					return err
				}
				info, err := a.Probe(t.Context(), ws, path)
				if err != nil {
					return err
				}
				plan, _ := editedPlan(t, c.ratio, c.pace)
				result, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: info}}, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
					return consume(clip.MediaSource{SourceID: "source", Fingerprint: "fp", Info: info, Path: path})
				})
				if err != nil {
					return err
				}
				// V12: the delivered file holds the edited timeline.
				if result.Info.DurationMS < 14966 || result.Info.DurationMS > 15034 {
					return fmt.Errorf("delivered %d ms for a 15000 ms timeline", result.Info.DurationMS)
				}
				byID := map[string]clip.CompositionElement{}
				for _, e := range result.Elements {
					byID[e.InstanceID] = e
				}
				if byID["narration-1"].Style != "neon" || byID["narration-2"].Style != "film" {
					return fmt.Errorf("the captions were drawn restyled: %q %q", byID["narration-1"].Style, byID["narration-2"].Style)
				}
				rows := func(e clip.CompositionElement) []string {
					out := []string{}
					for _, row := range e.Rows {
						out = append(out, row.Text)
					}
					return out
				}
				if !slices.Equal(rows(byID["project-intro"]), []string{"성수 로컬", "", "저녁 영업"}) || !slices.Equal(rows(byID["project-outro"]), []string{"다시 만나요", ""}) {
					return fmt.Errorf("the regions drew other words: %v %v", rows(byID["project-intro"]), rows(byID["project-outro"]))
				}
				// Frames 30 (1 s), 165 (5.5 s) and 420 (14 s): the intro, the uncaptioned
				// second cut, the outro.
				drawnIn := func(frame int, region clip.Region) (int, error) {
					out := filepath.Join(ws.Path, fmt.Sprintf("edited-%d.png", frame))
					if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-threads", "1", "-i", result.Path, "-vf", fmt.Sprintf("trim=start_frame=%d:end_frame=%d", frame, frame+1), "-frames:v", "1", "-threads", "1", out); err != nil {
						return 0, err
					}
					img, err := readPNG(out)
					if err != nil {
						return 0, err
					}
					bounds := image.Rect(int(region.X), int(region.Y), int(region.X+region.Width), int(region.Y+region.Height)).Intersect(img.Bounds())
					count := 0
					for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
						for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
							red, green, blue, _ := img.At(x, y).RGBA()
							if red > 12000 || green > 12000 || blue < 40000 {
								count++
							}
						}
					}
					return count, nil
				}
				whole := clip.Region{Width: 1920, Height: 1920}
				for _, check := range []struct {
					frame  int
					region clip.Region
					drawn  bool
				}{
					{30, byID["project-intro"].Region, true},
					{165, whole, false},
					{420, byID["project-outro"].Region, true},
				} {
					count, err := drawnIn(check.frame, check.region)
					if err != nil {
						return err
					}
					if check.drawn != (count > 0) {
						return fmt.Errorf("frame %d drew %d pixels over %v, want drawn=%v", check.frame, count, check.region, check.drawn)
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
