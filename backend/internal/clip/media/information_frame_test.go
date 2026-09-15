package media

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func informationPlan(t *testing.T, ratio string) clip.EditPlan {
	t.Helper()
	return declaredPlan(t, `<clip version="1"><text id="information" kind="fixed" role="info" align="left" basis="whole"><row role="label">된장찌개</row><row role="caption">3,000원</row></text><text id="disclosure" kind="fixed" role="badge" basis="whole">광고</text></clip>`, ratio)
}

func TestInformationPairAllRatios(t *testing.T) {
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		t.Run(ratio, func(t *testing.T) {
			layout := measuredDeclared(t, informationPlan(t, ratio))
			v, badge := layout.visuals[0], layout.visuals[1]
			canvas, _ := clip.ClipCanvas(ratio)
			if len(v.info.Lines) != 2 || v.info.Plate != nil || !inside(v.manifest.Region, canvas.Safe) {
				t.Fatal(v)
			}
			for i, name := range []string{"label", "caption"} {
				line, part := v.info.Lines[i], v.manifest.Parts[i]
				role := design.Type[name]
				if line.Size != role.Size || line.Weight != role.Weight || line.Fill != "#FFFFFF" || line.StrokeWidth != design.Spacing.StrokeSmall || !line.Shadow || part.TypeRole != name {
					t.Fatal(name, line, part)
				}
				center := part.Region.X + part.Region.Width/2
				if math.Abs(center-(v.manifest.Region.X+v.manifest.Region.Width/2)) > .001 {
					t.Fatal("not centered", part)
				}
			}
			if v.info.Lines[0].Opacity != "0.72" || v.info.Lines[0].Tracking != 36*.08 || v.info.Lines[1].Opacity != "1" {
				t.Fatal(v.info.Lines)
			}
			if v.manifest.Region.Y != badge.manifest.Region.Y || v.manifest.Region.Height != badge.manifest.Region.Height || badge.furniture.Badge.Height != 68 {
				t.Fatal("header container or pill height", v.manifest, badge.manifest, badge.furniture)
			}
			if badge.furniture.Badge.Y+34 != v.manifest.Region.Y+v.manifest.Region.Height/2 {
				t.Fatal("badge optical center")
			}
			_, r := measured(t)
			for _, bright := range []bool{false, true} {
				view := v
				view.manifest.Parts = append(clip.Manifest{}, v.manifest.Parts...)
				view.ground = Luminance{Frames: []float64{0, 0, 0}}
				if bright {
					view.ground = Luminance{Mean: 1, R: 1, G: 1, B: 1, Frames: []float64{1, 1, 1}}
				}
				applyDeclaredGround(canvas, &view)
				if !design.Legible(view.manifest.Parts) {
					t.Fatal("default stroke must pass contrast")
				}
				svg, err := r.declaredSVG(canvas, view)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(svg, `id="scrim"`) != bright || strings.Contains(svg, `rx=`) || strings.Contains(svg, `data-frame`) || strings.Contains(svg, `clipPath`) {
					t.Fatal(svg)
				}
				if !bright {
					path := filepath.Join("testdata", "information-pair-"+ratio+".svg")
					if os.Getenv("UPDATE_GOLDEN") == "1" {
						if err := os.WriteFile(path, []byte(svg+"\n"), 0644); err != nil {
							t.Fatal(err)
						}
					}
					expected, err := os.ReadFile(path)
					if err != nil || strings.TrimSpace(string(expected)) != svg {
						t.Fatal("information pair changed", err)
					}
				}
			}
			svg, err := r.declaredSVG(canvas, badge)
			if err != nil || !strings.Contains(svg, `height="68.000"`) || !strings.Contains(svg, `rx="12.000"`) || !strings.Contains(svg, `font-weight="800"`) {
				t.Fatal(svg, err)
			}
		})
	}
}

func TestScrimmedInformationContrastShortfallIsOneNamedNotice(t *testing.T) {
	layout := measuredDeclared(t, informationPlan(t, "vertical"))
	canvas, _ := clip.ClipCanvas("vertical")
	v := &layout.visuals[0]
	v.ground = Luminance{Mean: 1, R: 1, G: 1, B: 1, Frames: []float64{1, 1, 1}}
	// Simulate a diagnosed pairing whose effective outline is too faint.
	original := design.Color["stroke_dark"]
	weak := original
	weak.Alpha = .01
	design.Color["stroke_dark"] = weak
	t.Cleanup(func() { design.Color["stroke_dark"] = original })
	applyDeclaredGround(canvas, v)
	if v.info.Scrim == nil || design.Legible(v.manifest.Parts) {
		t.Fatal("fixture did not exercise shortfall")
	}
	layout.recordContrastNotices()
	layout.recordContrastNotices()
	if len(layout.plan.Notices) != 1 || layout.plan.Notices[0].Reason != "composition_contrast" || layout.plan.Notices[0].ElementID != "information" {
		t.Fatal(layout.plan.Notices)
	}
	_, r := measured(t)
	if err := clip.VerifyCompositionManifest(layout.plan, layout.elements(), r.cfg.Composition); err != nil {
		t.Fatal("contrast notice refused delivery", err)
	}
	if _, err := r.declaredSVG(canvas, *v); err != nil {
		t.Fatal(err)
	}
}

func TestInformationPairRealFontSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real bundled-font renderer gate")
	}
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		if err := a.WithWorkspace(t.Context(), "real-info", func(ws clip.MediaWorkspace) error {
			layout, err := r.layoutComposition(t.Context(), ws, informationPlan(t, ratio))
			if err != nil {
				return err
			}
			canvas, _ := clip.ClipCanvas(ratio)
			for i, v := range layout.visuals {
				svg, err := r.declaredSVG(canvas, v)
				if err != nil {
					return err
				}
				if _, err = r.rasterize(t.Context(), ws, canvas, svg, fmt.Sprintf("info-%d", i)); err != nil {
					return err
				}
			}
			return clip.VerifyCompositionManifest(layout.plan, layout.elements(), r.cfg.Composition)
		}); err != nil {
			t.Fatal(ratio, err)
		}
	}
}
