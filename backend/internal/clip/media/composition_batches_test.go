package media

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
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
	cfg := config.ClipRender(&config.Config{})
	visuals := []declaredVisual{{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", StartMS: 0, EndMS: 15000}}}
	window := overlayWindow{StartFrame: 307, EndFrame: 330, Layers: []int{0}}
	graph := declaredOverlayGraph(cfg, window, visuals, true)
	if !strings.Contains(graph, "trim=start_frame=7:end_frame=30") || !strings.Contains(graph, "loop=loop=22:size=1:start=0") || strings.Contains(graph, "fade=t=in") || strings.Contains(graph, "pow(") || !strings.Contains(graph, "format=yuv444p[v]") {
		t.Fatalf("restarted or unbounded overlay: %s", graph)
	}
	second := declaredOverlayGraph(cfg, window, visuals, false)
	if !strings.Contains(second, "trim=start_frame=0:end_frame=23") {
		t.Fatal("applied source offset to a local intermediate")
	}
}

func TestDeclaredAudioDucksOnlyTheAuthoredHookInterval(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
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
