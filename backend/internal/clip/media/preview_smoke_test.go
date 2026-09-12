package media

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestRenderSmokePreview(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real preview gate runs inside Docker")
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		t.Run(ratio, func(t *testing.T) {
			a, err := New(mediaConfig(t), nil)
			if err != nil {
				t.Fatal(err)
			}
			r, err := NewRenderer(a, renderConfig(t))
			if err != nil {
				t.Fatal(err)
			}
			plan := declaredPlan(t, `<clip version="1" styles="simple" pace="rapid"><text id="badge" kind="fixed" role="badge" basis="whole">직접 방문</text><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></clip>`, ratio)
			for i := range plan.Portable.Elements {
				if plan.Portable.Elements[i].Resolved.Element.Kind == "ai" {
					plan.Portable.Elements[i].Resolved.Text = "오늘은 철판 요리를 먹어요"
				}
			}
			refs := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
			cfg := clip.PreviewConfig{MaxAssets: 8, MaxAssetBytes: 512 << 10, MaxResponseBytes: 4 << 20, Timeout: 5 * time.Second}
			ctx, cancel := context.WithTimeout(t.Context(), cfg.Timeout)
			defer cancel()
			started := time.Now()
			prepared, err := r.PreparePreview(ctx, plan, refs, nil, 0, cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("source-free real-font preview: %d assets in %s", len(prepared.Assets), time.Since(started))
			if len(prepared.Assets) < 3 || prepared.NextOffset != -1 {
				t.Fatal("missing badge or rapid glyphs", prepared.NextOffset)
			}
			rapid := false
			for _, asset := range prepared.Assets {
				img, err := png.Decode(bytes.NewReader(asset.PNG))
				if err != nil {
					t.Fatal(err)
				}
				if img.Bounds().Dx() != asset.Width || img.Bounds().Dy() != asset.Height || asset.X < 0 || asset.Y < 0 || asset.X+asset.Width > prepared.Canvas.Width || asset.Y+asset.Height > prepared.Canvas.Height || len(asset.PNG) > cfg.MaxAssetBytes {
					t.Fatal("invalid cropped manifest", asset.Key)
				}
				if asset.EndMS-asset.StartMS == 300 {
					rapid = true
					if asset.InMS != 0 || asset.OutMS != 0 || asset.DY != 0 {
						t.Fatal("rapid motion added")
					}
				}
			}
			if !rapid {
				t.Fatal("300 ms cue lost")
			}
		})
	}
}
