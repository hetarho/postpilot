package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestOutputCaptionSamplingIncludesTheLastExistingFrame(t *testing.T) {
	if got := declaredSampleWindow(0, 15000, 15000, 30); got != ([2]int{0, 14967}) {
		t.Fatal(got)
	}
	if got := declaredSampleWindow(14950, 15000, 15000, 30); got != ([2]int{14950, 14967}) {
		t.Fatal(got)
	}
	if got := declaredSampleWindow(1000, 4000, 15000, 30); got != ([2]int{1000, 4000}) {
		t.Fatal(got)
	}
}

func TestCompositionWindowsCoverRapidCuesWithoutRestartingPersistentFades(t *testing.T) {
	visuals := []declaredVisual{{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", StartMS: 0, EndMS: 90000}}}
	for i := range 2400 {
		visuals = append(visuals, declaredVisual{text: clip.PortableText{Pace: "rapid"}, manifest: clip.CompositionElement{Role: "caption", StartMS: i * 37, EndMS: (i + 1) * 37}})
	}
	windows := compositionWindows(visuals, 2700, 30, 8)
	if len(windows) < 200 {
		t.Fatal("rapid cues forced through whole-output passes")
	}
	end := 0
	for _, window := range windows {
		if window.StartFrame != end || window.EndFrame <= window.StartFrame || window.EndFrame > 2700 {
			t.Fatalf("gap or overlap: %+v", window)
		}
		for _, index := range window.Layers {
			v := visuals[index]
			m := elementMotion(v.manifest, v.text.Pace)
			for _, b := range []int{window.StartFrame, window.EndFrame} {
				for _, phase := range [][2]int{{v.manifest.StartMS, v.manifest.StartMS + m.InMS}, {v.manifest.EndMS - m.OutMS, v.manifest.EndMS}} {
					if phase[0]*30 < b*1000 && b*1000 < phase[1]*30 {
						t.Fatal("partition splits an animation")
					}
				}
			}
		}
		end = window.EndFrame
	}
	if end != 2700 {
		t.Fatal("lost final frames")
	}
}

func TestWindowGraphUsesLocalTimeAndFiniteImageLoops(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	visuals := []declaredVisual{{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", StartMS: 0, EndMS: 15000}}}
	window := overlayWindow{StartFrame: 307, EndFrame: 330, Layers: []int{0}}
	graph := declaredOverlayGraph(cfg, window, visuals, make([]captionLayer, len(visuals)), true)
	if !strings.Contains(graph, "trim=start_frame=7:end_frame=30") || !strings.Contains(graph, "loop=loop=22:size=1:start=0") || strings.Contains(graph, "fade=t=in") || strings.Contains(graph, "pow(") || !strings.Contains(graph, "format=yuv444p[v]") {
		t.Fatalf("restarted or unbounded overlay: %s", graph)
	}
	second := declaredOverlayGraph(cfg, window, visuals, make([]captionLayer, len(visuals)), false)
	if !strings.Contains(second, "trim=start_frame=0:end_frame=23") {
		t.Fatal("applied source offset to a local intermediate")
	}
}

func TestDeclaredAudioDucksOnlyTheAuthoredHookInterval(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	plain := declaredAudioGraph(cfg, []int{450}, []int{0}, nil)
	if strings.Contains(plain, "-6") || strings.Contains(plain, "volume=") {
		t.Fatal("implicit hook dip")
	}
	elements := []clip.CompositionElement{{Role: "hook", StartMS: 4100, EndMS: 5300}, {Role: "hook", StartMS: 5000, EndMS: 6200}, {Role: "ending", StartMS: 13000, EndMS: 15000}}
	graph := declaredAudioGraph(cfg, []int{450}, []int{0}, elements)
	if strings.Count(graph, "volume=volume=") != 1 || !strings.Contains(graph, "gte(t,4.100)*lt(t,5.300)") || !strings.Contains(graph, "gte(t,5.000)*lt(t,6.200)") || strings.Contains(graph, "gte(t,13.000)") {
		t.Fatalf("incorrect hook union: %s", graph)
	}
	cut := clip.Cut{StartMS: 1000, EndMS: 16000, Focal: clip.Point{X: .5, Y: .5}}
	canvas, _ := clip.ClipCanvas("vertical")
	video := bareFootageGraph(cfg, canvas, cut, 450)
	if strings.Contains(video, "overlay") || !strings.Contains(video, "format=yuv444p[v]") {
		t.Fatal("cut is not lossless bare footage")
	}
	audio := bareAudioGraph(cfg, cut, true, 450)
	if !strings.Contains(audio, "first_pts=0") || !strings.Contains(audio, "atrim=end_sample=720000") || strings.Contains(audio, "loudnorm") {
		t.Fatal("source audio clock or normalization moved")
	}
}

// The two overlay paths, side by side and golden: a clip whose elements fit one
// window is delivered straight out of the overlay graph, and one that needs
// several keeps today's windowed pieces and its separate delivery pass.
func TestOverlayDeliveryGraphGoldens(t *testing.T) {
	const total = 450 // 15 s at 30 fps
	one := []declaredVisual{
		{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "badge", StartMS: 0, EndMS: 15000}},
		{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", StartMS: 1000, EndMS: 6000}},
		{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", StartMS: 6000, EndMS: 12000}},
	}
	many := make([]declaredVisual, 0, 20)
	for i := range 20 {
		many = append(many, declaredVisual{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", StartMS: i * 700, EndMS: i*700 + 700}})
	}
	for _, tc := range []struct {
		name    string
		visuals []declaredVisual
		audio   bool
	}{{"fused", one, false}, {"fused-audio", one, true}, {"windowed", many, true}} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
				return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("piece"), 0600)
			}}
			a := newAdapter(t, fake)
			r := testRenderer(t, a)
			measured := loudness{I: -20, TP: -3, LRA: 7, Threshold: -30, Offset: 0.1}
			if err := a.WithWorkspace(t.Context(), "overlay-plan", func(ws clip.MediaWorkspace) error {
				raw := filepath.Join(ws.Path, "composition-footage.mp4")
				layers := make([]captionLayer, len(tc.visuals))
				for i := range layers {
					layers[i] = captionLayer{Plate: filepath.Join(ws.Path, fmt.Sprintf("declared-%04d.png", i))}
				}
				assembled := ""
				if tc.audio {
					assembled = filepath.Join(ws.Path, "composition-audio.wav")
				}
				windows := compositionWindows(tc.visuals, total, r.cfg.FPS, r.cfg.OverlayBatchSize)
				var recorded strings.Builder
				fused := len(windows) == 1 && len(windows[0].Layers) <= r.cfg.OverlayBatchSize
				if fused != (tc.name != "windowed") {
					t.Fatalf("%s took the wrong path: %d windows", tc.name, len(windows))
				}
				if fused {
					args := r.deliveredOverlayArgs(raw, windows[0], tc.visuals, layers, assembled, &measured)
					fmt.Fprintf(&recorded, "delivered inputs=%d\n%s\n", strings.Count(strings.Join(args, " "), " -i "), args[slices.Index(args, "-filter_complex")+1])
					golden(t, "overlay-"+tc.name+".filter", recorded.String())
					return nil
				}
				var cleanup []string
				pieces, pieceFrames, err := r.overlayComposition(t.Context(), ws, raw, windows, tc.visuals, layers, &cleanup)
				if err != nil {
					return err
				}
				for _, pass := range fake.calls {
					fmt.Fprintf(&recorded, "window inputs=%d\n%s\n", strings.Count(strings.Join(pass.Args, " "), " -i "), pass.Args[slices.Index(pass.Args, "-filter_complex")+1])
				}
				args, err := r.compositionInputs(t.Context(), ws, pieces, pieceFrames, make([]int, len(pieces)), assembled, &measured, &cleanup)
				if err != nil {
					return err
				}
				fmt.Fprintf(&recorded, "delivery inputs=%d\n%s\n", strings.Count(strings.Join(args, " "), " -i "), args[slices.Index(args, "-filter_complex")+1])
				golden(t, "overlay-"+tc.name+".filter", recorded.String())
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
