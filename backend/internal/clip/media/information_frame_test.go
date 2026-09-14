package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestInformationFramesAllRatios(t *testing.T) {
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, variant := range []string{"emphasis", "compact"} {
			t.Run(ratio+"/"+variant, func(t *testing.T) {
				content := "해미연풍우가"
				if variant == "compact" {
					content = `<row role="label">된장찌개</row><row role="caption">3,000원</row>`
				}
				plan := declaredPlan(t, `<clip version="1"><text id="information" kind="fixed" role="info" basis="whole">`+content+`</text><text id="disclosure" kind="fixed" role="badge" basis="whole">광고</text></clip>`, ratio)
				layout := measuredDeclared(t, plan)
				v, badge := layout.visuals[0], layout.visuals[1]
				if v.infoVariant != variant || (v.info.Plate == nil) != (variant == "emphasis") {
					t.Fatal("wrong content variant", v.infoVariant)
				}
				if v.info.Frame.Height != v.manifest.Region.Height || badge.manifest.Region.Height != v.manifest.Region.Height {
					t.Fatal("shared header height lost")
				}
				canvas, _ := clip.ClipCanvas(ratio)
				if !inside(v.manifest.Region, canvas.Safe) || v.manifest.Region.Width > 600 {
					t.Fatal("frame outside safe geometry")
				}
				for _, line := range v.info.Lines {
					floor := design.Type["caption"].Min
					if variant == "emphasis" {
						floor = design.Type["title"].Min
					}
					if line.Size == design.Type["label"].Size {
						floor = design.Type["label"].Min
					}
					if line.Size < floor || !strings.Contains(line.Family, "Pretendard") {
						t.Fatal("wrong typography", line)
					}
				}
				_, renderer := measured(t)
				for _, bright := range []bool{false, true} {
					ground := Luminance{Frames: []float64{0, 0, 0}}
					if bright {
						ground = Luminance{Mean: 1, R: 1, G: 1, B: 1, Frames: []float64{1, 1, 1}}
					}
					view := v
					view.manifest.Parts = append(clip.Manifest{}, v.manifest.Parts...)
					view.ground = ground
					applyInfoGround(canvas, &view)
					if !design.Legible(view.manifest.Parts) {
						t.Fatal("V3 contrast floor")
					}
					svg, err := renderer.declaredSVG(canvas, view)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(svg, `id="scrim"`) != (bright && variant == "emphasis") {
						t.Fatal("wrong sampling treatment")
					}
					if !strings.Contains(svg, "info-"+variant+"-v1") {
						t.Fatal("missing versioned frame")
					}
					if !bright {
						path := filepath.Join("testdata", "information-"+variant+"-"+ratio+".svg")
						if os.Getenv("UPDATE_GOLDEN") == "1" {
							if err := os.WriteFile(path, []byte(svg+"\n"), 0644); err != nil {
								t.Fatal(err)
							}
						}
						expected, err := os.ReadFile(path)
						if err != nil || strings.TrimSpace(string(expected)) != svg {
							t.Fatal("information frame changed; review the SVG plate", err)
						}
					}
				}
				svg, err := renderer.declaredSVG(canvas, badge)
				if err != nil || strings.Contains(svg, "data-frame") || badge.furniture.BadgeText != "광고" || badge.manifest.Parts[0].FontSize != 40 {
					t.Fatal("disclosure adopted information styling", err)
				}
			})
		}
	}
}
