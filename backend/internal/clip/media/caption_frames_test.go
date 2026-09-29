package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// The sheet prefixes every cell's ids with its own frame, and a sequence file
// prefixes nothing: compare the drawings rather than the identifiers.
var anyID = regexp.MustCompile(`(id="|url\(#)[^")]+`)

func withoutIDs(svg string) string { return anyID.ReplaceAllString(svg, "$1") }

func framesPlan(t *testing.T, style string) (clip.EditPlan, clip.Canvas) {
	t.Helper()
	plan := declaredPlan(t, `<clip version="1" styles="`+style+`"><scene id="scene"><text id="caption" kind="ai" role="caption" basis="cut">Describe.</text></scene></clip>`, "vertical")
	plan.Portable.Elements[0].Resolved.Text = "여기 진짜 좋아요"
	plan.CaptionStyles = []string{style}
	canvas, _ := clip.ClipCanvas("vertical")
	return plan, canvas
}

// CLIP-159: a browser render draws a sequence caption from the server's own
// drawing of each output frame, and those are the frames the server render
// itself writes — same crop, same progress, same painter (CDS-85).
func TestACaptionSheetCarriesTheRendersOwnFrames(t *testing.T) {
	a, r := measured(t)
	plan, canvas := framesPlan(t, "word-pop")
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	frames, err := r.PrepareCaptionFrames(t.Context(), plan, sources, "caption/cut", 0, clip.DefaultGenerationConfig(clip.Environment{}).Preview)
	if err != nil {
		t.Fatal(err)
	}
	if frames.Cells <= 1 || frames.CellWidth <= 0 || frames.CellHeight <= 0 || frames.Columns <= 0 || len(frames.Sheet) == 0 {
		t.Fatalf("%+v", frames)
	}
	// The same caption, drawn the way a render draws it.
	layout := measuredDeclared(t, plan)
	visual := captionVisual(t, layout)
	var written []string
	if err := a.WithWorkspace(t.Context(), "sheet-compare", func(ws clip.MediaWorkspace) error {
		sequence, err := r.captionSequence(t.Context(), ws, canvas, visual.copy, visual.caption, visual.manifest.StartMS, visual.manifest.EndMS, 0)
		if err != nil {
			return err
		}
		if sequence.StartFrame != frames.FirstFrame {
			t.Fatalf("the sheet starts at frame %d, the render at %d", frames.FirstFrame, sequence.StartFrame)
		}
		for _, name := range []string{"00001.png", "00002.png"} {
			data, err := os.ReadFile(filepath.Join(sequence.Dir, name))
			if err != nil {
				return err
			}
			written = append(written, string(data))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sheet := withoutIDs(string(frames.Sheet))
	for i, frame := range written {
		body := withoutIDs(frame)
		if at := strings.Index(body, "</defs>"); at >= 0 {
			body = body[at+len("</defs>"):]
		} else {
			body = body[strings.Index(body, ">")+1:]
		}
		body = strings.TrimSuffix(body, "</svg>")
		if !strings.Contains(sheet, body) {
			t.Fatalf("cell %d is not the frame the render writes", i)
		}
	}
	if frames.X != int(sequenceCrop(canvas, visual.caption).X) {
		t.Fatalf("the sheet places its cells at %d, the render at %v", frames.X, sequenceCrop(canvas, visual.caption).X)
	}
}

// CDS-81: a static style has ONE raster, which the draft preview already serves.
func TestCaptionFramesRefuseAStaticStyleAndAnUnknownCaption(t *testing.T) {
	_, r := measured(t)
	cfg := clip.DefaultGenerationConfig(clip.Environment{}).Preview
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	static, _ := framesPlan(t, "bold")
	if _, err := r.PrepareCaptionFrames(t.Context(), static, sources, "caption/cut", 0, cfg); err == nil {
		t.Fatal("a static style was served as frames")
	}
	sequence, _ := framesPlan(t, "neon")
	if _, err := r.PrepareCaptionFrames(t.Context(), sequence, sources, "nothing", 0, cfg); err == nil {
		t.Fatal("an unknown caption was served")
	}
	if _, err := r.PrepareCaptionFrames(t.Context(), sequence, sources, "caption/cut", 1<<20, cfg); err == nil {
		t.Fatal("a frame beyond the caption was served")
	}
}

// A run larger than the ceiling is answered with what fits and the frame to ask
// for next; the last run says there is no next.
func TestACaptionSheetPagesALongCaption(t *testing.T) {
	_, r := measured(t)
	plan, _ := framesPlan(t, "neon")
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	cfg := clip.DefaultGenerationConfig(clip.Environment{}).Preview
	cfg.MaxFrameCells = 4
	// Every output frame of the caption's own interval, and no more.
	visual := captionVisual(t, measuredDeclared(t, plan))
	first := visual.manifest.StartMS * 30 / 1000
	want := (visual.manifest.EndMS*30+999)/1000 - first
	seen, offset, runs := 0, 0, 0
	for offset != -1 {
		frames, err := r.PrepareCaptionFrames(t.Context(), plan, sources, "caption/cut", offset, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if frames.Cells > cfg.MaxFrameCells || frames.FrameOffset != offset || frames.FirstFrame != first {
			t.Fatalf("%+v", frames)
		}
		seen += frames.Cells
		offset = frames.NextOffset
		if runs++; runs > want {
			t.Fatal("the paging did not end")
		}
	}
	if seen != want || runs < 2 {
		t.Fatalf("a paged caption delivered %d of %d frames in %d runs", seen, want, runs)
	}
}

// A rapid phrase in a sequence style has nothing to animate, so the render draws
// it from ONE frame of its style (CDS-4). A browser render follows the draft
// preview's word on it: every phrase's asset is a single raster rather than a
// frame run, and the frame endpoint refuses it as it refuses a static style, so
// the two kinds cannot move a phrase differently (CLIP-157, CLIP-159).
func TestARapidSequenceCaptionIsServedAsTheRendersOneRaster(t *testing.T) {
	_, r, _ := previewMeasured(t, "vertical")
	plan := declaredPlan(t, `<clip version="1" pace="rapid" styles="neon"><scene id="scene"><text id="caption" kind="ai" role="caption" basis="cut">여기 진짜 좋아요</text></scene></clip>`, "vertical")
	plan.CaptionStyles = []string{"neon"}
	plan.Portable.Elements[0].Resolved.Text = "여기 진짜 좋아요"
	plan.Portable.Elements[0].OwnerEdited = true
	plan.Portable.Elements[0].Phrases = []clip.EditablePhrase{{Text: "여기 진짜", StartMS: 200, EndMS: 1000}, {Text: "좋아요", StartMS: 1000, EndMS: 1600}}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	var phrases []declaredVisual
	for _, visual := range measuredDeclared(t, plan).visuals {
		if visual.manifest.InstanceID == "caption/cut" {
			phrases = append(phrases, visual)
		}
	}
	if len(phrases) != 2 {
		t.Fatalf("the fixture laid out %d phrases", len(phrases))
	}
	for _, phrase := range phrases {
		if sequenceDrawn(phrase) {
			t.Fatalf("the render would animate the rapid phrase %q", phrase.copy.Text)
		}
	}
	preview, err := r.PreparePreview(t.Context(), plan, sources, []string{"caption/cut"}, 0, clip.DefaultGenerationConfig(clip.Environment{}).Preview)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Assets) != len(phrases) {
		t.Fatalf("the preview served %d assets for %d phrases", len(preview.Assets), len(phrases))
	}
	for i, asset := range preview.Assets {
		if asset.RepresentativeFrame || asset.InMS != 0 || asset.OutMS != 0 || asset.DY != 0 {
			t.Fatalf("phrase %d is served as a moving frame run: %+v", i, asset)
		}
		if asset.StartMS != phrases[i].manifest.StartMS || asset.EndMS != phrases[i].manifest.EndMS {
			t.Fatalf("phrase %d is served over %d-%d, the render holds it over %d-%d", i, asset.StartMS, asset.EndMS, phrases[i].manifest.StartMS, phrases[i].manifest.EndMS)
		}
	}
	if _, err := r.PrepareCaptionFrames(t.Context(), plan, sources, "caption/cut", 0, clip.DefaultGenerationConfig(clip.Environment{}).Preview); err == nil {
		t.Fatal("a rapid phrase was served as a frame run")
	}
}

// A run that fits the sheet's pixels but not one response's bytes is cut to
// what does and drawn again, never refused, and the paging still covers every
// frame; a single frame over the budget is the one refusal (CLIP-159). The
// budget is the response's own, less base64's third, because the browser reads
// the sheet out of a JSON response.
func TestACaptionSheetIsCutToWhatOneResponseCarries(t *testing.T) {
	_, r := measured(t)
	plan, _ := framesPlan(t, "ember")
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	cfg := clip.DefaultGenerationConfig(clip.Environment{}).Preview
	whole, err := r.PrepareCaptionFrames(t.Context(), plan, sources, "caption/cut", 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if whole.Cells < 6 {
		t.Fatalf("the fixture's first run holds %d frames", whole.Cells)
	}
	cfg.MaxResponseBytes = len(whole.Sheet)/3*4/3 + 1024
	budget := sheetByteBudget(cfg)
	visual := captionVisual(t, measuredDeclared(t, plan))
	first := visual.manifest.StartMS * 30 / 1000
	want := (visual.manifest.EndMS*30+999)/1000 - first
	seen, offset, runs := 0, 0, 0
	for offset != -1 {
		frames, err := r.PrepareCaptionFrames(t.Context(), plan, sources, "caption/cut", offset, cfg)
		if err != nil {
			t.Fatalf("run at %d: %v", offset, err)
		}
		if len(frames.Sheet) > budget || frames.Cells >= whole.Cells || frames.FrameOffset != offset {
			t.Fatalf("run at %d: %d cells, %d bytes against %d", offset, frames.Cells, len(frames.Sheet), budget)
		}
		seen += frames.Cells
		offset = frames.NextOffset
		if runs++; runs > want {
			t.Fatal("the paging did not end")
		}
	}
	if seen != want {
		t.Fatalf("the cut runs delivered %d of %d frames", seen, want)
	}
	cfg.MaxResponseBytes = 2048
	if _, err := r.PrepareCaptionFrames(t.Context(), plan, sources, "caption/cut", 0, cfg); !errors.Is(err, clip.ErrPreviewTooLarge) {
		t.Fatalf("a frame larger than a response was not refused: %v", err)
	}
}

// slowDrawing makes every rasterisation cost what a heavy style costs on a slow
// box, leaving measurement as it was.
type slowDrawing struct {
	inner Runner
	cost  time.Duration
}

func (s slowDrawing) Run(ctx context.Context, c Command) ([]byte, error) {
	if !slices.Contains(c.Args, "--query-all") {
		time.Sleep(s.cost)
	}
	return s.inner.Run(ctx, c)
}

// A run is cut to what the request's deadline leaves, so a style that is slow
// to draw pages in smaller runs instead of timing the browser render out.
func TestACaptionSheetIsCutToTheTimeLeft(t *testing.T) {
	a, r := measured(t)
	a.runner = slowDrawing{inner: a.runner, cost: 60 * time.Millisecond}
	plan, _ := framesPlan(t, "neon")
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	cfg := clip.DefaultGenerationConfig(clip.Environment{}).Preview
	ctx, cancel := context.WithTimeout(t.Context(), 700*time.Millisecond)
	defer cancel()
	frames, err := r.PrepareCaptionFrames(ctx, plan, sources, "caption/cut", 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// At 60 ms a drawing, 700 ms leaves room for a handful of frames, not the
	// thirty the pixels would allow.
	if frames.Cells < 1 || frames.Cells > 10 || frames.NextOffset != frames.Cells {
		t.Fatalf("a slow style's run: %d cells, next %d", frames.Cells, frames.NextOffset)
	}
}
