package clip_test

import (
	"github.com/postpilot/backend/internal/clip"
	"strings"
	"testing"
)

func TestBrowserBackgroundVerdictIsFiniteAndAdmissionBound(t *testing.T) {
	binding := strings.Repeat("a", 64)
	r := clip.BrowserRender{Ratio: "vertical", DurationMS: 1000, Composition: &clip.BrowserCompositionContract{Version: clip.BrowserCompositionVersion, SnapshotFingerprint: binding, Components: clip.BrowserComponentVersion, Fonts: clip.BrowserFontVersion, Assets: clip.BrowserAssetVersion}}
	good := clip.RenderMeasurements{CompositionVersion: clip.BrowserCompositionVersion, SnapshotFingerprint: binding, BackgroundVersion: clip.BrowserBackgroundVersion, BackgroundSnapshotFingerprint: binding, BackgroundDigest: strings.Repeat("b", 64), BackgroundComplete: true, BackgroundSampleCount: 3, Width: 1080, Height: 1920, FrameRateNumerator: 30, FrameRateDenominator: 1, VideoFrames: 30, VideoCodec: "h264", VideoProfile: "High"}
	for _, tc := range []struct {
		name   string
		change func(*clip.RenderMeasurements)
	}{
		{"missing evidence", func(m *clip.RenderMeasurements) { m.BackgroundComplete = false }},
		{"different admission", func(m *clip.RenderMeasurements) { m.BackgroundSnapshotFingerprint = strings.Repeat("c", 64) }},
		{"unknown sampling version", func(m *clip.RenderMeasurements) { m.BackgroundVersion = "foreign-v1" }},
		{"old layout", func(m *clip.RenderMeasurements) { m.SnapshotFingerprint = strings.Repeat("c", 64) }},
		{"missing digest", func(m *clip.RenderMeasurements) { m.BackgroundDigest = "" }},
		{"uppercase digest", func(m *clip.RenderMeasurements) { m.BackgroundDigest = strings.Repeat("B", 64) }},
		{"partial three-frame read", func(m *clip.RenderMeasurements) { m.BackgroundSampleCount = 2 }},
		{"negative read count", func(m *clip.RenderMeasurements) { m.BackgroundSampleCount = -3 }},
		{"unbounded reads", func(m *clip.RenderMeasurements) { m.BackgroundSampleCount = 2403 }},
		{"invented notice", func(m *clip.RenderMeasurements) {
			m.BackgroundNotices = []clip.PlanNotice{{CopyFallback: clip.CopyFallback{Reason: "unknown", ElementID: "caption"}, Action: "shortfall"}}
		}},
		{"missing notice owner", func(m *clip.RenderMeasurements) {
			m.BackgroundNotices = []clip.PlanNotice{{CopyFallback: clip.CopyFallback{Reason: "composition_contrast"}, Action: "shortfall"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := good
			tc.change(&m)
			if _, e := clip.CheckRenderMeasurements(clip.DefaultRenderConfig(clip.Environment{}), r, m); e == nil {
				t.Fatal("invalid background accepted")
			}
		})
	}
	m := good
	m.BackgroundNotices = []clip.PlanNotice{{CopyFallback: clip.CopyFallback{Reason: "composition_contrast", ElementID: "caption", CutID: "cut"}, Action: "shortfall"}}
	v, e := clip.CheckRenderMeasurements(clip.DefaultRenderConfig(clip.Environment{}), r, m)
	if e != nil || !v.Passed || len(v.Notices) != 1 || v.Notices[0].ElementID != "caption" {
		t.Fatal(v, e)
	}
	// Plans with only plated/absent text still produce explicit complete evidence.
	m = good
	m.BackgroundSampleCount = 0
	if v, e = clip.CheckRenderMeasurements(clip.DefaultRenderConfig(clip.Environment{}), r, m); e != nil || !v.Passed {
		t.Fatal(v, e)
	}
	r.Composition = nil
	if _, e = clip.CheckRenderMeasurements(clip.DefaultRenderConfig(clip.Environment{}), r, good); e == nil {
		t.Fatal("local evidence accepted on native legacy admission")
	}
}
