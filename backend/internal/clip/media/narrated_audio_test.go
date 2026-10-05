package media

import (
	"context"
	_ "embed"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/worker"
	"github.com/postpilot/backend/internal/llm"
)

//go:embed testdata/narration-660.mp3
var narrated660 []byte

//go:embed testdata/narration-990.mp3
var narrated990 []byte

func fixtureNarration(t *testing.T, plan *clip.EditPlan) map[string][]byte {
	t.Helper()
	binding := strings.Repeat("a", 64)
	plan.Narration = &clip.NarrationPlan{Enabled: true, VoiceID: "fixture-voice", BindingDigest: binding, VolumePermille: 750}
	files := map[string][]byte{}
	for i, data := range [][]byte{narrated660, narrated990} {
		audio, err := llm.InspectSpeechAudio(t.Context(), data)
		if err != nil {
			t.Fatal(err)
		}
		id := []string{"speech-one", "speech-two"}[i]
		text := []string{"first fixture", "second fixture"}[i]
		ref := clip.SpeechRef{AssetID: id, VoiceID: "fixture-voice", BindingDigest: binding, InputHash: clip.SpokenInputHash(text), SettingsHash: strings.Repeat("b", 64), AudioHash: audio.SHA256, ProfileID: "fixture-profile", ProfileRevision: 1, Samples: audio.Samples, SampleRate: audio.SampleRate, Channels: audio.Channels}
		start := []int{1000, 4800}[i]
		plan.Narration.Segments = append(plan.Narration.Segments, clip.SpokenSegment{ID: []string{"spoken-1", "spoken-2"}[i], Text: text, TextRevision: 1, InputHash: ref.InputHash, StartMS: start, EndMS: start + ref.DurationMS(), Speech: &ref})
		files[id] = data
	}
	return files
}
func TestNarrationAudioGraphKeepsNaturalRateAndIndependentSourceDip(t *testing.T) {
	p := clip.EditPlan{DurationMS: 16000, Cuts: []clip.Cut{{StartMS: 0, EndMS: 16000}}}
	fixtureNarration(t, &p)
	gain := 250
	p.SourceVolumePermille = &gain
	graph := narrationAudioGraph(clip.DefaultRenderConfig(clip.Environment{}), p, true, map[string]int{"spoken-1": 1, "spoken-2": 2}, 480)
	for _, want := range []string{"[0:a:0]volume=0.250000[source]", "volume=0.750000,adelay=48000S:all=1", "adelay=230400S:all=1", "amix=inputs=3", "end_sample=768000"} {
		if !strings.Contains(graph, want) {
			t.Fatal(graph)
		}
	}
	if strings.Contains(graph, "atempo") || strings.Contains(graph, "afade") {
		t.Fatal("speech was transformed", graph)
	}
}
func toneLevel(pcm []byte, start, duration, frequency float64) float64 {
	first := int(start * 48000)
	n := int(duration * 48000)
	re, im := 0.0, 0.0
	for i := 0; i < n; i++ {
		at := (first + i) * 4
		if at+2 > len(pcm) {
			return 0
		}
		sample := float64(int16(binary.LittleEndian.Uint16(pcm[at:]))) / 32768
		angle := 2 * math.Pi * frequency * float64(i) / 48000
		re += sample * math.Cos(angle)
		im += sample * math.Sin(angle)
	}
	return 2 * math.Hypot(re, im) / float64(n)
}
func TestNarratedExportSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("matching CPU worker image")
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
	transfer := &parityArtifacts{originals: map[string]string{}, dir: t.TempDir(), outputs: map[string]string{}}
	task := clip.MediaTask{Version: clip.MediaContractVersion}
	var sources []clip.RenderSource
	err = a.WithWorkspace(t.Context(), "narrated-fixtures", func(ws clip.MediaWorkspace) error {
		built, paths, _, err := qaFootage(t, a, ws, cfg)
		if err != nil {
			return err
		}
		for _, s := range built[:2] {
			dest := filepath.Join(transfer.dir, s.ID+".mp4")
			if err := copyIdentityFile(paths[s.ID], dest); err != nil {
				return err
			}
			transfer.originals[s.ID] = dest
			m := parityMetadata(t, dest, s.Info)
			s.Fingerprint = m.Fingerprint
			sources = append(sources, s)
			task.Sources = append(task.Sources, clip.MediaTaskSource{ID: s.ID, SourceMetadata: m, Info: s.Info})
		}
		return nil
	})
	if err != nil {
		t.Fatal(renderFailure(err))
	}
	executor := worker.NewExecutor(a, r, transfer, cfg)
	for _, tc := range []struct {
		name, ratio string
		mixed       bool
	}{{"voice-only-vertical", "vertical", false}, {"voice-only-square", "square", false}, {"mixed-horizontal", "horizontal", true}} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := qaPlan(qaClipCases()[1], sources)
			if err != nil {
				t.Fatal(err)
			}
			plan.Ratio = tc.ratio
			for i := range plan.Cuts {
				for _, s := range sources {
					if s.ID == plan.Cuts[i].SourceID {
						plan.Cuts[i].Fingerprint = s.Fingerprint
					}
				}
			}
			if !tc.mixed {
				plan.SourceAudio = nil
			}
			gain := 250
			plan.SourceVolumePermille = &gain
			files := fixtureNarration(t, &plan)
			plan, _, err = r.Layout(t.Context(), plan, sources)
			if err != nil {
				t.Fatal(renderFailure(err))
			}
			frozen := task
			frozen.Plan, err = clip.EncodeEditPlan(plan)
			if err != nil {
				t.Fatal(err)
			}
			frozen.Render = clip.FreezeMediaRenderInputs(plan)
			for _, segment := range plan.Narration.Segments {
				ref := segment.Speech
				data := files[ref.AssetID]
				dest := filepath.Join(transfer.dir, ref.AssetID+".mp3")
				if err = os.WriteFile(dest, data, 0600); err != nil {
					t.Fatal(err)
				}
				transfer.originals[ref.AssetID] = dest
				frozen.Speech = append(frozen.Speech, clip.MediaTaskSpeech{AssetID: ref.AssetID, AudioHash: ref.AudioHash, Bytes: int64(len(data))})
			}
			raw, err := executor.Execute(t.Context(), parityWork(t, clip.MediaRender, frozen))
			if err != nil {
				t.Fatal(renderFailure(err))
			}
			result, err := mediacodec.DecodeResult(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Speech, clip.RequestedSpeech(plan)) || !result.Outputs[0].Info.HasAudio {
				t.Fatal("lost narration provenance/audio", result)
			}
			output := transfer.outputs["result"]
			cmd := exec.CommandContext(t.Context(), cfg.FFmpegPath, "-hide_banner", "-v", "error", "-i", output, "-map", "0:a:0", "-ar", "48000", "-ac", "2", "-f", "s16le", "pipe:1")
			pcm, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			for _, sample := range []struct{ at, hz, other float64 }{{1.1, 660, 990}, {2.7, 660, 990}, {4.9, 990, 660}, {6.5, 990, 660}} {
				level := toneLevel(pcm, sample.at, .1, sample.hz)
				other := toneLevel(pcm, sample.at, .1, sample.other)
				if level < .025 || other > level*.15 {
					t.Fatalf("missing/order/truncated voice at %.2f: %.5f / %.5f", sample.at, level, other)
				}
			}
			if tc.mixed {
				if toneLevel(pcm, 3.6, .1, 330) < .005 {
					t.Fatal("source gain lost")
				}
			} else {
				if toneLevel(pcm, 3.6, .1, 330) > .001 {
					t.Fatal("unrequested source audio")
				}
			}
			err = a.WithWorkspace(t.Context(), "narrated-measure", func(ws clip.MediaWorkspace) error {
				path := filepath.Join(ws.Path, "output.mp4")
				if err := copyIdentityFile(output, path); err != nil {
					return err
				}
				l, err := r.measureLoudness(t.Context(), ws, path)
				if err == nil && (l.Silent || math.Abs(l.I+16) > 1 || l.TP > -1.45) {
					t.Fatalf("loudness %+v", l)
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("CLIP_DIAGNOSTIC_FIXTURES"); dir != "" {
				if err = os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err = copyIdentityFile(output, filepath.Join(dir, tc.name+".mp4")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// Context import also pins the loader signature in compile-time coverage.
var _ clip.RenderSpeechLoader = func(context.Context, string, func(string) error) error { return nil }
