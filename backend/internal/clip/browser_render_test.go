package clip_test

import (
	"math"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestReportedOutputContract(t *testing.T) {
	level := -16.0
	good := clip.RenderMeasurements{Width: 1080, Height: 1920, FrameRateNumerator: 30, FrameRateDenominator: 1, VideoFrames: 900, VideoCodec: "h264", VideoProfile: "High", HasAudio: true, AudioCodec: "aac", AudioRate: 48000, LoudnessLUFS: &level}
	r := clip.BrowserRender{Ratio: "vertical", DurationMS: 30000, Audio: true}
	for _, tc := range []struct {
		name, check string
		change      func(*clip.RenderMeasurements)
	}{
		{"pass", "", func(*clip.RenderMeasurements) {}},
		{"resolution", "render_output_canvas", func(m *clip.RenderMeasurements) { m.Width = 720 }},
		{"frame rate", "render_output_frame_rate", func(m *clip.RenderMeasurements) { m.FrameRateNumerator = 24; m.VideoFrames = 720 }},
		{"codec", "render_output_codec", func(m *clip.RenderMeasurements) { m.VideoCodec = "hevc" }},
		{"profile", "render_output_codec", func(m *clip.RenderMeasurements) { m.VideoProfile = "Baseline" }},
		{"audio", "render_output_audio", func(m *clip.RenderMeasurements) { m.HasAudio = false }},
		{"sample rate", "render_output_audio_rate", func(m *clip.RenderMeasurements) { m.AudioRate = 44100 }},
		{"loudness", "render_output_loudness", func(m *clip.RenderMeasurements) { v := -20.0; m.LoudnessLUFS = &v }},
		{"missing loudness", "render_output_loudness", func(m *clip.RenderMeasurements) { m.LoudnessLUFS = nil }},
		{"silent track", "", func(m *clip.RenderMeasurements) { m.Silent = true; m.LoudnessLUFS = nil }},
		{"duration", "render_output_duration", func(m *clip.RenderMeasurements) { m.VideoFrames = 890 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := good
			tc.change(&m)
			v, err := clip.CheckRenderMeasurements(clip.DefaultRenderConfig(clip.Environment{}), r, m)
			if err != nil {
				t.Fatal(err)
			}
			if tc.check == "" {
				if !v.Passed || len(v.Notices) != 0 {
					t.Fatal(v)
				}
				return
			}
			if v.Passed || len(v.Notices) != 1 || v.Notices[0].Reason != tc.check {
				t.Fatal(v)
			}
		})
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		m := good
		m.LoudnessLUFS = &bad
		if _, err := clip.CheckRenderMeasurements(clip.DefaultRenderConfig(clip.Environment{}), r, m); err == nil {
			t.Fatal("nonfinite measurement accepted")
		}
	}
}
