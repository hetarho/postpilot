package media

import (
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

func renderConfig(t *testing.T) clip.RenderConfig {
	t.Helper()
	// Every face, from the same variables the image sets (CDS-17): the renderer
	// refuses to start without all of them, so a config that names only some
	// fails the constructor rather than any render.
	return clip.DefaultRenderConfig(clip.Environment{
		ResvgPath: path("CLIP_RESVG_PATH", "/usr/local/bin/resvg"),
		FontPaths: map[string]string{
			"wantedsans":        path("CLIP_FONT_PATH", "/usr/share/postpilot-fonts/wantedsans/WantedSansVariable.ttf"),
			"paperlogy":         path("CLIP_FONT_PAPERLOGY_PATH", "/usr/share/postpilot-fonts/paperlogy/Paperlogy-8ExtraBold.ttf"),
			"jua":               path("CLIP_FONT_JUA_PATH", "/usr/share/postpilot-fonts/jua/Jua-Regular.ttf"),
			"nanummyeongjo":     path("CLIP_FONT_NANUM_MYEONGJO_PATH", "/usr/share/postpilot-fonts/nanummyeongjo/NanumMyeongjo-Regular.ttf"),
			"nanummyeongjo-800": path("CLIP_FONT_NANUM_MYEONGJO_BOLD_PATH", "/usr/share/postpilot-fonts/nanummyeongjo/NanumMyeongjo-ExtraBold.ttf"),
		},
	})
}

// renderFailure names WHAT the render refused. A rejected delivery and a failed
// substage both arrive as the same sentence ("clip attempt validation failed"),
// which on a CI runner is all the log holds — and the check, the phase and the
// numbers it was measured against are the whole question (CLIP-88, CDS-52).
//
// The check alone is not always the answer: a substage that fails on an untyped
// error reports its step with no values at all (`check=render_encode values=map[]`),
// because CLIP-88 makes the diagnostic's own sentence the privacy-safe one the
// server may show and the cause underneath it never prints. Inside the image
// that privacy rule buys nothing, so the cause chain is spelled out here — and
// with it the media operation, failure class and elapsed time a commandFailure
// carries, which say whether ffmpeg ran at all.
func renderFailure(err error) error {
	detail := ""
	if d, ok := clip.DiagnosticFromError(err); ok {
		detail = fmt.Sprintf(": check=%s phase=%s element=%s values=%v", d.Check, d.Phase, d.ElementID, d.Values)
	}
	var c interface {
		MediaOperation() string
		MediaFailureClass() string
		MediaElapsedMS() int64
	}
	if errors.As(err, &c) {
		detail += fmt.Sprintf(" media=%s class=%s elapsed=%dms", c.MediaOperation(), c.MediaFailureClass(), c.MediaElapsedMS())
	}
	for e := errors.Unwrap(err); e != nil; e = errors.Unwrap(e) {
		detail += fmt.Sprintf(" <- %T(%v)", e, e)
	}
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w%s", err, detail)
}

// footagePlan hands a smoke's cuts to the one renderer there is, the
// composition, declaring the given body's text and nothing else: a smoke about
// the footage itself draws nothing over it.
func footagePlan(t *testing.T, plan clip.EditPlan, body string) clip.EditPlan {
	t.Helper()
	limits := clip.DefaultCompositionLimits()
	doc, problem := composition.ReadStored(body, limits)
	if problem != nil {
		t.Fatal(problem)
	}
	plan.Portable = &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: 1, Body: body}}
	for _, cut := range plan.Cuts {
		plan.Portable.Cuts = append(plan.Portable.Cuts, composition.Cut{ID: cut.ID, SourceID: cut.SourceID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS, PlaybackRatePermille: cut.Rate()})
	}
	resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: plan.Portable.Cuts}, limits, 30000)
	if problem != nil {
		t.Fatal(problem)
	}
	for _, element := range resolved.Elements {
		plan.Portable.Elements = append(plan.Portable.Elements, clip.PortableText{Resolved: element})
	}
	return plan
}

