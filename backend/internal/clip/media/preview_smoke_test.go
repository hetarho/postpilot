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
	t.Parallel()
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
			plan := declaredPlan(t, `<clip version="1" intro="b" caption="bold" outro="e" pace="rapid"><text id="badge" kind="fixed" role="badge" basis="whole">직접 방문</text><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`, ratio)
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

func TestRenderSmokePreviewSequenceRepresentative(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real preview gate runs inside Docker")
	}
	t.Parallel()
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	plan := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: "neon"})
	plan.CaptionStyles = []string{"bold", "neon"}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 60000, Width: 1920, Height: 1080}}}
	out, err := r.PreparePreview(t.Context(), plan, sources, nil, 0, clip.PreviewConfig{MaxAssets: 8, MaxAssetBytes: 512 << 10, MaxResponseBytes: 4 << 20})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, asset := range out.Assets {
		if !asset.RepresentativeFrame {
			continue
		}
		found = true
		img, err := png.Decode(bytes.NewReader(asset.PNG))
		if err != nil || img.Bounds().Dx() != asset.Width || img.Bounds().Dy() != asset.Height {
			t.Fatal("invalid representative raster", err)
		}
		visible := false
		for y := 0; y < asset.Height && !visible; y++ {
			for x := 0; x < asset.Width; x++ {
				_, _, _, alpha := img.At(x, y).RGBA()
				if alpha > 0 {
					visible = true
					break
				}
			}
		}
		if !visible {
			t.Fatal("empty representative caption")
		}
	}
	if !found {
		t.Fatal("sequence caption missing")
	}
}

// CLIP-159: the sheet a browser render draws a sequence caption from is a real
// PNG, rasterised by the same resvg the render uses, with one visible cell per
// output frame of the run.
func TestRenderSmokeCaptionFrames(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real caption-frame gate runs inside Docker")
	}
	t.Parallel()
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	plan := declaredPlan(t, `<clip version="1" styles="word-pop"><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></clip>`, "vertical")
	for i := range plan.Portable.Elements {
		if plan.Portable.Elements[i].Resolved.Element.Kind == "ai" {
			plan.Portable.Elements[i].Resolved.Text = "오늘은 철판 요리를 먹어요"
		}
	}
	plan.CaptionStyles = []string{"word-pop"}
	refs := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
	cfg := clip.DefaultGenerationConfig(clip.Environment{}).Preview
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	started := time.Now()
	frames, err := r.PrepareCaptionFrames(ctx, plan, refs, "copy/cut", 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("real-font caption sheet: %d cells in %s, %d bytes", frames.Cells, time.Since(started), len(frames.Sheet))
	img, err := png.Decode(bytes.NewReader(frames.Sheet))
	if err != nil {
		t.Fatal(err)
	}
	rows := (frames.Cells + frames.Columns - 1) / frames.Columns
	if img.Bounds().Dx() != frames.Columns*frames.CellWidth || img.Bounds().Dy() != rows*frames.CellHeight {
		t.Fatalf("the sheet is %v for %d cells of %dx%d", img.Bounds(), frames.Cells, frames.CellWidth, frames.CellHeight)
	}
	// The cells are the caption's own motion, so they DIFFER: a style that fades
	// in draws its first frame transparent — the render's own first frame is the
	// same — and the frames after it carry the drawing.
	ink := make([]uint64, frames.Cells)
	for cell := range frames.Cells {
		col, row := cell%frames.Columns, cell/frames.Columns
		for y := row * frames.CellHeight; y < (row+1)*frames.CellHeight; y += 4 {
			for x := col * frames.CellWidth; x < (col+1)*frames.CellWidth; x += 4 {
				r, g, b, alpha := img.At(x, y).RGBA()
				ink[cell] += uint64(alpha>>8)*1 + uint64(r>>8)<<8 + uint64(g>>8)<<16 + uint64(b>>8)<<24
			}
		}
	}
	if ink[frames.Cells-1] == 0 {
		t.Fatal("the settled cell of the sheet is empty")
	}
	distinct := map[uint64]bool{}
	for _, value := range ink {
		distinct[value] = true
	}
	if len(distinct) < 2 {
		t.Fatalf("every cell drew the same picture: %v", ink)
	}
}
