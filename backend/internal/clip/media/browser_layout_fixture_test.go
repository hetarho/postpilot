package media

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/llm"
)

// Independent native output: each input is exported BEFORE native layout adds Placement.
// JSON-only fixtures avoid large media allocations and load no account/provider/storage.
func TestExportBrowserAutomaticLayoutFixtures(t *testing.T) {
	root := os.Getenv("CLIP_BROWSER_LAYOUT_FIXTURES")
	if root == "" {
		t.Skip("explicit independent native automatic-layout fixture export")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	adapter, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := renderConfig(t)
	if selected := os.Getenv("CLIP_RESVG_PATH"); selected != "" {
		cfg.ResvgPath = selected
	}
	if _, err := os.Stat(cfg.FontPaths["wantedsans"]); err != nil {
		cfg.FontPaths = bundledFontPaths(t)
	}
	renderer, err := NewRenderer(adapter, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cases := []map[string]any{}
	export := func(id, ratio, style, pace, mode string) {
		t.Helper()
		end := "8"
		if pace == "rapid" {
			end = "2.4"
		}
		body := fmt.Sprintf(`<clip version="1" pace="%s" styles="%s"><text id="caption" kind="fixed" role="caption" style="%s" position="auto" basis="output-start" start="1" end="%s">현재 장면</text></clip>`, pace, style, style, end)
		if mode == "header" {
			var header strings.Builder
			header.WriteString(`<text id="badge" kind="fixed" role="badge" position="header" basis="whole">직접 방문</text>`)
			for i := 0; i < 10; i++ {
				fmt.Fprintf(&header, `<text id="info%d" kind="fixed" role="info" position="header" basis="whole"><row role="label">오늘의 기록</row><row role="caption">요약 %d</row></text>`, i, i)
			}
			body = strings.Replace(body, "<text id=\"caption\"", header.String()+"<text id=\"caption\"", 1)
		}
		if mode == "previous" {
			body = strings.Replace(body, `<text id="caption"`, `<text id="prior" kind="fixed" role="caption" style="bold" position="bottom" basis="output-start" start="1" end="3">이전 장면</text><text id="caption"`, 1)
			body = strings.Replace(body, `start="1" end="8">현재`, `start="4" end="8">현재`, 1)
		}
		plan := declaredPlan(t, body, ratio)
		plan.CaptionStyles = []string{style}
		plan.Cuts[0].Fingerprint = strings.Repeat("a", 64)
		observation := clip.SourceAnalysis{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: strings.Repeat("a", 64), Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}, Segments: []clip.Segment{{EndMS: 15000, Scene: "food"}}}
		if mode == "safe" {
			observation.Segments[0].CaptionSafe = []clip.Region{{X: 0, Y: .48, Width: 1, Height: .25}}
		}
		if mode == "subject" {
			observation.Segments[0].Subject = clip.Region{X: .2, Y: .2, Width: .6, Height: .3}
		}
		if mode == "readable" {
			observation.Segments[0].ReadableText = true
		}
		if mode == "crop" {
			plan.Cuts[0].Focal = clip.Point{X: .8, Y: .3}
			observation.Segments[0].Subject = clip.Region{X: .55, Y: .1, Width: .3, Height: .4}
			observation.Segments[0].CaptionSafe = []clip.Region{{X: .5, Y: .5, Width: .45, Height: .45}}
		}
		if mode == "owner" {
			for i := range plan.Portable.Elements {
				if plan.Portable.Elements[i].Resolved.Element.Role == "caption" {
					plan.Portable.Elements[i].Owner = clip.OwnerCaption{Position: &clip.CaptionPlacement{X: 980, Y: 1400}, Size: 64}
				}
			}
		}
		if mode == "crosscut" || mode == "rapid-first" || mode == "zero-subject" {
			plan.Cuts = []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: strings.Repeat("a", 64), StartMS: 0, EndMS: 1234, PlaybackRatePermille: 750, Focal: clip.Point{X: .5, Y: .5}}, {ID: "second", SourceID: "source", Fingerprint: strings.Repeat("a", 64), StartMS: 5000, EndMS: 15351, PlaybackRatePermille: 1250, TransitionMS: 200, Focal: clip.Point{X: .7, Y: .5}}}
			plan.Portable.Cuts = []composition.Cut{{ID: "cut", SourceID: "source", StartMS: 0, EndMS: 1234, PlaybackRatePermille: 750}, {ID: "second", SourceID: "source", StartMS: 5000, EndMS: 15351, PlaybackRatePermille: 1250, TransitionMS: 200}}
			plan.DurationMS = plan.Cuts[0].OutputDurationMS() + plan.Cuts[1].OutputDurationMS() - 200
			observation.Source.Info.DurationMS = 30000
			observation.Segments = []clip.Segment{{EndMS: 5000, Scene: "food", Subject: clip.Region{X: .2, Y: .2, Width: .6, Height: .25}, CaptionSafe: []clip.Region{{X: 0, Y: .5, Width: 1, Height: .2}}}, {StartMS: 5000, EndMS: 30000, Scene: "person", Subject: clip.Region{X: .2, Y: .5, Width: .6, Height: .25}, CaptionSafe: []clip.Region{{X: 0, Y: .15, Width: 1, Height: .2}}}}
			if mode == "zero-subject" {
				observation.Segments[0].Subject = clip.Region{}
			}
		}
		plan.Portable.Observations = []clip.SourceAnalysis{observation}
		rawCuts := []map[string]any{}
		for _, cut := range plan.Cuts {
			rawCuts = append(rawCuts, map[string]any{"id": cut.ID, "sourceId": cut.SourceID, "fingerprint": cut.Fingerprint, "startMs": cut.StartMS, "endMs": cut.EndMS, "transitionMs": cut.TransitionMS, "playbackRatePermille": cut.Rate(), "volumePermille": 0, "copies": []any{}, "focal": map[string]any{"x": cut.Focal.X, "y": cut.Focal.Y}})
		}
		rawPlan := map[string]any{"nativeComposition": true, "durationMs": plan.DurationMS, "elements": browserInkFixtureElements(plan), "cuts": rawCuts}
		segments := []map[string]any{}
		for _, segment := range observation.Segments {
			box := func(value clip.Region) map[string]any {
				return map[string]any{"x": value.X, "y": value.Y, "width": value.Width, "height": value.Height}
			}
			safe := []map[string]any{}
			for _, value := range segment.CaptionSafe {
				safe = append(safe, box(value))
			}
			segments = append(segments, map[string]any{"startMs": segment.StartMS, "endMs": segment.EndMS, "scene": segment.Scene, "readableText": segment.ReadableText, "subject": box(segment.Subject), "captionSafe": safe})
		}
		err := renderer.media.WithWorkspace(t.Context(), "browser-native-layout", func(ws clip.MediaWorkspace) error {
			layout, err := renderer.layoutComposition(t.Context(), ws, plan)
			if err != nil {
				return err
			}
			expected := []map[string]any{}
			for _, visual := range layout.visuals {
				if visual.manifest.Role != "caption" {
					continue
				}
				expected = append(expected, map[string]any{"instanceId": visual.manifest.InstanceID, "text": visual.copy.Text, "manifestText": visual.manifest.Text, "position": visual.manifest.Position, "style": visual.manifest.Style, "startMs": visual.manifest.StartMS, "endMs": visual.manifest.EndMS, "box": map[string]any{"x": visual.caption.Region.X, "y": visual.caption.Region.Y, "width": visual.caption.Region.Width, "height": visual.caption.Region.Height}, "fontSize": visual.caption.FontSize})
			}
			cases = append(cases, map[string]any{"id": id, "ratio": ratio, "plan": rawPlan, "design": map[string]any{"hideDisclosure": true, "captionStyles": []string{style}, "captionPace": pace, "introPreset": "a", "outroPreset": "b", "accent": plan.Accent}, "sources": []map[string]any{{"sourceId": "source", "fingerprint": strings.Repeat("a", 64), "durationMs": observation.Source.Info.DurationMS, "width": 1920, "height": 1080, "hasAudio": false, "originalMeasurementProvenance": "", "allowedRatePermille": []int{500, 750, 1000, 1250, 1500, 2000}}}, "layoutObservations": []map[string]any{{"sourceId": "source", "fingerprint": strings.Repeat("a", 64), "durationMs": observation.Source.Info.DurationMS, "width": 1920, "height": 1080, "originalMeasurementProvenance": "", "segments": segments}}, "expected": expected})
			return nil
		})
		if err != nil {
			t.Fatal(id, renderFailure(err))
		}
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, style := range design.CaptionStyles() {
			for _, pace := range []string{"steady", "rapid"} {
				export(ratio+"-"+style.ID+"-"+pace, ratio, style.ID, pace, "default")
			}
		}
		for _, mode := range []string{"safe", "subject", "readable", "crop", "header", "owner", "previous", "crosscut", "zero-subject"} {
			export(ratio+"-"+mode, ratio, "bold", "steady", mode)
		}
		export(ratio+"-rapid-first", ratio, "bold", "rapid", "rapid-first")
	}
	speechAssets := []map[string]any{}
	for i, bytes := range [][]byte{narrated660, narrated990} {
		audio, err := llm.InspectSpeechAudio(t.Context(), bytes)
		if err != nil {
			t.Fatal(err)
		}
		speechAssets = append(speechAssets, map[string]any{"file": []string{"narration-660.mp3", "narration-990.mp3"}[i], "audioHash": audio.SHA256, "samples": audio.Samples, "sampleRate": audio.SampleRate, "channels": audio.Channels, "bytes": len(bytes)})
	}
	body, err := json.MarshalIndent(map[string]any{"version": 1, "syntheticOnly": true, "qualification": false, "componentVersion": clip.BrowserComponentVersion, "assetVersion": clip.BrowserAssetVersion, "cases": cases, "speechAssets": speechAssets}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "manifest.json"), append(body, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("exported %d raw unplaced inputs and independently measured native layouts", len(cases))
}