func path(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func TestRenderSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	t.Parallel()
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "glyphs", func(ws clip.MediaWorkspace) error {
		semibold, err := r.measure(t.Context(), ws, []string{"한글 여행 W"}, 600, 0, fontFamily)
		if err != nil {
			return err
		}
		bold, err := r.measure(t.Context(), ws, []string{"한글 여행 W"}, 800, 0, fontFamily)
		if err != nil {
			return err
		}
		if semibold["한글 여행 W"] == bold["한글 여행 W"] {
			t.Fatal("variable font weight was ignored")
		}
		canvas, _ := clip.ClipCanvas("vertical")
		copy := clip.Copy{Text: "한글 여행", Style: "bold", Anchor: "bottom", Align: "center"}
		layout, err := r.layoutCopy(t.Context(), ws, canvas, copy)
		if err != nil {
			return err
		}
		plate, err := copyPlate(t.Context(), r, ws, canvas, copy, layout, 0)
		if err != nil {
			return err
		}
		img, err := readPNG(plate)
		if err != nil {
			return err
		}
		_, _, _, alpha := img.At(0, 0).RGBA()
		if alpha != 0 {
			t.Fatal("copy plate background is not transparent")
		}
		// CDS-84: Paperlogy maps 갂 to a glyph with no outline, so 크게 강조 sets
		// it in Wanted Sans Variable. The real resvg has to honour that run: the
		// plate paints it, and the same plate without the substitute paints
		// nothing at all.
		gap := clip.Copy{Text: "갂갂갂", Style: "bold", Anchor: "bottom", Align: "center"}
		gapLayout, err := r.layoutCopy(t.Context(), ws, canvas, gap)
		if err != nil {
			return fmt.Errorf("substituted caption: %w", err)
		}
		if gapLayout.Caption.ID != "bold" || !gapLayout.Substitute['갂'] {
			t.Fatalf("갂 was not substituted inside 크게 강조: %s %v", gapLayout.Caption.ID, gapLayout.Substitute)
		}
		painted := func(index int, l copyLayout) int {
			t.Helper()
			plate, err := copyPlate(t.Context(), r, ws, canvas, gap, l, index)
			if err != nil {
				t.Fatal(err)
			}
			img, err := readPNG(plate)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
						count++
					}
				}
			}
			return count
		}
		if n := painted(90, gapLayout); n < 1000 {
			t.Fatalf("the substituted 갂 painted %d pixels", n)
		}
		gapLayout.Substitute = nil
		if n := painted(91, gapLayout); n != 0 {
			t.Fatalf("Paperlogy's own 갂 painted %d pixels, so the empty-glyph premise no longer holds", n)
		}
		// Every style, rasterized by the real resvg against the real font, is
		// checked in pixels: a plated style paints its plate token and its accent
		// where CDS puts it, an unplated one paints the stroke instead, and
		// 형광펜's highlight sits where the measured advance puts it (CDS-25).
		amber := design.Accent["amber"]
		for style, rule := range map[string]design.StyleRule{"bold": design.Caption()} {
			c := clip.Copy{Text: "가격 9900원", Keyword: "9900원", Style: style, Anchor: rule.Anchor, Align: rule.Align, Accent: "amber"}
			l, err := r.layoutCopy(t.Context(), ws, canvas, c)
			if err != nil {
				return fmt.Errorf("%s: %w", style, err)
			}
			path, err := copyPlate(t.Context(), r, ws, canvas, c, l, 1)
			if err != nil {
				return fmt.Errorf("%s: %w", style, err)
			}
			img, err := readPNG(path)
			if err != nil {
				return err
			}
			p := l.Region
			if rule.Plate != "" {
				// A point on the plate's top edge, inside its own padding and
				// clear of the corner radius: only the plate can have painted it.
				token := design.Color[rule.Plate]
				red, _, _, alpha := img.At(int(p.X+p.Width/2), int(p.Y+2)).RGBA()
				want := uint32(math.Round(token.Alpha * 0xffff))
				if alpha < want-0x300 || alpha > want+0x300 {
					return fmt.Errorf("%s plate alpha %d want %d", style, alpha, want)
				}
				// Premultiplied by that alpha: ink reads dark, paper reads light.
				if dark := red < alpha/2; dark != (rule.Plate == "ink_900") {
					return fmt.Errorf("%s plate colour %d over alpha %d", style, red, alpha)
				}
				// The accent bar runs the plate's full height at its left edge;
				// the dot sits inside the top-left padding instead.
				x, y := int(p.X+2), int(p.Y+p.Height/2)
				if rule.Dot {
					x, y = int(p.X+rule.Padding.H+design.Spacing.DotAccent/2), int(p.Y+rule.Padding.V+design.Spacing.DotAccent/2)
				}
				if !isAccent(img, x, y) {
					return fmt.Errorf("%s is missing its accent at %d,%d", style, x, y)
				}
				continue
			}
			// The stroke under the fill is the only thing an unplated style paints
			// nearly opaque and nearly black; the shadow is softer than α0.85.
			if !scan(img, p, func(r, g, b, a uint32) bool {
				return a > 0xd000 && r < 0x3000 && g < 0x3000 && b < 0x3000
			}) {
				return fmt.Errorf("%s painted no %s stroke", style, rule.Stroke)
			}
			if style == "simple" {
				if scan(img, p, func(r, g, b, a uint32) bool { return a > 0x8000 && r > 2*b }) {
					return fmt.Errorf("simple coloured a keyword")
				}
				continue
			}
			if !rule.Highlight {
				// 크게 강조 colours the word itself, so the accent is on a glyph.
				if !scan(img, p, func(r, g, b, a uint32) bool { return a > 0x8000 && r > 2*b }) {
					return fmt.Errorf("%s did not colour its accent word %s", style, amber)
				}
				continue
			}
			u := design.Spacing.UnderlineMark
			x := int(p.X + l.Keyword.Offset + l.Keyword.Width/2)
			y := int(p.Y + (1-u.RaiseEM-u.HeightEM/2)*l.FontSize)
			if !isAccent(img, x, y) {
				return fmt.Errorf("%s highlight is missing at %d,%d", style, x, y)
			}
			// It is BEHIND the keyword, not before or after it.
			if isAccent(img, int(p.X+2), y) {
				return fmt.Errorf("%s highlighted the whole line", style)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// isAccent reads amber #FFB020 through whatever is drawn over it: red leads and
// blue trails, which no other colour in these styles does.
func isAccent(img image.Image, x, y int) bool {
	r, g, b, a := img.At(x, y).RGBA()
	return a > 0x8000 && r > 2*b && g > b
}
func scan(img image.Image, region clip.Region, match func(r, g, b, a uint32) bool) bool {
	for y := int(region.Y); y < int(region.Y+region.Height); y++ {
		for x := int(region.X); x < int(region.X+region.Width); x++ {
			if match(img.At(x, y).RGBA()) {
				return true
			}
		}
	}
	return false
}
func readPNG(path string) (image.Image, error) { return readFrame(path) }

// Only an explicit local test export retains synthetic fixtures for owner picker
// verification. Ordinary build smoke leaves no file outside its test workspace.
func exportRenderSmoke(name, path string) error {
	dir := os.Getenv("CLIP_SMOKE_EXPORT")
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || filepath.Dir(dir) == "/" {
		return fmt.Errorf("unsafe smoke export directory")
	}
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(dest, source)
	return errorsJoinClose(err, dest)
}
func errorsJoinClose(err error, file *os.File) error {
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func volume(v float64) *float64 { return &v }

// blueAt is the blue channel at one pixel, which is what the flat blue fixture
// makes a plate measurable by: ink at α0.88 keeps only an eighth of it.
func blueAt(img image.Image, x, y int) uint32 {
	_, _, b, _ := img.At(x, y).RGBA()
	return b
}
