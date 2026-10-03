package media

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// timelinePlan joins four 1x cuts: a dissolve, a fade through black and a hard
// cut, which is every way compositionGraph puts two cuts together.
func timelinePlan() clip.EditPlan {
	return clip.EditPlan{Cuts: []clip.Cut{
		{ID: "a", SourceID: "one", StartMS: 0, EndMS: 1000},
		{ID: "b", SourceID: "two", StartMS: 0, EndMS: 1500, TransitionMS: design.Transition.FadeMS},
		{ID: "c", SourceID: "one", StartMS: 0, EndMS: 2000, TransitionMS: design.Transition.BlackMS},
		{ID: "d", SourceID: "two", StartMS: 0, EndMS: 1000},
	}}
}

// The sampler's frame arithmetic is the composition's: the offsets its xfades
// open at and the join are where the timeline puts each cut (CDS-36).
func TestTheFootageTimelineIsTheCompositionGraphs(t *testing.T) {
	cfg := renderConfig(t)
	plan := timelinePlan()
	timeline := newFootageTimeline(cfg, plan)
	frames, transitions := cutFrames(plan, cfg.FPS), planTransitions(plan)
	graph := compositionGraph(cfg, frames, transitions, "yuv444p")
	fade, black := transitionFrames(cfg, design.Transition.FadeMS), transitionFrames(cfg, design.Transition.BlackMS)
	if !strings.Contains(graph, "xfade=transition=fade:duration="+seconds(design.Transition.FadeMS)+":offset="+frameSeconds(timeline.starts[1], cfg.FPS)) ||
		!strings.Contains(graph, "xfade=transition=fadeblack:duration="+seconds(design.Transition.BlackMS)+":offset="+frameSeconds(timeline.starts[2], cfg.FPS)) ||
		!strings.Contains(graph, "[vx2][v3]concat") || !strings.Contains(graph, fmt.Sprintf("trim=end_frame=%d,", timeline.total)) {
		t.Fatalf("the timeline %+v is not the graph's %s", timeline, graph)
	}
	b, c, d := timeline.starts[1], timeline.starts[2], timeline.starts[3]
	for _, tc := range []struct {
		name  string
		frame int
		want  outputFootage
	}{
		{"inside the first cut", 10, outputFootage{outgoing: footageTake{cut: -1}, incoming: footageTake{0, 10}}},
		{"the dissolve opens", b, outputFootage{outgoing: footageTake{0, b}, incoming: footageTake{1, 0}, kind: "fade", progress: 1}},
		{"inside the dissolve", b + 2, outputFootage{outgoing: footageTake{0, b + 2}, incoming: footageTake{1, 2}, kind: "fade", progress: 1 - 2/float64(fade)}},
		{"after the dissolve", b + fade, outputFootage{outgoing: footageTake{cut: -1}, incoming: footageTake{1, fade}}},
		{"inside the fade through black", c + 7, outputFootage{outgoing: footageTake{1, c + 7 - b}, incoming: footageTake{2, 7}, kind: "fadeblack", progress: 1 - 7/float64(black)}},
		{"the last frame before the join", d - 1, outputFootage{outgoing: footageTake{cut: -1}, incoming: footageTake{2, d - 1 - c}}},
		{"the join", d, outputFootage{outgoing: footageTake{cut: -1}, incoming: footageTake{3, 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := timeline.at(tc.frame)
			if !ok || got.outgoing != tc.want.outgoing || got.incoming != tc.want.incoming || got.kind != tc.want.kind || math.Abs(got.progress-tc.want.progress) > 1e-9 {
				t.Fatalf("frame %d is %+v, want %+v", tc.frame, got, tc.want)
			}
		})
	}
	if _, ok := timeline.at(timeline.total); ok {
		t.Fatal("a frame past the clip was composed")
	}
}

func grey(level uint8, samples int) []uint8 {
	return slices.Repeat([]uint8{level, level, level}, samples)
}

// FFmpeg 9.0.1's xfade from a 200 grey to an 80 grey over the composition's
// yuv444p footage, read back as RGB: a dissolve over 6 frames and a fade through
// black over 9. The dissolve is the dissolve of the two means; the fade through
// black blends to a black below video black and clips, which the pixel model
// follows within a few levels.
func TestTransitionsBlendAsTheCompositionsXfadeDoes(t *testing.T) {
	for k, want := range []float64{200, 180, 160, 140, 120, 100} {
		read := groundRead{footage: outputFootage{kind: "fade", progress: 1 - float64(k)/6}, means: [2][3]float64{{200. / 255, 200. / 255, 200. / 255}, {80. / 255, 80. / 255, 80. / 255}}}
		if got := read.mean()[0] * 255; math.Abs(got-want) > 1 {
			t.Fatalf("dissolve frame %d: %.2f, xfade gave %.0f", k, got, want)
		}
	}
	for k, want := range []float64{200, 61, 0, 0, 5, 22, 40, 55, 67} {
		if got := fadeBlackMean(grey(200, 4), grey(80, 4), 1-float64(k)/9)[0] * 255; math.Abs(got-want) > 3 {
			t.Fatalf("fade through black frame %d: %.2f, xfade gave %.0f", k, got, want)
		}
	}
}

func solidPNG(t *testing.T, path string, level uint8) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1080, 1920))
	for i := range img.Pix {
		img.Pix[i] = level
		if i%4 == 3 {
			img.Pix[i] = 255
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// Each cut is asked for exactly the frames the reads take from it, through its
// own chain, and a read inside a dissolve is the dissolve of what its two cuts
// gave.
func TestTheSamplerTakesEachCutsOwnFrames(t *testing.T) {
	plan := timelinePlan()
	var runner *fakeRunner
	runner = &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		level := uint8(0)
		if slices.Contains(c.Args, "white.mp4") || slices.ContainsFunc(c.Args, func(a string) bool { return strings.HasSuffix(a, "/white.mp4") }) {
			level = 255
		}
		for _, arg := range c.Args {
			if strings.HasSuffix(arg, ".png") {
				solidPNG(t, arg, level)
			}
		}
		return nil, nil
	}}
	a := newAdapter(t, runner)
	r := testRenderer(t, a)
	canvas, _ := clip.ClipCanvas("vertical")
	timeline := newFootageTimeline(r.cfg, plan)
	b := timeline.starts[1]
	region := clip.Region{X: 100, Y: 800, Width: 880, Height: 300}
	s := groundSampler{canvas: canvas}
	for _, frame := range []int{10, b + 2, b + 16} {
		footage, _ := timeline.at(frame)
		s.reads = append(s.reads, groundRead{visual: 0, footage: footage, region: region})
	}
	if !slices.Equal(s.needs(0), []int{10, b + 2}) || !slices.Equal(s.needs(1), []int{2, 16}) || len(s.needs(2)) != 0 {
		t.Fatalf("the cuts are asked for %v %v %v", s.needs(0), s.needs(1), s.needs(2))
	}
	if err := a.WithWorkspace(t.Context(), "ground-cuts", func(ws clip.MediaWorkspace) error {
		for i, name := range []string{"white.mp4", "black.mp4"} {
			path := ws.Path + "/" + name
			if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
				return err
			}
			if err := r.sampleCut(t.Context(), ws, &s, i, plan.Cuts[i], clip.MediaSource{Path: path}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		chains := 0
		for i, arg := range call.Args {
			if arg == "-vf" {
				chains++
				if !strings.Contains(call.Args[i+1], "trim=start_frame=") || !strings.Contains(call.Args[i+1], coverChain(canvas, clip.Point{})) {
					t.Fatalf("a frame was not cut through the cut's own chain: %s", call.Args[i+1])
				}
			}
		}
		if chains != 2 {
			t.Fatalf("one decode of a cut served %d frames, want 2: %v", chains, call.Args)
		}
	}
	grounds, err := s.grounds()
	if err != nil {
		t.Fatal(err)
	}
	p := 1 - 2/float64(transitionFrames(r.cfg, design.Transition.FadeMS))
	want := []float64{1, relativeLuminance(p, p, p), 0}
	for i, v := range grounds[0].Frames {
		if math.Abs(v-want[i]) > 1e-6 {
			t.Fatalf("frame %d measured %.6f, want %.6f", i, v, want[i])
		}
	}
}

// A measurement survives the trip a browser render's grounds make — out of the
// worker, into storage and back — and draws exactly what the measuring render
// drew: the region scrims, each line's ground and contrast notice, and one
// ground per phrase of a rapid caption.
func TestSampledGroundsDrawTheSameAfterARoundTrip(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	bright := Luminance{Mean: .82, Sigma: .01, R: .9, G: .9, B: .88, Frames: []float64{.81, .82, .83}}
	dark := Luminance{Mean: .05, Sigma: .01, R: .1, G: .1, B: .1, Frames: []float64{.05, .05, .05}}
	for _, tc := range []struct{ name, body string }{
		{"regions", `<clip version="1" intro="a" outro="b"><text id="caption" kind="fixed" role="caption" basis="output-start" start="3" end="12">현재 장면</text><text id="intro" kind="fixed" role="hook" basis="output-start"><row>첫 장면</row><row>기록</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"><row>또 올 곳</row><row>성수</row></text></clip>`},
		{"rapid", `<clip version="1" pace="rapid" styles="neon"><scene id="scene"><text id="caption" kind="ai" role="caption" basis="cut">여기 진짜 좋아요</text></scene></clip>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := declaredPlan(t, tc.body, "vertical")
			if tc.name == "rapid" {
				plan.CaptionStyles = []string{"neon"}
				plan.Portable.Elements[0].Resolved.Text = "여기 진짜 좋아요"
				plan.Portable.Elements[0].OwnerEdited = true
				plan.Portable.Elements[0].Phrases = []clip.EditablePhrase{{Text: "여기 진짜", StartMS: 200, EndMS: 1000}, {Text: "좋아요", StartMS: 1000, EndMS: 1600}}
			}
			measured, drawn := measuredDeclared(t, plan), measuredDeclared(t, plan)
			grounds := map[int]Luminance{}
			for i, visual := range measured.visuals {
				if sampledBounds(visual).Width > 0 {
					grounds[i] = bright
					if i%2 == 1 {
						grounds[i] = dark
					}
				}
			}
			for i, ground := range grounds {
				measured.visuals[i].ground = ground
				applyDeclaredGround(canvas, &measured.visuals[i])
			}
			data, err := json.Marshal(measured.sampledGrounds(grounds))
			if err != nil {
				t.Fatal(err)
			}
			var stored []clip.SampledGround
			if err := json.Unmarshal(data, &stored); err != nil {
				t.Fatal(err)
			}
			drawn.applyGrounds(canvas, stored)
			scrims := 0
			for i := range measured.visuals {
				want, got := measured.visuals[i], drawn.visuals[i]
				if !reflect.DeepEqual(want.ground, got.ground) || !reflect.DeepEqual(want.manifest, got.manifest) || !reflect.DeepEqual(want.region, got.region) || !reflect.DeepEqual(want.info, got.info) {
					t.Fatalf("visual %d (%s) drew differently after the trip", i, want.manifest.InstanceID)
				}
				for _, part := range got.manifest.Parts {
					if part.Kind == "scrim" {
						scrims++
					}
				}
			}
			if len(grounds) == 0 || tc.name == "regions" && scrims == 0 {
				t.Fatalf("the fixture measured %d grounds and drew %d scrims", len(grounds), scrims)
			}
		})
	}
}
