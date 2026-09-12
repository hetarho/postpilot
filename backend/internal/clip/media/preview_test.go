package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func previewMeasured(t *testing.T, ratio string) (*Adapter, *Rendering, *fakeRunner) {
	a, r := measured(t)
	measure := a.runner
	canvas, _ := clip.ClipCanvas(ratio)
	runner := &fakeRunner{run: func(ctx context.Context, c Command) ([]byte, error) {
		last := c.Args[len(c.Args)-1]
		if !strings.HasSuffix(last, ".png") {
			return measure.Run(ctx, c)
		}
		if c.Binary != r.cfg.ResvgPath {
			t.Fatal("preview invoked a media decoder")
		}
		img := image.NewNRGBA(image.Rect(0, 0, canvas.Width, canvas.Height))
		for y := 23; y < 28; y++ {
			for x := 11; x < 18; x++ {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 200})
			}
		}
		f, err := os.Create(last)
		if err != nil {
			return nil, err
		}
		err = png.Encode(f, img)
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		return nil, closeErr
	}}
	a.runner = runner
	return a, r, runner
}
func TestPreviewCropsTransparentAssetsWithExportIntervalsOnAllRatios(t *testing.T) {
	cfg := clip.PreviewConfig{MaxAssets: 8, MaxAssetBytes: 512 << 10, MaxResponseBytes: 4 << 20, Timeout: 5 * time.Second}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		t.Run(ratio, func(t *testing.T) {
			a, r, runner := previewMeasured(t, ratio)
			plan := declaredPlan(t, `<clip version="1"><text id="fixed" kind="fixed" role="badge" basis="whole">정확한 문구</text></clip>`, ratio)
			sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
			result, err := r.PreparePreview(t.Context(), plan, sources, []string{"fixed"}, 0, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if result.NextOffset != -1 || len(result.Assets) != 1 || len(result.Parity) != 3 {
				t.Fatal(result)
			}
			asset := result.Assets[0]
			if asset.X != 11 || asset.Y != 23 || asset.Width != 7 || asset.Height != 5 || asset.StartMS != 0 || asset.EndMS != 15000 || len(asset.Key) != 64 {
				t.Fatalf("crop=%d,%d %dx%d interval=%d..%d key=%s", asset.X, asset.Y, asset.Width, asset.Height, asset.StartMS, asset.EndMS, asset.Key)
			}
			image, err := png.Decode(bytes.NewReader(asset.PNG))
			if err != nil || image.Bounds().Dx() != 7 {
				t.Fatal(err)
			}
			for _, call := range runner.calls {
				if call.Binary != r.cfg.ResvgPath {
					t.Fatal("source pixels requested")
				}
			}
			dirs, _ := filepath.Glob(filepath.Join(a.cfg.WorkRoot, "clip-preview*"))
			if len(dirs) != 0 {
				t.Fatal("workspace leaked", dirs)
			}
			cfgSmall := cfg
			cfgSmall.MaxAssetBytes = 1
			if _, err := r.PreparePreview(t.Context(), plan, sources, []string{"fixed"}, 0, cfgSmall); !errors.Is(err, clip.ErrPreviewTooLarge) {
				t.Fatal(err)
			}
			if _, err := r.PreparePreview(t.Context(), plan, sources, []string{"foreign"}, 0, cfg); !errors.Is(err, clip.ErrInvalid) {
				t.Fatal(err)
			}
		})
	}
}
func TestPreviewRapidPageUsesExactCuesAndCancellation(t *testing.T) {
	_, r, _ := previewMeasured(t, "vertical")
	plan := declaredPlan(t, `<clip version="1" pace="rapid" styles="simple"><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></clip>`, "vertical")
	plan.Portable.Elements[0].Resolved.Text = "오늘은 철판 요리를 먹어요"
	cfg := clip.PreviewConfig{MaxAssets: 1, MaxAssetBytes: 512 << 10, MaxResponseBytes: 4 << 20, Timeout: 5 * time.Second}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
	id := plan.Portable.Elements[0].Resolved.InstanceID
	first, err := r.PreparePreview(t.Context(), plan, sources, []string{id}, 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.NextOffset != 1 || first.Assets[0].EndMS-first.Assets[0].StartMS != 300 || first.Assets[0].InMS != 0 || first.Assets[0].OutMS != 0 || first.Assets[0].DY != 0 {
		t.Fatal(first)
	}
	second, err := r.PreparePreview(t.Context(), plan, sources, []string{id}, first.NextOffset, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.Assets[0].EndMS != second.Assets[0].StartMS {
		t.Fatal("rapid clock gap")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.PreparePreview(ctx, plan, sources, []string{id}, 0, cfg); err == nil {
		t.Fatal("cancelled preparation succeeded")
	}
}
