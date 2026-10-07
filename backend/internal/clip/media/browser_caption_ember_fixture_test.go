package media

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestExportBrowserEmberFixtures(t *testing.T) {
	root := os.Getenv("CLIP_BROWSER_EMBER_FIXTURES")
	if root == "" {
		t.Skip("explicit local transform fixture export")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	r := realMetricsRenderer(t)
	var cases []map[string]any
	export := func(id, ratio, style, pace, text, groundName string) {
		t.Helper()
		plan := declaredPlan(t, fmt.Sprintf(`<clip version="1" pace="%s" styles="%s"><text id="caption" kind="fixed" role="caption" style="%s" position="bottom" basis="output-start" start="1.05" end="5.55">%s</text></clip>`, pace, style, style, escaped(text)), ratio)
		plan.CaptionStyles = []string{style}
		plan.Cuts[0].Fingerprint = strings.Repeat("a", 64)
		// Cross a real300ms cut overlap without changing the output-frame clock.
		plan.Cuts[0].EndMS = 4000
		second := plan.Cuts[0]
		second.ID, second.StartMS, second.EndMS, second.TransitionMS = "cut2", 0, 11300, 300
		plan.Cuts = append(plan.Cuts, second)
		plan.Portable.Cuts[0].EndMS = 4000
		portableSecond := plan.Portable.Cuts[0]
		portableSecond.ID, portableSecond.StartMS, portableSecond.EndMS, portableSecond.TransitionMS = "cut2", 0, 11300, 300
		plan.Portable.Cuts = append(plan.Portable.Cuts, portableSecond)
		plan.Portable.Elements[0].Keyword = "AV"
		err := r.media.WithWorkspace(t.Context(), "browser-ember-fixture", func(ws clip.MediaWorkspace) error {
			layout, err := r.layoutComposition(t.Context(), ws, plan)
			if err != nil {
				return err
			}
			canvas, _ := clip.ClipCanvas(ratio)
			for visualIndex, selected := range layout.visuals {
				var ground any
				backdrop := ""
				if groundName != "" {
					rgb := [3]float64{0, 0, 0}
					if groundName == "bright" {
						rgb = [3]float64{253.0 / 255, 1, 1}
					}
					value := relativeLuminance(rgb[0], rgb[1], rgb[2])
					selected.ground = summarize([]float64{value, value, value}, [][3]float64{rgb, rgb, rgb})
					applyDeclaredGround(canvas, &selected)
					ground = map[string]any{"hex": selected.ground.Hex(), "rgb": rgb, "scrim": selected.ground.Scrim(), "accentWhite": selected.ground.AccentWhite(), "mean": selected.ground.Mean}
					backdrop = fmt.Sprintf(`<rect width="100%%" height="100%%" fill="%s"/>`, selected.ground.Hex())
				}
				start, end := selected.manifest.StartMS, selected.manifest.EndMS
				first, visible, last := int(math.Floor(float64(start)*30/1000)), (start*30+999)/1000, (end*30+999)/1000
				frames := []int{visible, visible + 1, visible + 4, last - 2, last - 1}
				if groundName != "" {
					frames = []int{visible, visible + 4, last - 2}
				}
				points := []float64{1.0 / 12, 19.0 / 97, 0.25, 0.5, 7.0 / 12, 0.75}
				if groundName != "" {
					points = []float64{1.0 / 12, 19.0 / 97, 0.5}
				}
				for _, p := range points {
					frames = append(frames, first+int(math.Round(p*float64(last-first-1))))
				}
				frames = append(frames, 111, 115, 119)
				seen := map[int]bool{}
				for _, frame := range frames {
					if frame < visible || frame >= last || seen[frame] {
						continue
					}
					seen[frame] = true
					progress := float64(frame-first) / float64(max(1, last-first-1))
					if pace == "rapid" {
						progress = representativeProgress
					}
					fr := captionFrame(canvas, selected.copy, selected.caption, end-start, progress)
					defs, body, ok := design.DrawCaptionFrame(fr)
					if !ok {
						return fmt.Errorf("not a sequence style: %s", style)
					}
					svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><defs>%s</defs>%s%s</svg>`, canvas.Width, canvas.Height, defs, backdrop, body)
					file := fmt.Sprintf("%s-%d.png", id, frame)
					if pace == "rapid" {
						file = fmt.Sprintf("%s-cue%d-%d.png", id, visualIndex, frame)
					}
					local := filepath.Join(ws.Path, file)
					if err := r.rasterizeTo(t.Context(), ws, clip.Region{Width: float64(canvas.Width), Height: float64(canvas.Height)}, svg, local); err != nil {
						return err
					}
					if err := copyIdentityFile(local, filepath.Join(root, file)); err != nil {
						return err
					}
					cases = append(cases, map[string]any{"id": strings.TrimSuffix(file, ".png"), "ratio": ratio, "frame": frame, "reference": file, "style": style, "pace": pace, "nativeProgress": progress, "ground": ground, "nativeEmber": nativeEmberFixtureGeometry(body), "nativeLayout": captionFrame(canvas, selected.copy, selected.caption, end-start, progress).Lines,
						"plan":   map[string]any{"nativeComposition": true, "durationMs": plan.DurationMS, "elements": browserInkFixtureElements(layout.plan), "cuts": []map[string]any{{"id": "cut", "sourceId": "source", "fingerprint": strings.Repeat("a", 64), "startMs": 0, "endMs": 4000, "transitionMs": 0, "playbackRatePermille": 1000, "volumePermille": 0, "copies": []any{}}, {"id": "cut2", "sourceId": "source", "fingerprint": strings.Repeat("a", 64), "startMs": 0, "endMs": 11300, "transitionMs": 300, "playbackRatePermille": 1000, "volumePermille": 0, "copies": []any{}}}},
						"design": map[string]any{"hideDisclosure": true, "introPreset": "a", "outroPreset": "b", "captionStyles": []string{style}, "captionPace": pace, "accent": plan.Accent}})
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(id, renderFailure(err))
		}
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, style := range []string{"ember"} {
			for _, pace := range []string{"steady", "rapid"} {
				if os.Getenv("CLIP_BROWSER_EMBER_GROUND_ONLY") != "1" {
					export(ratio+"-"+style+"-"+pace, ratio, style, pace, "갂 AV 12,500원", "")
				}
				if os.Getenv("CLIP_BROWSER_EMBER_GROUND_ONLY") != "1" {
					export(ratio+"-"+style+"-two-"+pace, ratio, style, pace, "갂 AV\n정확한 두 줄", "")
					export(ratio+"-"+style+"-long-"+pace, ratio, style, pace, "갂 AV 12,500원 정확한 불꽃과 글자", "")
				}
				for _, ground := range []string{"bright", "dark"} {
					export(ratio+"-"+style+"-two-"+pace+"-"+ground, ratio, style, pace, "갂 AV\n정확한 두 줄", ground)
				}
			}
		}
	}
	nativeProfile := "resvg0.48.1-host"
	if os.Getenv("CLIP_MEDIA_SMOKE") == "1" {
		nativeProfile = "resvg0.48.1-cpu-image"
	}
	data, err := json.MarshalIndent(map[string]any{"version": 1, "native": nativeProfile, "syntheticOnly": true, "rendererVersion": clip.MediaRendererVersion, "componentVersion": clip.BrowserComponentVersion, "cases": cases}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "manifest.json"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported %d native ember frames", len(cases))
}

// Independent reference values are parsed from the actual native painter's SVG.
func nativeEmberFixtureGeometry(body string) map[string]any {
	numbers := regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?`)
	parse := func(s string) []float64 {
		var out []float64
		for _, v := range numbers.FindAllString(s, -1) {
			n, _ := strconv.ParseFloat(v, 64)
			out = append(out, n)
		}
		return out
	}
	var paths, sparks [][]float64
	for _, match := range regexp.MustCompile(`<path d="([^"]+)"`).FindAllStringSubmatch(body, -1) {
		paths = append(paths, parse(match[1]))
	}
	for _, match := range regexp.MustCompile(`<circle cx="([^"]+)" cy="([^"]+)" r="([^"]+)" fill="[^"]+" opacity="([^"]+)"`).FindAllStringSubmatch(body, -1) {
		sparks = append(sparks, parse(strings.Join(match[1:], " ")))
	}
	return map[string]any{"paths": paths, "sparks": sparks}
}
