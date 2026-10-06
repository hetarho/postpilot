package media

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestExportBrowserGroundedSceneFixtures(t *testing.T) {
	root := os.Getenv("CLIP_BROWSER_GROUND_SCENE_FIXTURES")
	if root == "" || os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("explicit matching CPU image fixture export")
	}
	if e := os.MkdirAll(root, 0755); e != nil {
		t.Fatal(e)
	}
	r := realMetricsRenderer(t)
	cases := []map[string]any{}
	for _, style := range []string{"word-pop", "outline"} {
		for _, bright := range []bool{false, true} {
			id := "dark"
			rgb := [3]float64{0, 0, 0}
			if bright {
				id = "bright"
				rgb = [3]float64{253. / 255, 1, 1}
			}
			plan := declaredPlan(t, fmt.Sprintf(`<clip version="1" styles="%s"><text id="caption" kind="fixed" role="caption" style="%s" position="bottom" basis="output-start" start="0" end="2">지금 보는 화면 기록</text></clip>`, style, style), "vertical")
			plan.CaptionStyles = []string{style}
			plan.Accent = "blue"
			plan.Portable.Elements[0].Keyword = "화면"
			e := r.media.WithWorkspace(t.Context(), "grounded-scene-fixture", func(ws clip.MediaWorkspace) error {
				layout, e := r.layoutComposition(t.Context(), ws, plan)
				if e != nil {
					return e
				}
				visual := layout.visuals[0]
				canvas, _ := clip.ClipCanvas("vertical")
				value := relativeLuminance(rgb[0], rgb[1], rgb[2])
				visual.ground = summarize([]float64{value, value, value}, [][3]float64{rgb, rgb, rgb})
				applyDeclaredGround(canvas, &visual)
				for _, frame := range []int{1, 30, 58} {
					caption := captionFrame(canvas, visual.copy, visual.caption, 2000, float64(frame)/59)
					defs, body, ok := design.DrawCaptionFrame(caption)
					if !ok {
						return clip.ErrInvalid
					}
					svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><defs>%s</defs><rect width="100%%" height="100%%" fill="%s"/>%s</svg>`, canvas.Width, canvas.Height, defs, visual.ground.Hex(), body)
					file := fmt.Sprintf("%s-%s-%d.png", style, id, frame)
					local := filepath.Join(ws.Path, file)
					if e := r.rasterizeTo(t.Context(), ws, clip.Region{Width: float64(canvas.Width), Height: float64(canvas.Height)}, svg, local); e != nil {
						return e
					}
					if e := copyIdentityFile(local, filepath.Join(root, file)); e != nil {
						return e
					}
					cases = append(cases, map[string]any{"id": file, "style": style, "ground": id, "frame": frame, "reference": file, "sampledMean": visual.ground.Mean, "scrim": visual.ground.Scrim(), "accentWhite": visual.ground.AccentWhite(), "nativeAccent": caption.Accent, "parts": visual.manifest.Parts})
				}
				return nil
			})
			if e != nil {
				t.Fatal(style, id, e)
			}
		}
	}
	raw, e := json.MarshalIndent(map[string]any{"rendererVersion": clip.MediaRendererVersion, "components": clip.BrowserComponentVersion, "qualification": false, "cases": cases}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "manifest.json"), append(raw, '\n'), 0644); e != nil {
		t.Fatal(e)
	}
}
