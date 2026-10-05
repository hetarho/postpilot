package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// This image gate executes the sixty-second boundary through the production
// CPU encoder and decoder. The longer timeline is refused before any encode.
func TestRenderSmokeSixtySecondBound(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
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
	err = a.WithWorkspace(t.Context(), "sixty-second-bound", func(ws clip.MediaWorkspace) error {
		path := filepath.Join(ws.Path, "source.mp4")
		if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=640x360:r=30", "-t", "61", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", path); err != nil {
			return err
		}
		info, err := a.Probe(t.Context(), ws, path)
		if err != nil {
			return err
		}
		refs := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: info}}
		plan := clip.EditPlan{Ratio: "vertical", DurationMS: 60000, Disclosure: "ad", Cuts: []clip.EditCut{{ID: "one", SourceID: "source", Fingerprint: "fp", EndMS: 60000, Focal: clip.Point{X: .5, Y: .5}}}}
		if err := clip.ValidateEditPlan(renderConfig(t), plan, refs); err != nil {
			return err
		}
		result, err := r.Render(t.Context(), ws, footagePlan(t, plan, `<clip version="1"/>`), refs, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
			return consume(clip.MediaSource{SourceID: "source", Fingerprint: "fp", Info: info, Path: path})
		})
		if err != nil {
			return renderFailure(err)
		}
		if result.Info.DurationMS < 59966 || result.Info.DurationMS > 60034 {
			return fmt.Errorf("60-second output measured %d ms", result.Info.DurationMS)
		}
		plan.DurationMS = 60001
		plan.Cuts[0].EndMS = 60001
		if err := clip.ValidateEditPlan(renderConfig(t), plan, refs); err == nil {
			return fmt.Errorf("over-60-second output was admitted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
