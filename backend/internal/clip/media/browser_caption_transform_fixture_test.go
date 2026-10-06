package media

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestExportBrowserCaptionTransformFixtures(t *testing.T) {
	root := os.Getenv("CLIP_BROWSER_TRANSFORM_FIXTURES")
	if root == "" {
		t.Skip("explicit local transform fixture export")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	r := realMetricsRenderer(t)
	var cases []map[string]any
	export := func(id, ratio, style, pace, text string) {
		t.Helper()
		plan := declaredPlan(t, fmt.Sprintf(`<clip version="1" pace="%s" styles="%s"><text id="caption" kind="fixed" role="caption" style="%s" position="bottom" basis="output-start" start="1.05" end="5.55">%s</text></clip>`, pace, style, style, escaped(text)), ratio)
		plan.CaptionStyles = []string{style}
		plan.Cuts[0].Fingerprint = strings.Repeat("a", 64)
		plan.Portable.Elements[0].Keyword = "AV"
		if strings.Contains(id, "pop-long-rapid") || (strings.Contains(id, "pop-short") && pace == "rapid") {
			plan.Portable.Elements[0].Phrases = []clip.EditablePhrase{{Text: text, StartMS: 1050, EndMS: 1850}}
		}
		err := r.media.WithWorkspace(t.Context(), "browser-transform-fixture", func(ws clip.MediaWorkspace) error {
			layout, err := r.layoutComposition(t.Context(), ws, plan)
			if err != nil {
				return err
			}
			canvas, _ := clip.ClipCanvas(ratio)
			for visualIndex, selected := range layout.visuals {
				start, end := selected.manifest.StartMS, selected.manifest.EndMS
				first, visible, last := int(math.Floor(float64(start)*30/1000)), (start*30+999)/1000, (end*30+999)/1000
				frames := []int{visible, last - 2, last - 1}
				for _, p := range []float64{0.1, 0.3, 0.5, 0.74} {
					frames = append(frames, first+int(math.Round(p*float64(last-first-1))))
				}
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
					defs, body, ok := design.DrawCaptionFrame(captionFrame(canvas, selected.copy, selected.caption, end-start, progress))
					if !ok {
						return fmt.Errorf("not a sequence style: %s", style)
					}
					svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><defs>%s</defs>%s</svg>`, canvas.Width, canvas.Height, defs, body)
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
					cases = append(cases, map[string]any{"id": strings.TrimSuffix(file, ".png"), "ratio": ratio, "frame": frame, "reference": file, "style": style, "pace": pace, "nativeProgress": progress, "nativeLayout": captionFrame(canvas, selected.copy, selected.caption, end-start, progress).Lines,
						"plan":   map[string]any{"nativeComposition": true, "durationMs": plan.DurationMS, "elements": browserInkFixtureElements(layout.plan), "cuts": []map[string]any{{"id": "cut", "sourceId": "source", "fingerprint": strings.Repeat("a", 64), "startMs": 0, "endMs": 15000, "transitionMs": 0, "playbackRatePermille": 1000, "volumePermille": 0, "copies": []any{}}}},
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
		for _, style := range []string{"word-pop", "pop", "stack", "sticker", "bubble", "serif", "outline"} {
			for _, pace := range []string{"steady", "rapid"} {
				export(ratio+"-"+style+"-"+pace, ratio, style, pace, "갂 AV 12,500원")
			}
		}
		export(ratio+"-pop-long-steady", ratio, "pop", "steady", "가 나 다 라 마 바 사 아 자 차")
		export(ratio+"-pop-long-rapid", ratio, "pop", "rapid", "가 나 다 라 마 바 사 아 자")
		for count := 1; count <= 4; count++ {
			for _, pace := range []string{"steady", "rapid"} {
				export(fmt.Sprintf("%s-pop-short-%d-%s", ratio, count, pace), ratio, "pop", pace, strings.Join([]string{"가", "나", "다", "라"}[:count], " "))
			}
		}
	}
	data, err := json.MarshalIndent(map[string]any{"version": 2, "native": "resvg0.48.1-host", "syntheticOnly": true, "rendererVersion": clip.MediaRendererVersion, "componentVersion": clip.BrowserComponentVersion, "cases": cases}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "manifest.json"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported %d native transform frames", len(cases))
}
