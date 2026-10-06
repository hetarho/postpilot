package media

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// This opt-in exporter makes only synthetic footage and native component assets.
// It has no database, provider, account, job or object-store connection.
func TestBrowserBenchmarkFixtureExport(t *testing.T) {
	root := os.Getenv("CLIP_BROWSER_BENCHMARK_DIR")
	if root == "" || os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("browser reference fixtures are generated explicitly in the media runtime")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("fixture output directory must be empty", err)
	}
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 15 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := r.RuntimeProfile(t.Context(), "cpu")
	if err != nil {
		t.Fatal(err)
	}
	var info clip.MediaInfo
	sourcePath := filepath.Join(root, "source.mp4")
	if err = a.WithWorkspace(t.Context(), "browser-benchmark-source", func(ws clip.MediaWorkspace) error {
		path := filepath.Join(ws.Path, "source.mp4")
		_, err := a.run(t.Context(), ws, cfg.FFmpegPath,
			"-hide_banner", "-nostdin", "-v", "error", "-filter_complex_threads", "1",
			"-f", "lavfi", "-i", "color=c=0x2E4C8A:s=1280x720:r=60",
			"-f", "lavfi", "-i", "color=c=white:s=160x160:r=60",
			"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
			"-filter_complex", "[0:v][1:v]overlay=x='(W-w)*mod(t,4)/4':y='H/2-h/2'[v]",
			"-map", "[v]", "-map", "2:a", "-t", "20", "-c:v", "libx264",
			"-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-ar", "48000", "-ac", "2", path)
		if err != nil {
			return err
		}
		info, err = a.Probe(t.Context(), ws, path)
		if err != nil {
			return err
		}
		return copyIdentityFile(path, sourcePath)
	}); err != nil {
		t.Fatal(renderFailure(err))
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(sourceBytes)
	fingerprint := hex.EncodeToString(sum[:])
	source := clip.RenderSource{ID: "source", Fingerprint: fingerprint, Info: info}
	bundle := map[string]any{
		"version": 1, "provenance": "synthetic-only", "frameRate": 30,
		"nativeProfile": profile.RuntimeManifest,
	}
	var cases []map[string]any
	selectedStyles := strings.Split(os.Getenv("CLIP_BROWSER_BENCHMARK_STYLES"), ",")
	selectedRatios := strings.Split(os.Getenv("CLIP_BROWSER_BENCHMARK_RATIOS"), ",")
	all := func(values []string, value string) bool {
		return values[0] == "" || slices.Contains(values, value)
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, style := range design.CaptionStyles() {
			id := ratio + "-" + style.ID
			fixture := map[string]any{
				"id": id, "ratio": ratio, "durationMs": 15000, "styles": []string{style.ID},
				"audio": "none", "prepared": false,
				"nativeFrames": []string{"entrance", "settled", "effect-extreme", "exit"},
			}
			cases = append(cases, fixture)
			if !all(selectedStyles, style.ID) || !all(selectedRatios, ratio) {
				continue
			}
			if err := os.Mkdir(filepath.Join(root, id), 0755); err != nil {
				t.Fatal(err)
			}
			plan := declaredPlan(t, `<clip version="1" styles="`+style.ID+`"><text id="caption" kind="fixed" role="caption" style="`+style.ID+`" position="bottom" basis="output-start" start="1" end="3">자막 움직임 확인</text></clip>`, ratio)
			plan.Cuts[0].Fingerprint = fingerprint
			plan.CaptionStyles = []string{style.ID}
			plan, _, err = r.Layout(t.Context(), plan, []clip.RenderSource{source})
			if err != nil {
				t.Fatal(id, renderFailure(err))
			}
			started := time.Now()
			preview, err := r.PreparePreview(t.Context(), plan, []clip.RenderSource{source}, nil, 0, clip.DefaultGenerationConfig(clip.Environment{}).Preview)
			if err != nil {
				t.Fatal(id, renderFailure(err))
			}
			assets := []map[string]any{}
			sheets := map[string][]map[string]any{}
			for i, asset := range preview.Assets {
				file := fmt.Sprintf("%s/asset-%03d.png", id, i)
				if err := os.WriteFile(filepath.Join(root, file), asset.PNG, 0644); err != nil {
					t.Fatal(err)
				}
				assets = append(assets, map[string]any{
					"key": asset.Key, "instanceId": asset.InstanceID, "file": file,
					"x": asset.X, "y": asset.Y, "width": asset.Width, "height": asset.Height,
					"startMs": asset.StartMS, "endMs": asset.EndMS, "inMs": asset.InMS,
					"outMs": asset.OutMS, "dy": asset.DY, "layer": asset.Layer,
					"representativeFrame": asset.RepresentativeFrame,
				})
				if !asset.RepresentativeFrame {
					continue
				}
				for offset, ordinal := 0, 0; offset != -1; ordinal++ {
					page, err := r.PrepareCaptionFrames(t.Context(), plan, []clip.RenderSource{source}, asset.InstanceID, offset, clip.DefaultGenerationConfig(clip.Environment{}).Preview)
					if err != nil {
						t.Fatal(id, renderFailure(err))
					}
					file := fmt.Sprintf("%s/sheet-%03d-%03d.png", id, i, ordinal)
					if err := os.WriteFile(filepath.Join(root, file), page.Sheet, 0644); err != nil {
						t.Fatal(err)
					}
					sheets[asset.InstanceID] = append(sheets[asset.InstanceID], map[string]any{
						"file": file, "cellWidth": page.CellWidth, "cellHeight": page.CellHeight,
						"columns": page.Columns, "cells": page.Cells, "x": page.X, "y": page.Y,
						"firstFrame": page.FirstFrame, "frameOffset": page.FrameOffset, "nextOffset": page.NextOffset,
					})
					offset = page.NextOffset
				}
			}
			fixture["nativeAssetPreparationMs"] = time.Since(started).Milliseconds()
			fixture["prepared"], fixture["assets"], fixture["sheets"] = true, assets, sheets
			fixture["source"] = map[string]any{
				"file": "source.mp4", "fingerprint": fingerprint, "bytes": len(sourceBytes),
				"durationMs": info.DurationMS, "width": info.Width, "height": info.Height,
				"frameRateNumerator": info.FrameRateNumerator, "frameRateDenominator": info.FrameRateDenominator,
			}
			fixture["plan"] = map[string]any{
				"durationMs": plan.DurationMS,
				"cuts": []map[string]any{{
					"id": "cut", "sourceId": "source", "fingerprint": fingerprint,
					"startMs": 0, "endMs": plan.DurationMS, "transitionMs": 0,
					"playbackRatePermille": 1000, "volumePermille": 1000, "copies": []any{},
					"focal": map[string]float64{"x": .5, "y": .5},
				}},
				"sourceAudio": []map[string]any{{"sourceId": "source", "fingerprint": fingerprint, "retainOriginalAudio": false}},
			}
			t.Logf("prepared %s native-assets=%dms", id, fixture["nativeAssetPreparationMs"])
		}
	}
	for _, extra := range []struct {
		id, audio string
		duration  int
	}{
		{"all-styles-60s", "none", 60000}, {"rates-and-transitions", "source", 15000},
		{"narration-only", "narration", 15000}, {"source-and-narration", "mixed", 15000},
	} {
		styles := []string{}
		for _, style := range design.CaptionStyles() {
			styles = append(styles, style.ID)
		}
		cases = append(cases, map[string]any{"id": extra.id, "ratio": "vertical", "durationMs": extra.duration, "styles": styles, "audio": extra.audio, "prepared": false})
	}
	bundle["cases"] = cases
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
}
