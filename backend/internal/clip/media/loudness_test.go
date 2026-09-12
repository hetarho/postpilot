package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestLoudnessDriftCorrectsOnlyAudioFromOriginalPCM(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readings []string
		wantRuns int
		wantErr  bool
	}{
		{"already valid", []string{"-16.02"}, 0, false},
		{"silent", []string{"-inf"}, 0, false},
		{"twenty second originals", []string{"-14.97", "-16.01"}, 1, false},
		{"bounded correction", []string{"-14.9", "-14.9", "-14.9"}, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			measurements, corrections := 0, 0
			fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
				if c.CaptureStderr {
					i := tc.readings[min(measurements, len(tc.readings)-1)]
					measurements++
					return []byte(strings.Replace(loudnormLog, `"input_i" : "-23.45"`, `"input_i" : "`+i+`"`, 1)), nil
				}
				corrections++
				joined := strings.Join(c.Args, " ")
				if !strings.Contains(joined, "-c:v copy") || strings.Contains(joined, "libx264") || !strings.Contains(joined, "[1:a:0]loudnorm=") || !strings.Contains(joined, "atrim=end_sample=960000") || !strings.Contains(joined, "-c:a aac") {
					t.Fatal("audio repair must preserve the picture and exact timing", joined)
				}
				var inputs []string
				for i, arg := range c.Args {
					if arg == "-i" {
						inputs = append(inputs, c.Args[i+1])
					}
				}
				if len(inputs) != 2 || filepath.Base(inputs[1]) != "compose-audio.wav" {
					t.Fatal("re-encoded previous AAC instead of original PCM", inputs)
				}
				return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("corrected"), 0600)
			}}
			a := newAdapter(t, fake)
			r := testRenderer(t, a)
			err := a.WithWorkspace(t.Context(), "loudness", func(ws clip.MediaWorkspace) error {
				path, pcm := filepath.Join(ws.Path, "clip-result.mp4"), filepath.Join(ws.Path, "compose-audio.wav")
				if err := os.WriteFile(path, []byte("original video"), 0600); err != nil {
					return err
				}
				if err := os.WriteFile(pcm, []byte("original pcm"), 0600); err != nil {
					return err
				}
				err := r.finishLoudness(t.Context(), ws, path, pcm, loudness{I: -23.45, TP: -4.12, LRA: 7.3, Threshold: -33.6, Offset: .12}, 600)
				if (err != nil) != tc.wantErr {
					return fmt.Errorf("correction error %v, want error=%v", err, tc.wantErr)
				}
				files, _ := filepath.Glob(filepath.Join(ws.Path, "audio-correction-*"))
				if len(files) != 0 {
					t.Fatal("temporary correction leaked", files)
				}
				return nil
			})
			if err != nil || corrections != tc.wantRuns {
				t.Fatalf("corrections=%d want=%d err=%v", corrections, tc.wantRuns, err)
			}
		})
	}
}

// What FFmpeg actually prints: a log, then the filter's own JSON object, with
// every number quoted as a string.
const loudnormLog = `Input #0, wav, from 'compose-audio.wav':
  Duration: 00:00:15.00, bitrate: 1536 kb/s
  Stream #0:0: Audio: pcm_s16le ([1][0][0][0] / 0x0001), 48000 Hz, stereo, s16, 1536 kb/s
[Parsed_loudnorm_0 @ 0x600001a8c000]
{
	"input_i" : "-23.45",
	"input_tp" : "-4.12",
	"input_lra" : "7.30",
	"input_thresh" : "-33.60",
	"output_i" : "-16.02",
	"output_tp" : "-1.50",
	"output_lra" : "7.10",
	"output_thresh" : "-26.10",
	"normalization_type" : "dynamic",
	"target_offset" : "0.12"
}
`

func TestLoudnessMeasurementParsesTheReportAndFeedsTheSecondPass(t *testing.T) {
	got, err := parseLoudness([]byte(loudnormLog))
	if err != nil {
		t.Fatal(err)
	}
	if got != (loudness{I: -23.45, TP: -4.12, LRA: 7.3, Threshold: -33.6, Offset: 0.12}) {
		t.Fatalf("measured %+v", got)
	}
	// The measuring pass asks for the report and names no measured value; the
	// applying pass names every one of them and is linear because of it.
	first := loudnormFilter(nil)
	if !strings.Contains(first, "print_format=json") || strings.Contains(first, "measured_") || strings.Contains(first, "linear") {
		t.Fatal("the measuring pass is not a measurement", first)
	}
	second := loudnormFilter(&got)
	for _, want := range []string{"I=-16", "TP=-1.5", "LRA=11", "measured_I=-23.45", "measured_TP=-4.12", "measured_LRA=7.3", "measured_thresh=-33.6", "offset=0.12", "linear=true"} {
		if !strings.Contains(second, want) {
			t.Fatalf("second pass lost %q: %s", want, second)
		}
	}
	// CDS-35's target is the design system's, never a literal in the filter.
	if design.Audio.Loudnorm != (design.Loudnorm{I: -16, TP: -1.5, LRA: 11}) {
		t.Fatal("the loudness target moved", design.Audio.Loudnorm)
	}
}

// A track of digital silence is delivered as it is: no gain makes silence −16
// LUFS, so the second pass passes it through and V12 asks nothing of it.
func TestSilenceIsDeliveredRatherThanAmplified(t *testing.T) {
	got, err := parseLoudness([]byte(strings.Replace(loudnormLog, `"input_i" : "-23.45"`, `"input_i" : "-inf"`, 1)))
	if err != nil || !got.Silent || got.I != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	if loudnormFilter(&got) != "anull" {
		t.Fatal("normalised silence", loudnormFilter(&got))
	}
}

func TestLoudnessMeasurementRefusesWhatItCannotRead(t *testing.T) {
	for name, log := range map[string]string{
		"no report":      "Input #0, wav\nStream #0:0: Audio",
		"missing field":  strings.Replace(loudnormLog, `"target_offset" : "0.12"`, `"target_offset" : ""`, 1),
		"not an object":  "{",
		"truncated json": strings.Replace(loudnormLog, `"input_tp" : "-4.12",`, "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseLoudness([]byte(log)); err == nil {
				t.Fatal("accepted an unreadable measurement")
			}
		})
	}
}
