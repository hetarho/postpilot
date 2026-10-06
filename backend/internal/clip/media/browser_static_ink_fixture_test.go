package media

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// Opt-in synthetic component qualification. No database, provider, footage,
// storage credential or account is loaded; native resvg is the reference.
func TestExportBrowserStaticInkFixtures(t *testing.T) {
	root := os.Getenv("CLIP_BROWSER_INK_FIXTURES")
	if root == "" {
		t.Skip("explicit local static component fixture export")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	r := realMetricsRenderer(t)
	var cases []map[string]any
	export := func(id, ratio, body string, intro, outro string, styles ...string) {
		t.Helper()
		plan := declaredPlan(t, body, ratio)
		plan.IntroPreset, plan.OutroPreset = intro, outro
		plan.CaptionStyles = styles
		plan.Cuts[0].Fingerprint = strings.Repeat("a", 64)
		err := r.media.WithWorkspace(t.Context(), "browser-ink-fixture", func(ws clip.MediaWorkspace) error {
			layout, err := r.layoutComposition(t.Context(), ws, plan)
			if err != nil {
				return err
			}
			canvas, _ := clip.ClipCanvas(ratio)
			frame := 45
			if len(layout.visuals) > 0 {
				frame = (layout.visuals[0].manifest.StartMS*30+999)/1000 + 15
			}
			var content strings.Builder
			for _, visual := range layout.visuals {
				if frame*1000 < visual.manifest.StartMS*30 || frame*1000 >= visual.manifest.EndMS*30 {
					continue
				}
				svg, err := r.declaredSVG(canvas, visual)
				if err != nil {
					return err
				}
				content.WriteString(prefixIDs(visual.manifest.InstanceID, innerSVG(svg)))
			}
			svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">%s</svg>`, canvas.Width, canvas.Height, content.String())
			file := id + ".png"
			local := filepath.Join(ws.Path, file)
			if err := r.rasterizeTo(t.Context(), ws, clip.Region{Width: float64(canvas.Width), Height: float64(canvas.Height)}, svg, local); err != nil {
				return err
			}
			if err := copyIdentityFile(local, filepath.Join(root, file)); err != nil {
				return err
			}
			cases = append(cases, map[string]any{"id": id, "ratio": ratio, "frame": frame, "reference": file,
				"plan": map[string]any{"nativeComposition": true, "durationMs": plan.DurationMS, "elements": browserInkFixtureElements(layout.plan),
					"cuts": []map[string]any{{"id": "cut", "sourceId": "source", "fingerprint": strings.Repeat("a", 64), "startMs": 0, "endMs": 15000, "transitionMs": 0, "playbackRatePermille": 1000, "volumePermille": 0, "copies": []any{}}}},
				"design": map[string]any{"hideDisclosure": true, "introPreset": intro, "outroPreset": outro, "captionStyles": styles, "captionPace": plan.CaptionPaceOrDefault(), "accent": plan.Accent}})
			return nil
		})
		if err != nil {
			t.Fatal(id, renderFailure(err))
		}
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, style := range []string{"bold", "keynote", "film"} {
			for _, pace := range []string{"steady", "rapid"} {
				copy := "정확한 자막 갂 AV 12,500원"
				if pace == "rapid" {
					copy = "여기 진짜 좋아요"
				}
				export(ratio+"-"+style+"-"+pace, ratio, fmt.Sprintf(`<clip version="1" pace="%s" styles="%s"><text id="caption" kind="fixed" role="caption" style="%s" position="bottom" basis="output-start" start="1" end="8">%s</text></clip>`, pace, style, style, copy), "a", "b", style)
			}
		}
		for _, sample := range regionSamples {
			role, basis, start, end := "hook", "output-start", "0", "2.5"
			if sample.kind == "outro" {
				role, basis, start, end = "ending", "output-end", "-3", "0"
			}
			var rows strings.Builder
			for _, row := range sample.rows {
				rows.WriteString("<row>" + escaped(row) + "</row>")
			}
			intro, outro := "a", "b"
			if sample.kind == "intro" {
				intro = sample.id
			} else {
				outro = sample.id
			}
			export(ratio+"-"+sample.kind+"-"+sample.id, ratio, fmt.Sprintf(`<clip version="1"><text id="region" kind="fixed" role="%s" basis="%s" start="%s" end="%s">%s</text></clip>`, role, basis, start, end, rows.String()), intro, outro)
		}
		export(ratio+"-badge", ratio, `<clip version="1"><text id="badge" kind="fixed" role="badge" position="header" basis="whole">직접 방문</text></clip>`, "a", "b")
		export(ratio+"-header-info", ratio, `<clip version="1"><text id="badge" kind="fixed" role="badge" position="header" basis="whole">직접 방문</text><text id="info" kind="fixed" role="info" position="header" basis="whole"><row role="label">오늘의 기록</row><row role="caption">12,500원 · 서울</row></text></clip>`, "a", "b")
		export(ratio+"-blank-slots", ratio, `<clip version="1"><text id="region" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>점수</row><row></row><row>오늘의 기록</row></text></clip>`, "a", "e")
	}
	data, err := json.MarshalIndent(map[string]any{"version": 1, "native": "resvg0.48.1-host", "syntheticOnly": true, "cases": cases}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "manifest.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported %d static native component fixtures", len(cases))
}
