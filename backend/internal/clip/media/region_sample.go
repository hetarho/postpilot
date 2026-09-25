package media

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

var _ clip.RegionPresetSampler = (*Rendering)(nil)

// RegionPresetSamples draws every intro and outro preset on a ratio the way the
// render draws a region, each slot filled with the label numbered in outline
// order (CLIP-165): the same layout, the same overlay and the same verifier.
// No footage is read, so no block carries a scrim. A preset whose face cannot
// draw the label is left out rather than drawn in another face (CDS-84).
func (r *Rendering) RegionPresetSamples(ctx context.Context, ratio, label string) ([]clip.RegionPresetSample, []clip.RegionPresetSample, error) {
	if !clip.ValidSlotLabel(label) {
		return nil, nil, clip.ErrInvalid
	}
	canvas, err := clip.ClipCanvas(ratio)
	if err != nil {
		return nil, nil, err
	}
	samples := map[string][]clip.RegionPresetSample{}
	err = r.media.WithWorkspace(ctx, "clip-region-sample", func(ws clip.MediaWorkspace) error {
		for _, kind := range []string{"intro", "outro"} {
			for _, id := range design.RegionIDs(kind) {
				sample, err := r.regionPresetSample(ctx, ws, canvas, ratio, kind, id, label)
				var problem *composition.Problem
				if errors.As(err, &problem) {
					continue
				}
				if err != nil {
					return err
				}
				samples[kind] = append(samples[kind], sample)
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return samples["intro"], samples["outro"], nil
}

func (r *Rendering) regionPresetSample(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio, kind, id, label string) (clip.RegionPresetSample, error) {
	preset, _ := design.Region(kind, id)
	count := len(preset.Slots())
	role := "hook"
	if kind == "outro" {
		role = "ending"
	}
	text := clip.PortableText{Resolved: composition.ResolvedElement{InstanceID: kind, Element: composition.Element{ID: kind, Role: role, Kind: "fixed", Align: "center", Position: "auto", Style: "auto"}}}
	rows := make([]string, count)
	for i := range rows {
		rows[i] = strings.ReplaceAll(label, "{n}", strconv.Itoa(i+1))
		text.Resolved.Rows = append(text.Resolved.Rows, composition.ResolvedRow{Text: rows[i]})
	}
	visual := declaredVisual{text: text, manifest: declaredManifest(text)}
	visual, err := r.layoutDeclaredRegion(ctx, ws, canvas, ratio, visual, kind, id, clip.RegionPlacement{Drawn: count, Rules: true}, rows)
	if err != nil {
		return clip.RegionPresetSample{}, err
	}
	document, err := r.declaredSVG(canvas, visual)
	if err != nil {
		return clip.RegionPresetSample{}, err
	}
	// This document reaches a browser, which the render's own never does.
	if err := wellFormed(document); err != nil {
		return clip.RegionPresetSample{}, err
	}
	box := visual.manifest.Region
	return clip.RegionPresetSample{Preset: id, Box: clip.Region{X: rounded(box.X), Y: rounded(box.Y), Width: rounded(box.Width), Height: rounded(box.Height)},
		SVG: fmt.Sprintf(`<g>%s</g>`, prefixIDs(kind+"-"+id, innerSVG(document)))}, nil
}
