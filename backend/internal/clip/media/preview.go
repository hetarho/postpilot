package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"

	"github.com/postpilot/backend/internal/clip"
)

var (
	_ clip.PreviewPreparer         = (*Rendering)(nil)
	_ clip.GroundedPreviewPreparer = (*Rendering)(nil)
)

func (r *Rendering) PreparePreview(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource, ids []string, offset int, cfg clip.PreviewConfig) (clip.PreparedPreview, error) {
	return r.preparePreview(ctx, plan, sources, nil, ids, offset, cfg)
}

// PrepareGroundedPreview draws a browser render's assets with the grounds the
// server sampled for it: the scrims, accent colours and contrast notices a
// server render of the same plan draws (CLIP-192). The source contrast is then
// no longer final-only.
func (r *Rendering) PrepareGroundedPreview(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource, grounds []clip.SampledGround, ids []string, offset int, cfg clip.PreviewConfig) (clip.PreparedPreview, error) {
	if grounds == nil {
		grounds = []clip.SampledGround{}
	}
	return r.preparePreview(ctx, plan, sources, grounds, ids, offset, cfg)
}

// preparePreview draws on no ground when grounds is nil, which is the editing
// preview's, and on those grounds otherwise.
func (r *Rendering) preparePreview(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource, grounds []clip.SampledGround, ids []string, offset int, cfg clip.PreviewConfig) (clip.PreparedPreview, error) {
	out := clip.PreparedPreview{NextOffset: -1, Parity: []clip.PreviewParity{clip.PreviewSourceContrast, clip.PreviewAudioNormalization, clip.PreviewFrameTiming}}
	if grounds != nil {
		out.Parity = out.Parity[1:]
	}
	if cfg.MaxAssets <= 0 || cfg.MaxAssetBytes <= 0 || cfg.MaxResponseBytes <= 0 || offset < 0 || len(ids) > cfg.MaxAssets {
		return out, clip.ErrPreviewTooLarge
	}
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		return out, err
	}
	out.Canvas = canvas
	if err := clip.ValidateEditPlan(r.cfg, compositionGeometry(plan), sources); err != nil {
		return out, err
	}
	requested := map[string]bool{}
	for _, id := range ids {
		if id == "" || requested[id] {
			return out, clip.ErrInvalid
		}
		requested[id] = true
	}
	known := map[string]bool{}
	if plan.Portable == nil {
		return out, clip.ErrInvalid
	}
	for _, text := range plan.Portable.Elements {
		known[text.Resolved.InstanceID] = true
	}
	for id := range requested {
		if !known[id] {
			return out, clip.ErrInvalid
		}
	}
	err = r.media.WithWorkspace(ctx, "clip-preview", func(ws clip.MediaWorkspace) error {
		layout, err := r.previewLayout(ctx, ws, canvas, plan, grounds)
		if err != nil {
			return err
		}
		visuals := []declaredVisual{}
		for _, v := range layout.visuals {
			if len(requested) == 0 || requested[v.manifest.InstanceID] {
				visuals = append(visuals, v)
			}
		}
		if offset > len(visuals) {
			return clip.ErrInvalid
		}
		end := min(offset+cfg.MaxAssets, len(visuals))
		if end < len(visuals) {
			out.NextOffset = end
		}
		total := 0
		for i := offset; i < end; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			v := visuals[i]
			// No original is loaded or sampled here: an editing preview draws on no
			// ground, labelled final-only, and a browser render's assets on the
			// grounds its sampling job measured.
			var body string
			if v.manifest.Role == "caption" {
				body, err = r.captionDocument(canvas, v)
			} else {
				body, err = r.declaredSVG(canvas, v)
			}
			if err != nil {
				return err
			}
			path, err := r.rasterize(ctx, ws, canvas, body, fmt.Sprintf("preview-%04d", i))
			if err != nil {
				return err
			}
			data, rect, err := cropPreviewPNG(ctx, path, canvas, cfg.MaxAssetBytes)
			_ = os.Remove(path)
			if err != nil {
				return err
			}
			total += len(data)
			if total > cfg.MaxResponseBytes {
				return clip.ErrPreviewTooLarge
			}
			hash := sha256.Sum256(data)
			m := v.manifest
			out.Assets = append(out.Assets, clip.PreviewAsset{Key: hex.EncodeToString(hash[:]), InstanceID: m.InstanceID, PNG: data, X: rect.Min.X, Y: rect.Min.Y, Width: rect.Dx(), Height: rect.Dy(), StartMS: m.StartMS, EndMS: m.EndMS, InMS: m.InMS, OutMS: m.OutMS, DY: m.DY, Layer: m.Layer, RepresentativeFrame: sequenceDrawn(v), ContrastNotice: contrastNoticed(m)})
		}
		return ctx.Err()
	})
	if err != nil {
		return clip.PreparedPreview{}, err
	}
	return out, nil
}

// Alpha bounds preserve shadows/plates exactly; manifest text bounds alone
// would clip their decoration. The decoded raster cannot exceed one canvas.
func cropPreviewPNG(ctx context.Context, path string, canvas clip.Canvas, limit int) ([]byte, image.Rectangle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	defer file.Close()
	header, err := png.DecodeConfig(file)
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	if header.Width != canvas.Width || header.Height != canvas.Height {
		return nil, image.Rectangle{}, clip.ErrInvalidMedia
	}
	if _, err = file.Seek(0, 0); err != nil {
		return nil, image.Rectangle{}, err
	}
	img, err := png.Decode(file)
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	bounds := image.Rectangle{Min: image.Pt(canvas.Width, canvas.Height)}
	for y := 0; y < canvas.Height; y++ {
		if err := ctx.Err(); err != nil {
			return nil, image.Rectangle{}, err
		}
		for x := 0; x < canvas.Width; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0 {
				bounds.Min.X = min(bounds.Min.X, x)
				bounds.Min.Y = min(bounds.Min.Y, y)
				bounds.Max.X = max(bounds.Max.X, x+1)
				bounds.Max.Y = max(bounds.Max.Y, y+1)
			}
		}
	}
	if bounds.Empty() {
		bounds = image.Rect(0, 0, 1, 1)
	}
	cropped := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(cropped, cropped.Bounds(), img, bounds.Min, draw.Src)
	var b bytes.Buffer
	if err := png.Encode(&b, cropped); err != nil {
		return nil, image.Rectangle{}, err
	}
	if b.Len() > limit {
		return nil, image.Rectangle{}, clip.ErrPreviewTooLarge
	}
	return b.Bytes(), bounds, nil
}

// contrastNoticed reports whether any line of an element still reads under the
// contrast floor on its ground, after any scrim (CDS-44).
func contrastNoticed(m clip.CompositionElement) bool {
	for _, part := range m.Parts {
		if part.Kind == "copy" && part.ContrastNotice {
			return true
		}
	}
	return false
}
