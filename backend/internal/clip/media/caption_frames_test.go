package media

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
