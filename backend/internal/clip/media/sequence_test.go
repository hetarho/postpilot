package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/platform/config"
)

// captionLayout lays one caption out the way the composition does, so a
// sequence test starts from a real measurement rather than an invented box.
func captionLayout(t *testing.T, style string) (*Adapter, *Rendering, clip.Canvas, clip.Copy, copyLayout) {
	t.Helper()
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	rule, ok := design.CaptionRule(style)
	if !ok {
		t.Fatal(style)
	}
	c := clip.Copy{Text: "여기 진짜 좋아요", Style: style, Anchor: rule.Anchor, Align: rule.Align, Accent: "coral", Keyword: "진짜"}
	var layout copyLayout
	if err := a.WithWorkspace(t.Context(), "sequence-layout", func(ws clip.MediaWorkspace) error {
		var err error
		layout, err = r.layoutCopy(t.Context(), ws, canvas, c)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return a, r, canvas, c, layout
}

// CDS-80: a sequence style draws one PNG per OUTPUT frame of its own interval,
// into the attempt's own workspace, cropped to the pixels it paints.
func TestASequenceStyleDrawsOnePNGPerOutputFrame(t *testing.T) {
	a, r, canvas, c, layout := captionLayout(t, "blur-in")
	if layout.Caption.Static() {
		t.Fatal("the fixture chose a static style")
	}
	if err := a.WithWorkspace(t.Context(), "sequence-frames", func(ws clip.MediaWorkspace) error {
		// 1.0 s at 30 fps is 30 frames, first to last.
		sequence, err := r.captionSequence(t.Context(), ws, canvas, c, layout, 2000, 3000, 3)
		if err != nil {
			return err
		}
		if sequence.Frames != 30 || sequence.StartFrame != 60 {
			return fmt.Errorf("%d frames from frame %d", sequence.Frames, sequence.StartFrame)
		}
		entries, err := os.ReadDir(sequence.Dir)
		if err != nil {
			return err
		}
		if len(entries) != 30 {
			return fmt.Errorf("%d files on disk", len(entries))
		}
		for i := range 30 {
			if _, err := os.Lstat(filepath.Join(sequence.Dir, fmt.Sprintf("%05d.png", i+1))); err != nil {
				return err
			}
		}
		// The layer is cropped to the caption's box plus the style's own bleed,
		// never to the whole canvas: a full-canvas layer per frame would reserve
		// ten megabytes of workspace for every frame of every caption (CLIP-33).
		crop := sequenceCrop(canvas, layout)
		if sequence.Origin != crop || crop.Width >= float64(canvas.Width) && crop.Height >= float64(canvas.Height) {
			return fmt.Errorf("uncropped layer %+v", crop)
		}
		if crop.X > layout.Region.X || crop.Y > layout.Region.Y {
			return fmt.Errorf("the crop cut into the caption: %+v vs %+v", crop, layout.Region)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The same plan draws the same frames twice, because the delivered clip is
// compared byte for byte (CLIP-125).
func TestASequenceDrawsTheSameFramesTwice(t *testing.T) {
	a, r, canvas, c, layout := captionLayout(t, "ember")
	read := func(name string) map[string]string {
		out := map[string]string{}
		if err := a.WithWorkspace(t.Context(), name, func(ws clip.MediaWorkspace) error {
			sequence, err := r.captionSequence(t.Context(), ws, canvas, c, layout, 0, 500, 0)
			if err != nil {
				return err
			}
			entries, err := os.ReadDir(sequence.Dir)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join(sequence.Dir, entry.Name()))
				if err != nil {
					return err
				}
				out[entry.Name()] = string(data)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	first, second := read("determinism-a"), read("determinism-b")
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("%d against %d frames", len(first), len(second))
	}
	for name, data := range first {
		if second[name] != data {
			t.Fatalf("%s differs between two runs of one caption", name)
		}
	}
}

// A sequence layer enters through image2 at the output frame rate and carries no
// loop and no fade: its motion is already drawn into every frame. A static one
// keeps the looped single plate, unchanged (CDS-81).
func TestTheOverlayGraphReadsASequenceWithImage2AndLoopsAStaticPlate(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	_, r := measured(t)
	visuals := []declaredVisual{
		{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", Style: "blur-in", StartMS: 1000, EndMS: 4000}},
		{text: clip.PortableText{Pace: "steady"}, manifest: clip.CompositionElement{Role: "caption", Style: "bold", StartMS: 5000, EndMS: 8000}},
	}
	layers := []captionLayer{
		{Sequence: &captionSequence{Pattern: "/w/seq-0000/%05d.png", Dir: "/w/seq-0000", Origin: clip.Region{X: 200, Y: 600, Width: 640, Height: 400}, Frames: 90, StartFrame: 30}},
		{Plate: "/w/declared-0001.png"},
	}
	window := overlayWindow{StartFrame: 0, EndFrame: 300, Layers: []int{0, 1}}
	graph := declaredOverlayGraph(cfg, window, visuals, layers, true)
	if !strings.Contains(graph, "[1:v:0]format=rgba,setpts=PTS+1.000000000/TB[layer0];") {
		t.Fatalf("the sequence was not shifted to its own start: %s", graph)
	}
	if !strings.Contains(graph, "[layer0]overlay=x=200:y=600:format=auto:shortest=0:eof_action=pass:enable='gte(t,1.000000000)*lt(t,4.000000000)'") {
		t.Fatalf("the sequence was not overlaid at its crop origin: %s", graph)
	}
	if strings.Contains(graph, "[1:v:0]format=rgba,loop") || strings.Count(graph, "fade=t=in") != 1 {
		t.Fatalf("the sequence was looped or faded a second time: %s", graph)
	}
	if !strings.Contains(graph, "[2:v:0]format=rgba,loop=loop=299:size=1:start=0,fade=t=in") {
		t.Fatalf("the static plate lost its loop: %s", graph)
	}
	// image2 at the output frame rate, with decoder threads bounded exactly as
	// they are on a single-frame input.
	args := r.layerInput(nil, layers[0], window)
	if strings.Join(args, " ") != fmt.Sprintf("-threads %d -framerate 30 -f image2 -start_number 1 -i /w/seq-0000/%%05d.png", r.media.cfg.DecodeThreads) {
		t.Fatalf("%v", args)
	}
	if got := strings.Join(r.layerInput(nil, layers[1], window), " "); !strings.HasSuffix(got, "-framerate 30 -i /w/declared-0001.png") {
		t.Fatalf("%v", got)
	}
	// A window that opens after the caption did resumes at the frame it reaches,
	// rather than replaying the entrance the owner already saw.
	later := overlayWindow{StartFrame: 45, EndFrame: 300, Layers: []int{0, 1}}
	if got := strings.Join(r.layerInput(nil, layers[0], later), " "); !strings.Contains(got, "-start_number 16 ") {
		t.Fatalf("a spanning sequence restarted: %v", got)
	}
	beyond := overlayWindow{StartFrame: 4000, EndFrame: 4300, Layers: []int{0}}
	if got := strings.Join(r.layerInput(nil, layers[0], beyond), " "); !strings.Contains(got, "-start_number 90 ") {
		t.Fatalf("a sequence was asked for a frame it never drew: %v", got)
	}
}

// CLIP-33: a sequence's frames are counted against the workspace budget like
// every other intermediate, and released as soon as the pass that read them is
// encoded.
func TestSequenceFramesAreBudgetedAndReleased(t *testing.T) {
	a, r, canvas, c, layout := captionLayout(t, "blur-in")
	if err := a.WithWorkspace(t.Context(), "sequence-budget", func(ws clip.MediaWorkspace) error {
		sequence, err := r.captionSequence(t.Context(), ws, canvas, c, layout, 0, 400, 0)
		if err != nil {
			return err
		}
		frames, err := directoryBytes(sequence.Dir, 1<<40)
		if err != nil {
			return err
		}
		if frames <= 0 {
			return fmt.Errorf("the sequence occupies nothing")
		}
		// The whole sequence, offered as one more request, is refused: the
		// accounting saw the frames rather than an empty directory entry.
		if err := a.capacity(ws, a.cfg.WorkspaceMaxBytes-frames+1); err == nil {
			return fmt.Errorf("the budget did not count %d bytes of frames", frames)
		}
		if err := releaseLayers([]captionLayer{{Sequence: &sequence}}); err != nil {
			return err
		}
		if _, err := os.Lstat(sequence.Dir); !os.IsNotExist(err) {
			return fmt.Errorf("the frames outlived the pass that read them: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
