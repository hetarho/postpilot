package media

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// movingBlock writes a source whose picture changes every frame: a block
// crossing a ground, so a frame taken one early or one late measures differently.
func movingBlock(t *testing.T, a *Adapter, ws clip.MediaWorkspace, name, ground, block, size, period string) string {
	t.Helper()
	path := filepath.Join(ws.Path, name)
	args := []string{"-v", "error", "-f", "lavfi", "-i", "color=c=" + ground + ":s=" + size + ":r=30", "-f", "lavfi", "-i", "color=c=" + block + ":s=" + size + ":r=30",
		"-filter_complex", "[1:v]crop=iw:ih/4[b];[0:v][b]overlay=x=0:y='(H-h)*mod(t," + period + ")/" + period + "'[v]", "-map", "[v]",
		"-t", "5", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", path}
	if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, args...); err != nil {
		t.Fatal(err)
	}
	return path
}

// The frames the sampler rebuilds from the originals are the composed footage's
// (CLIP-192): the same pixels wherever one cut stands alone, because every
// intermediate is lossless — on a cut at a non-1x rate and on both sides of a
// hard cut — the same dissolve, and a fade through black within the few levels
// its pixel model allows.
func TestRenderSmokeSamplesGroundsAsTheComposedFootage(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	t.Parallel()
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 10 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	canvas, _ := clip.ClipCanvas("vertical")
	if err := a.WithWorkspace(t.Context(), "ground-parity", func(ws clip.MediaWorkspace) error {
		sources := map[string]clip.MediaSource{
			"one": {SourceID: "one", Path: movingBlock(t, a, ws, "one.mp4", "black", "white", "1280x720", "2")},
			"two": {SourceID: "two", Path: movingBlock(t, a, ws, "two.mp4", "0xE0E0E0", "0x202020", "720x1280", "3")},
		}
		plan := clip.EditPlan{Ratio: "vertical", Cuts: []clip.Cut{
			{ID: "a", SourceID: "one", StartMS: 0, EndMS: 2000, Focal: clip.Point{X: .3, Y: .5}},
			{ID: "b", SourceID: "two", StartMS: 500, EndMS: 3500, PlaybackRatePermille: 1500, TransitionMS: design.Transition.FadeMS, Focal: clip.Point{X: .5, Y: .5}},
			{ID: "c", SourceID: "one", StartMS: 1000, EndMS: 3000, TransitionMS: design.Transition.BlackMS, Focal: clip.Point{X: .7, Y: .4}},
			{ID: "d", SourceID: "two", StartMS: 0, EndMS: 1000, Focal: clip.Point{X: .5, Y: .5}},
		}}
		// The composition's footage, built exactly as a server render builds it.
		timeline := newCutTimeline(r.cfg.FPS, plan)
		frames, transitions := timeline.frames, timeline.transitions
		cuts, cleanup := []string{}, []string{}
		for i, cut := range plan.Cuts {
			video := filepath.Join(ws.Path, fmt.Sprintf("bare-%04d.mp4", i))
			if err := r.renderBareFootage(t.Context(), ws, canvas, cut, sources[cut.SourceID], frames[i], video); err != nil {
				return err
			}
			cuts = append(cuts, video)
		}
		raw := filepath.Join(ws.Path, "composition-footage.mp4")
		args, err := r.compositionInputsFormat(t.Context(), ws, cuts, frames, transitions, "", nil, &cleanup, "yuv444p")
		if err != nil {
			return err
		}
		if err := r.runRender(t.Context(), ws, raw, append(args, r.encodeProfile(false, 0, "yuv444p")...)); err != nil {
			return err
		}
		b, c, d := timeline.starts[1], timeline.starts[2], timeline.starts[3]
		picks := []int{15, b + 3, b + 20, c + 1, c + 4, c + 7, d - 1, d, d + 5}
		region := clip.Region{X: 90, Y: 600, Width: 900, Height: 700}
		// Before: an output-side seek into the composed footage, the frame a
		// render used to read.
		offsets := make([]int, len(picks))
		for i, frame := range picks {
			offsets[i] = frame * 1000 / r.cfg.FPS
		}
		composed := clip.MediaSource{Path: raw, Info: clip.MediaInfo{Width: canvas.Width, Height: canvas.Height, DurationMS: timeline.total * 1000 / r.cfg.FPS}}
		before, err := r.sampleFrames(t.Context(), ws, canvas, composed, clip.Point{X: .5, Y: .5}, offsets, 0)
		if err != nil {
			return err
		}
		// After: each frame rebuilt from the originals.
		s := groundSampler{canvas: canvas}
		for i, frame := range picks {
			footage, ok := timeline.at(frame)
			if !ok {
				return fmt.Errorf("frame %d is outside the clip", frame)
			}
			s.reads = append(s.reads, groundRead{visual: i, footage: footage, region: region})
		}
		for i, cut := range plan.Cuts {
			if err := r.sampleCut(t.Context(), ws, &s, i, cut, sources[cut.SourceID]); err != nil {
				return err
			}
		}
		after, err := s.grounds()
		if err != nil {
			return err
		}
		spread := 0.0
		for i, frame := range picks {
			want, _, _, _ := regionLuminance(before[i], region)
			got := after[i].Frames[0]
			tolerance := 1e-4
			switch s.reads[i].footage.kind {
			case "fade":
				tolerance = 2e-3
			case "fadeblack":
				tolerance = 2e-2
			}
			t.Logf("frame %d %-9s originals %.5f composed %.5f", frame, s.reads[i].footage.kind, got, want)
			if math.Abs(got-want) > tolerance {
				t.Errorf("frame %d (%+v) measured %.5f from the originals, %.5f in the composed footage", frame, s.reads[i].footage, got, want)
			}
			if composedGround := measureFrames(before[i:i+1], []clip.Region{region}); composedGround.Scrim() != after[i].Scrim() || composedGround.AccentWhite() != after[i].AccentWhite() {
				t.Errorf("frame %d decides a scrim or accent differently from the originals", frame)
			}
			if i > 0 {
				spread = max(spread, math.Abs(got-after[i-1].Frames[0]))
			}
		}
		if spread < .05 {
			t.Errorf("the fixture's frames barely differ (%.4f), so a frame taken early or late would pass", spread)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// legacyGroundRead is how the sampler read a cut's frames before T559: one
// output per frame, each cover-scaling every frame of the cut's span before
// trimming to its own one.
func legacyGroundRead(t *testing.T, r *Rendering, ws clip.MediaWorkspace, s *groundSampler, index int, cut clip.Cut, source clip.MediaSource) {
	t.Helper()
	locals := s.needs(index)
	if len(locals) == 0 {
		return
	}
	args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.DecodeThreads), "-protocol_whitelist", "file,pipe", "-ss", seconds(cut.StartMS), "-i", source.Path)
	paths := make([]string, len(locals))
	for i, local := range locals {
		paths[i] = filepath.Join(ws.Path, fmt.Sprintf("legacy-%d-%d.png", index, local))
		chain := "trim=duration=" + seconds(cut.SourceSpanMS()) + "," + rateChain(cut.Rate(), r.cfg.FPS) + "," + coverChain(s.canvas, cut.Focal) +
			",setsar=1,trim=start_frame=" + strconv.Itoa(local) + ":end_frame=" + strconv.Itoa(local+1) + ",setpts=PTS-STARTPTS,format=yuv444p"
		args = append(args, "-map", "0:V:0", "-vf", chain, "-frames:v", "1", "-c:v", "png", "-threads", strconv.Itoa(r.media.cfg.EncodeThreads), paths[i])
	}
	if _, err := r.media.run(t.Context(), ws, r.media.cfg.FFmpegPath, args...); err != nil {
		t.Fatal(err)
	}
	for i, path := range paths {
		frame, err := readFrame(path)
		_ = os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
		s.take(index, locals[i], frame)
	}
}

// The grounds a server render now reads from each cut's bare footage, and the
// ones a browser render's sampling reads through the frame-picking chain, are
// the legacy read's to the bit: every read's mean colour over the whole canvas
// and under a caption region, a fade through black's pixels, and so every
// scrim, accent and contrast decision made from them — on a cut at a non-1x
// rate, both sides of a dissolve, a fade through black and a hard cut.
func TestRenderSmokeReadsTheGroundsTheLegacyChainRead(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	t.Parallel()
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 10 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	canvas, _ := clip.ClipCanvas("vertical")
	if err := a.WithWorkspace(t.Context(), "ground-legacy", func(ws clip.MediaWorkspace) error {
		sources := map[string]clip.MediaSource{
			"one": {SourceID: "one", Path: movingBlock(t, a, ws, "one.mp4", "black", "white", "1280x720", "2")},
			"two": {SourceID: "two", Path: movingBlock(t, a, ws, "two.mp4", "0xE0E0E0", "0x202020", "720x1280", "3")},
		}
		plan := clip.EditPlan{Ratio: "vertical", Cuts: []clip.Cut{
			{ID: "a", SourceID: "one", StartMS: 0, EndMS: 2000, Focal: clip.Point{X: .3, Y: .5}},
			{ID: "b", SourceID: "two", StartMS: 500, EndMS: 3500, PlaybackRatePermille: 1500, TransitionMS: design.Transition.FadeMS, Focal: clip.Point{X: .5, Y: .5}},
			{ID: "c", SourceID: "one", StartMS: 1000, EndMS: 3000, TransitionMS: design.Transition.BlackMS, Focal: clip.Point{X: .7, Y: .4}},
			{ID: "d", SourceID: "two", StartMS: 0, EndMS: 1000, Focal: clip.Point{X: .5, Y: .5}},
		}}
		timeline := newCutTimeline(r.cfg.FPS, plan)
		b, c, d := timeline.starts[1], timeline.starts[2], timeline.starts[3]
		whole := clip.Region{Width: float64(canvas.Width), Height: float64(canvas.Height)}
		caption := clip.Region{X: 90, Y: 600, Width: 900, Height: 700}
		reads := []groundRead{}
		for i, frame := range []int{0, 15, b + 3, b + 20, c + 1, c + 4, c + 7, d - 1, d, d + 5} {
			footage, ok := timeline.at(frame)
			if !ok {
				return fmt.Errorf("frame %d is outside the clip", frame)
			}
			reads = append(reads, groundRead{visual: 2 * i, footage: footage, region: whole}, groundRead{visual: 2*i + 1, footage: footage, region: caption})
		}
		legacy, browser, server := groundSampler{canvas: canvas, reads: slices.Clone(reads)}, groundSampler{canvas: canvas, reads: slices.Clone(reads)}, groundSampler{canvas: canvas, reads: slices.Clone(reads)}
		frames := timeline.frames
		for i, cut := range plan.Cuts {
			legacyGroundRead(t, r, ws, &legacy, i, cut, sources[cut.SourceID])
			if err := r.sampleCut(t.Context(), ws, &browser, i, cut, sources[cut.SourceID]); err != nil {
				return err
			}
			bare := filepath.Join(ws.Path, fmt.Sprintf("bare-%04d.mp4", i))
			if err := r.renderBareFootage(t.Context(), ws, canvas, cut, sources[cut.SourceID], frames[i], bare); err != nil {
				return err
			}
			if err := r.sampleFootage(t.Context(), ws, &server, i, bare); err != nil {
				return err
			}
			if err := os.Remove(bare); err != nil {
				return err
			}
		}
		want, err := legacy.grounds()
		if err != nil {
			return err
		}
		for _, got := range []struct {
			name    string
			sampler groundSampler
		}{{"browser", browser}, {"server", server}} {
			for i, read := range got.sampler.reads {
				if read.means != legacy.reads[i].means || !reflect.DeepEqual(read.pixels, legacy.reads[i].pixels) || read.filled != legacy.reads[i].filled {
					t.Errorf("%s read %d (%+v) took other pixels than the legacy read: %v, want %v", got.name, i, read.footage, read.means, legacy.reads[i].means)
				}
			}
			grounds, err := got.sampler.grounds()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(grounds, want) {
				t.Errorf("%s grounds %+v, legacy %+v", got.name, grounds, want)
			}
			for visual, ground := range grounds {
				if ground.Scrim() != want[visual].Scrim() || ground.AccentWhite() != want[visual].AccentWhite() || ground.Hex() != want[visual].Hex() {
					t.Errorf("%s visual %d decides a scrim, accent or contrast colour differently", got.name, visual)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A centred outro over a bright ground takes scrim.radial in a server render,
// and the server's measurement a browser render is handed draws the same scrim
// (CDS-32, CDS-44, CLIP-192).
func TestRenderSmokeScrimsAnOutroOverABrightGround(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	t.Parallel()
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 10 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	canvas, _ := clip.ClipCanvas("vertical")
	if err := a.WithWorkspace(t.Context(), "bright-outro", func(ws clip.MediaWorkspace) error {
		path := filepath.Join(ws.Path, "source.mp4")
		if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=0xF4EFE6:s=1280x720:r=30", "-t", "16", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", path); err != nil {
			return err
		}
		info, err := a.Probe(t.Context(), ws, path)
		if err != nil {
			return err
		}
		plan := declaredPlan(t, `<clip version="1" outro="b"><text id="outro" kind="fixed" role="ending" basis="output-end"><row>또 올 곳</row><row>성수</row></text></clip>`, "vertical")
		sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: info}}
		load := func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
			return consume(clip.MediaSource{SourceID: "source", Fingerprint: "fp", Info: info, Path: path})
		}
		scrimmed := func(parts []design.Element) bool {
			for _, part := range parts {
				if part.Kind == "scrim" {
					return true
				}
			}
			return false
		}
		result, err := r.Render(t.Context(), ws, plan, sources, load)
		if err != nil {
			return err
		}
		rendered := false
		for _, e := range result.Elements {
			rendered = rendered || e.Role == "ending" && scrimmed(e.Parts)
		}
		if !rendered {
			return fmt.Errorf("the server render left the bright outro unscrimmed: %+v", result.Elements)
		}
		_ = os.Remove(result.Path)
		grounds, err := r.SampleGrounds(t.Context(), ws, plan, sources, load)
		if err != nil {
			return err
		}
		layout, err := r.layoutComposition(t.Context(), ws, plan)
		if err != nil {
			return err
		}
		layout.applyGrounds(canvas, grounds)
		for _, visual := range layout.visuals {
			if visual.manifest.Role == "ending" && (!scrimmed(visual.manifest.Parts) || visual.region.Radial == nil && visual.region.Scrim == nil) {
				return fmt.Errorf("the browser render's grounds left the outro unscrimmed: %+v", grounds)
			}
		}
		// The assets a browser render draws carry that scrim in the outro's own
		// raster and the server render's contrast verdict; the editing preview's
		// do not, because it reads no footage.
		cfg := clip.DefaultGenerationConfig(clip.Environment{}).Preview
		editing, err := r.PreparePreview(t.Context(), plan, sources, nil, 0, cfg)
		if err != nil {
			return err
		}
		bound, err := r.PrepareGroundedPreview(t.Context(), plan, sources, grounds, nil, 0, cfg)
		if err != nil {
			return err
		}
		outro := func(p clip.PreparedPreview) (clip.PreviewAsset, bool) {
			for _, a := range p.Assets {
				if strings.HasPrefix(a.InstanceID, "outro") {
					return a, true
				}
			}
			return clip.PreviewAsset{}, false
		}
		plain, ok1 := outro(editing)
		drawn, ok2 := outro(bound)
		if !ok1 || !ok2 || drawn.Width*drawn.Height <= plain.Width*plain.Height || bytes.Equal(drawn.PNG, plain.PNG) {
			return fmt.Errorf("the browser render's outro asset (%dx%d) is not drawn over its scrim (the editing preview's is %dx%d)", drawn.Width, drawn.Height, plain.Width, plain.Height)
		}
		noticed := false
		for _, e := range result.Elements {
			if e.Role == "ending" {
				for _, part := range e.Parts {
					noticed = noticed || part.Kind == "copy" && part.ContrastNotice
				}
			}
		}
		if drawn.ContrastNotice != noticed || plain.ContrastNotice {
			return fmt.Errorf("the contrast verdicts disagree: server %v, browser %v, editing %v", noticed, drawn.ContrastNotice, plain.ContrastNotice)
		}
		for _, parity := range bound.Parity {
			if parity == clip.PreviewSourceContrast {
				return fmt.Errorf("a sampled render still calls its contrast final-only")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
