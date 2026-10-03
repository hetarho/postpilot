package media

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestRenderedCaptionsStayBetweenProjectRegions(t *testing.T) {
	plan := narrationPlan(t,
		narrationText("narration-1", "첫 자막", 600, 3800),
		narrationText("narration-2", "끝 자막", 10500, 14500),
	)
	plan.IntroPreset, plan.OutroPreset = "cover", "b"
	regions := clip.ProjectRegions{
		Intro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{{ID: "intro-1", Text: "시작"}}},
		Outro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{{ID: "outro-1", Text: "마침"}}},
	}
	projected, _, err := clip.ProjectPlanRegions(plan, regions, plan.Design().RegionPresets())
	if err != nil {
		t.Fatal(err)
	}
	layout := measuredDeclared(t, projected)
	start, end := clip.CaptionBodyWindow(layout.plan)
	if start != 2500 || end != 12000 {
		t.Fatalf("unexpected caption body %d..%d", start, end)
	}
	if err := clip.VerifyCompositionManifest(layout.plan, layout.elements(), clip.DefaultCompositionLimits()); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, element := range layout.elements() {
		if element.Role != "caption" {
			continue
		}
		count++
		if element.StartMS < start || element.EndMS > end {
			t.Fatalf("caption %s overlaps a project region: %d..%d", element.ElementID, element.StartMS, element.EndMS)
		}
	}
	if count != 2 {
		t.Fatalf("rendered %d captions, want both body captions", count)
	}
	_, previewer, _ := previewMeasured(t, "vertical")
	preview, err := previewer.PreparePreview(t.Context(), projected,
		[]clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1080, Height: 1920}}},
		nil, 0, clip.PreviewConfig{MaxAssets: 16, MaxAssetBytes: 512 << 10, MaxResponseBytes: 8 << 20, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range preview.Assets {
		if strings.HasPrefix(asset.InstanceID, "narration-") && (asset.StartMS < start || asset.EndMS > end) {
			t.Fatalf("preview caption %s overlaps a region: %d..%d", asset.InstanceID, asset.StartMS, asset.EndMS)
		}
	}
	for i := range projected.Portable.Elements {
		if projected.Portable.Elements[i].Resolved.Element.ID == "narration-1" {
			projected.Portable.Elements[i].OwnerEdited = true
		}
	}
	_, err = clip.ResolvePortableIntervals(projected, clip.DefaultCompositionLimits())
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.Reason != clip.NoticeCaptionRegionOverlap {
		t.Fatalf("owner caption overlapping intro was not refused: %v", err)
	}
}

func TestRetainedFixedCaptionCannotCoverProjectRegions(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1"><text id="caption" kind="fixed" role="caption" basis="whole">본문 자막</text><text id="intro" kind="fixed" role="hook" basis="output-start"><row>시작</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"><row>마침</row></text></clip>`, "vertical")
	layout := measuredDeclared(t, plan)
	start, end := clip.CaptionBodyWindow(layout.plan)
	for _, element := range layout.elements() {
		if element.Role == "caption" && (element.StartMS != start || element.EndMS != end) {
			t.Fatalf("retained fixed caption covers a region: %d..%d, body %d..%d", element.StartMS, element.EndMS, start, end)
		}
	}
}
