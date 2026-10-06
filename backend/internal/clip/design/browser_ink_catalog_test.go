package design

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// The browser consumes the native registry's actual effective role/paint/motion,
// never a second hand-kept family or an estimate from editor CSS.
func TestBrowserInkCatalogMatchesNative(t *testing.T) {
	styles := map[string]any{}
	for _, style := range CaptionStyles() {
		s := style.Paint.Shadow
		role := style.Role()
		styles[style.ID] = map[string]any{
			"id": style.ID, "face": style.Face, "weight": style.Weight, "rendering": style.Rendering,
			"role": map[string]any{"size": role.Size, "min": role.Min, "tracking": role.Tracking, "lineHeight": role.LineHeight},
			"rule": style.Rule(), "bleed": style.Bleed(), "perWord": style.PerWord(),
			"motion": map[string]any{"inMs": style.Motion.InMS, "outMs": style.Motion.OutMS, "dy": style.Motion.InDY},
			"paint": map[string]any{"fill": style.Paint.Fill, "stroke": style.Paint.Stroke, "plate": style.Paint.Plate,
				"accent": style.Paint.Accent, "scrim": style.Paint.Scrim,
				"shadow": map[string]any{"hex": s.Hex, "alpha": s.Alpha, "blur": s.Blur, "dx": s.DX, "dy": s.DY}},
		}
	}
	implementation := map[string]string{}
	for _, file := range []string{"caption_draw.go", "caption_frame.go", "caption_transform.go", "caption_effect.go"} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		implementation[file] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	effectSources := map[string]string{}
	for _, file := range []string{"frontend/src/entities/clip-preview/model/ink-caption-effects.ts", "frontend/src/entities/clip-preview/model/ink-caption-filter-pixi.ts", "frontend/src/entities/clip-preview/model/ink-caption-pixi.ts", "frontend/src/entities/clip-preview/model/ink-caption-draw.ts", "frontend/src/entities/clip-preview/model/ink-caption-scene.ts", "frontend/src/entities/clip-preview/model/local-components.ts", "frontend/src/entities/clip-preview/model/composition-placement.ts", "frontend/src/entities/clip-preview/config/composition-placement.ts", "frontend/src/entities/clip-preview/model/ink-caption-ember.ts", "frontend/src/entities/clip-preview/model/ink-caption-ember-pixi.ts", "pnpm-lock.yaml"} {
		body, err := os.ReadFile("../../../../" + file)
		if err != nil {
			t.Fatal(err)
		}
		effectSources[file] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	effectImplementation := map[string]any{"version": "caption-filters-v1-ember-v2-retained", "pixi": "8.22.0", "colorSpace": "sRGB", "alpha": "premultiplied-alpha", "kernel": 15, "quality": 4, "sources": effectSources}
	expected, err := json.MarshalIndent(map[string]any{"version": 3, "styles": styles, "transformPaint": CaptionTransformPaint(), "effectPaint": CaptionEffectPaint(), "effectImplementation": effectImplementation, "nativePainterSources": implementation}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	expected = append(expected, '\n')
	path := "../../../../frontend/src/entities/clip-design/config/clip-caption-ink.json"
	if os.Getenv("CLIP_BROWSER_INK_UPDATE") == "1" {
		if err := os.WriteFile(path, expected, 0644); err != nil {
			t.Fatal(err)
		}
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if json.Unmarshal(actual, &got) != nil || json.Unmarshal(expected, &want) != nil {
		t.Fatal("invalid browser ink catalog")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("browser ink catalog differs from the native caption registry")
	}
}
