package design_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// One caption, measured once, drawn by every sequence style: the geometry is
// identical across them so a golden difference is the style's own drawing and
// nothing else.
func sequenceFrame(style design.CaptionStyle, progress float64) design.CaptionFrame {
	role := style.Role()
	frame := design.CaptionFrame{
		Canvas: design.Size{Width: 1080, Height: 1920}, Style: style,
		Family: design.FontFamily(style.Face), Size: role.Size, Tracking: role.Tracking,
		Region:   design.Bounds{X: 290, Y: 640, Width: 500, Height: 100},
		Accent:   design.Accent["coral"],
		Progress: progress, DurationMS: 3200,
	}
	frame.Lines = []design.CaptionLine{{
		Text: "여기 진짜 좋아요", X: 290, Y: 720, Top: 640, Width: 500, Height: 100,
		Keyword: "진짜", KeywordX: 400, KeywordWidth: 120,
		Words: []design.CaptionWord{
			{Text: "여기", X: 290, Width: 140},
			{Text: "진짜", X: 450, Width: 140},
			{Text: "좋아요", X: 610, Width: 180},
		},
	}}
	return frame
}

func golden(t *testing.T, name, actual string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_CLIP_GOLDENS") == "1" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual != string(want) {
		t.Fatalf("%s changed\n got %s\nwant %s", name, actual, want)
	}
}

// Every sequence style draws, at three points of one interval, and what it draws
// is pinned: the motion CDS-4 says it declares has to be visible in the drawing,
// because the overlay chain adds none of its own to a sequence layer.
func TestSequenceCaptionFrameGoldens(t *testing.T) {
	if !design.SequenceStylesAreDrawable() {
		t.Fatal("a sequence-rendered style in the approved set has no painter")
	}
	for _, style := range design.CaptionStyles() {
		if style.Static() {
			if _, _, ok := design.DrawCaptionFrame(sequenceFrame(style, 0.5)); ok {
				t.Fatalf("%s is static and still draws its own frames", style.ID)
			}
			continue
		}
		body := &strings.Builder{}
		for _, progress := range []float64{0, 0.5, 1} {
			defs, drawn, ok := design.DrawCaptionFrame(sequenceFrame(style, progress))
			if !ok {
				t.Fatalf("%s did not draw", style.ID)
			}
			if strings.Contains(drawn, "feDisplacementMap") || strings.Contains(defs, "feDisplacementMap") {
				t.Fatalf("%s uses feDisplacementMap, which resvg 0.48.1 misplaces", style.ID)
			}
			// CDS-83: a filter computed in linearRGB draws a different colour
			// from the one the preview shows.
			if n := strings.Count(defs, "<filter"); n != strings.Count(defs, `color-interpolation-filters="sRGB"`) {
				t.Fatalf("%s has a filter that does not declare sRGB: %s", style.ID, defs)
			}
			// Nothing may be set in a face the style does not name (CDS-18).
			for face, family := range design.Faces {
				if face != style.Face && strings.Contains(drawn, `font-family="`+family+`"`) {
					t.Fatalf("%s reached for %s", style.ID, family)
				}
			}
			body.WriteString(defs + "\n" + drawn + "\n")
		}
		golden(t, "sequence-"+style.ID+".svg", body.String())
	}
}

// A sequence style has to draw the same frame twice, because the delivered clip
// is compared byte for byte (CDS-7, CLIP-125).
func TestSequenceCaptionFramesAreDeterministic(t *testing.T) {
	for _, style := range design.CaptionStyles() {
		if style.Static() {
			continue
		}
		for _, progress := range []float64{0.17, 0.62, 0.93} {
			firstDefs, first, _ := design.DrawCaptionFrame(sequenceFrame(style, progress))
			secondDefs, second, _ := design.DrawCaptionFrame(sequenceFrame(style, progress))
			if first != second || firstDefs != secondDefs {
				t.Fatalf("%s drew two different frames for the same instant", style.ID)
			}
		}
	}
}

// A style with no accent draws none: CDS-15 approves seven colours and a style
// may not invent an eighth when a project chose none.
func TestASequenceStyleWithoutAnAccentPaintsNone(t *testing.T) {
	for _, style := range design.CaptionStyles() {
		if style.Static() {
			continue
		}
		frame := sequenceFrame(style, 0.5)
		frame.Accent = ""
		_, drawn, ok := design.DrawCaptionFrame(frame)
		if !ok {
			t.Fatal(style.ID)
		}
		for _, accent := range design.Accent {
			if strings.Contains(drawn, accent) {
				t.Fatalf("%s painted %s with no accent chosen", style.ID, accent)
			}
		}
	}
}
