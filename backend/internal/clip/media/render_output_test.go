package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// The probe document a conforming delivery produces. Each case below breaks
// exactly one property of it, so a rejection can be attributed to that property
// and to nothing else.
func deliveredProbe(t *testing.T) map[string]any {
	t.Helper()
	canvas, err := clip.ClipCanvas("vertical")
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"streams": []any{
			map[string]any{"index": 0, "codec_type": "video", "codec_name": "h264", "profile": h264Profile, "width": canvas.Width, "height": canvas.Height, "avg_frame_rate": "30/1", "sample_aspect_ratio": "1:1", "pix_fmt": "yuv420p", "duration": "25.333333"},
			map[string]any{"index": 1, "codec_type": "audio", "codec_name": "aac", "profile": "LC", "channels": 2, "sample_rate": "48000", "duration": "25.333333"},
		},
		"format": map[string]any{"format_name": "mov,mp4,m4a,3gp,3g2,mj2", "duration": "25.333333"},
	}
}

func probeRunner(t *testing.T, doc map[string]any, decodedMicros, frames int) *fakeRunner {
	t.Helper()
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		if filepath.Base(c.Binary) == "ffprobe" {
			return encoded, nil
		}
		return fmt.Appendf(nil, "frame=%d\nout_time_us=%d\nprogress=end\n", frames, decodedMicros), nil
	}}
}

func TestRenderedOutputRejectionNamesTheProperty(t *testing.T) {
	canvas, err := clip.ClipCanvas("vertical")
	if err != nil {
		t.Fatal(err)
	}
	// 760 frames at 30 fps is the 25334 ms plan every case below is judged
	// against; a case only says otherwise when the delivery is genuinely short.
	for _, tc := range []struct {
		name, check string
		micros      int
		frames      int
		values      map[string]int
		break_      func(map[string]any)
	}{
		{name: "conforming", micros: 25333333},
		{
			// The production refusal: an AAC track declares its codec padding,
			// so the container reads 66 ms longer than the clip plays while the
			// video track is exact. Jobs b12a4bcd and d66ec47e.
			name: "padded audio declaration", micros: 25400000,
			break_: func(d map[string]any) {
				d["format"].(map[string]any)["duration"] = "25.400000"
				d["streams"].([]any)[1].(map[string]any)["duration"] = "25.400000"
			},
		},
		{
			name: "canvas", check: "render_output_canvas", micros: 25333333,
			values: map[string]int{"width": 720, "height": canvas.Height, "expected_width": canvas.Width, "expected_height": canvas.Height},
			break_: func(d map[string]any) { video(d)["width"] = 720 },
		},
		{
			name: "rotation", check: "render_output_rotation", micros: 25333333,
			values: map[string]int{"rotation": 270},
			break_: func(d map[string]any) {
				// Stored sideways, so the probe's own swap still lands on the
				// canvas and only the rotation is left to reject.
				video(d)["width"], video(d)["height"] = canvas.Height, canvas.Width
				video(d)["side_data_list"] = []any{map[string]any{"rotation": 270.0}}
			},
		},
		{
			name: "pixel format", check: "render_output_pixel_format", micros: 25333333,
			break_: func(d map[string]any) { video(d)["pix_fmt"] = "yuv444p" },
		},
		{
			name: "aspect", check: "render_output_aspect", micros: 25333333,
			break_: func(d map[string]any) { video(d)["sample_aspect_ratio"] = "" },
		},
		{
			name: "frame rate", check: "render_output_frame_rate", micros: 25333333,
			values: map[string]int{"frame_rate_numerator": 24, "frame_rate_denominator": 1, "expected_fps": 30},
			break_: func(d map[string]any) { video(d)["avg_frame_rate"] = "24/1" },
		},
		{
			name: "audio", check: "render_output_audio", micros: 25333333,
			break_: func(d map[string]any) { d["streams"] = []any{video(d)} },
		},
		{
			name: "short video track", check: "render_output_duration", micros: 23333333, frames: 700,
			values: map[string]int{"duration_ms": 23333, "expected_duration_ms": 25334, "video_frames": 700, "decoded_duration_ms": 23333, "container_duration_ms": 23333},
			break_: func(d map[string]any) {
				d["format"].(map[string]any)["duration"] = "23.333333"
				video(d)["duration"] = "23.333333"
				d["streams"].([]any)[1].(map[string]any)["duration"] = "23.333333"
			},
		},
		{
			name: "codec", check: "render_output_codec", micros: 25333333,
			values: map[string]int{"stream_index": 0},
			break_: func(d map[string]any) { video(d)["codec_name"] = "hevc" },
		},
		{
			name: "audio rate", check: "render_output_audio_rate", micros: 25333333,
			values: map[string]int{"audio_rate": 44100, "expected_audio_rate": 48000},
			break_: func(d map[string]any) { d["streams"].([]any)[1].(map[string]any)["sample_rate"] = "44100" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := deliveredProbe(t)
			if tc.break_ != nil {
				tc.break_(doc)
			}
			frames := tc.frames
			if frames == 0 {
				frames = 760
			}
			a := newAdapter(t, probeRunner(t, doc, tc.micros, frames))
			r := testRenderer(t, a)
			plan := clip.EditPlan{Ratio: "vertical", DurationMS: 25334}
			err := a.WithWorkspace(t.Context(), "validate", func(ws clip.MediaWorkspace) error {
				output := filepath.Join(ws.Path, "clip-result.mp4")
				if err := os.WriteFile(output, []byte("delivered"), 0600); err != nil {
					return err
				}
				_, err := r.validateRenderedOutput(t.Context(), ws, output, plan, true, nil)
				return err
			})
			if tc.check == "" {
				if err != nil {
					t.Fatalf("conforming delivery rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("non-conforming delivery accepted")
			}
			d, ok := clip.DiagnosticFromError(err)
			if !ok {
				t.Fatal("rejection carries no diagnostic")
			}
			if d.Check != tc.check || d.Phase != "render" {
				t.Fatalf("diagnostic is %q/%q, want %q/render", d.Check, d.Phase, tc.check)
			}
			// A name outside the allowlist reaches the owner as "unknown", which
			// is the very failure this split exists to end.
			if clip.SafeAttemptCheck(d.Check) != tc.check {
				t.Fatalf("check %q does not survive the allowlist", tc.check)
			}
			safe := clip.SafeAttemptValues(d.Values)
			for key, want := range tc.values {
				if safe[key] != want {
					t.Fatalf("value %q is %d, want %d (kept: %v)", key, safe[key], want, safe)
				}
			}
			if len(safe) != len(tc.values) {
				t.Fatalf("kept values %v, want exactly %v", safe, tc.values)
			}
		})
	}
}

func video(doc map[string]any) map[string]any {
	return doc["streams"].([]any)[0].(map[string]any)
}
