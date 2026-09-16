package media

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Three synthetic grounds: an even bright one, an even dark one, and a busy one
// whose frames swing between them. CDS-44 reads L and σ off exactly these.
func fill(level uint8) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 1080, 1920))
	for i := range img.Pix {
		if i%4 == 3 {
			img.Pix[i] = 0xff
			continue
		}
		img.Pix[i] = level
	}
	return img
}

func TestRegionLuminanceAndSummary(t *testing.T) {
	region := clip.Region{X: 300, Y: 1200, Width: 400, Height: 110}
	white, _, _, _ := regionLuminance(fill(0xff), region)
	black, _, _, _ := regionLuminance(fill(0x00), region)
	if white != 1 || black != 0 {
		t.Fatalf("white %v black %v", white, black)
	}
	// Only the region is measured: a bright frame with a dark band under the
	// copy reads dark, which is the whole point of cropping to the plate.
	banded := image.NewRGBA(image.Rect(0, 0, 1080, 1920))
	for y := range 1920 {
		for x := range 1080 {
			level := uint8(0xff)
			if y >= 1200 && y < 1310 {
				level = 0x00
			}
			banded.Set(x, y, color.RGBA{level, level, level, 0xff})
		}
	}
	if value, _, _, _ := regionLuminance(banded, region); value != 0 {
		t.Fatalf("the band under the copy is what counts: %v", value)
	}
	// A region off the frame measures nothing rather than guessing.
	if value, _, _, _ := regionLuminance(fill(0xff), clip.Region{X: 2000, Y: 0, Width: 10, Height: 10}); value != 0 {
		t.Fatalf("off-frame %v", value)
	}

	// σ is the deviation ACROSS the three frames: even footage has none, footage
	// that swings has a lot (CDS-44).
	even := summarize([]float64{0.2, 0.2, 0.2}, [][3]float64{{0.2, 0.2, 0.2}, {0.2, 0.2, 0.2}, {0.2, 0.2, 0.2}})
	if math.Abs(even.Mean-0.2) > 1e-12 || even.Sigma > 1e-12 || !even.Sampled() {
		t.Fatalf("even %+v", even)
	}
	busy := summarize([]float64{0, 0.5, 1}, [][3]float64{{0, 0, 0}, {0.5, 0.5, 0.5}, {1, 1, 1}})
	if math.Abs(busy.Mean-0.5) > 1e-9 || math.Abs(busy.Sigma-math.Sqrt(0.5/3)) > 1e-9 {
		t.Fatalf("busy %+v", busy)
	}
	if (Luminance{}).Sampled() {
		t.Fatal("the zero value is no sample, not a black frame")
	}
	if summarize(nil, nil).Sampled() {
		t.Fatal("no frames, no ground")
	}
}

// The two thresholds, at their exact values (CDS-44).
func TestScrimAndAccentDecisionsAtTheThresholds(t *testing.T) {
	for _, tc := range []struct {
		mean, sigma  float64
		scrim, white bool
	}{
		{0.599, 0.249, false, false},
		{0.6, 0.249, true, true},   // bright: scrim, and the accent turns white
		{0.599, 0.25, true, false}, // busy but not bright: scrim, accent stays
		{0.9, 0.9, true, true},
		{0, 0, false, false},
	} {
		l := Luminance{Mean: tc.mean, Sigma: tc.sigma, Frames: []float64{tc.mean}}
		if l.Scrim() != tc.scrim || l.AccentWhite() != tc.white {
			t.Fatalf("L %v σ %v: scrim %v white %v", tc.mean, tc.sigma, l.Scrim(), l.AccentWhite())
		}
	}
}

// The scrim's opacity where the text actually sits, which is what V3 credits.
func TestScrimGeometryAndEffectiveBackground(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	l, _ := design.Layout("vertical")
	if _, ok := scrimFor(canvas, "nowhere"); ok {
		t.Fatal("an unknown anchor takes no scrim")
	}
	for anchor, edge := range map[string]string{"top": "top", "upper_mid": "top", "bottom": "bottom", "lower_mid": "bottom"} {
		s, ok := scrimFor(canvas, anchor)
		if !ok || s.Edge != edge {
			t.Fatalf("%s took %q", anchor, s.Edge)
		}
	}
	bottom := design.Scrim["bottom"]
	band := clip.Region(l.ScrimBottom)
	// The gradient runs 0 at the band's top edge to 0.55 at its bottom.
	if got := scrimAlpha(bottom, band, clip.Region{Y: band.Y, Height: 0}); got != bottom.From {
		t.Fatalf("top of the band %v", got)
	}
	if got := scrimAlpha(bottom, band, clip.Region{Y: band.Y + band.Height, Height: 0}); got != bottom.To {
		t.Fatalf("bottom of the band %v", got)
	}
	// A copy at UPPER_MID is outside the top band entirely, so the scrim does
	// nothing for it — which is why CDS-44's last clause exists.
	top := design.Scrim["top"]
	if got := scrimAlpha(top, clip.Region(l.ScrimTop), clip.Region{Y: 640, Height: 200}); got != 0 {
		t.Fatalf("a band that does not cover the copy credits nothing: %v", got)
	}

	// A bright ground under a BOTTOM copy: the scrim darkens it, and the stroke
	// CDS-26 gives 형광펜 is what the white text is actually read against, so the
	// pairing clears V3 on footage as bright as a white frame.
	bright := Luminance{Mean: 0.95, R: 1, G: 1, B: 1, Frames: []float64{0.95}}
	copyAt := func(y float64) clip.Region { return clip.Region{X: 300, Y: y, Width: 400, Height: 110} }
	mark, clean := design.Caption(), design.Caption()
	washed := bright.Background(canvas, mark, "bottom", copyAt(1270))
	white := design.Color["text_white"].Hex
	if ratio, _ := design.Contrast(white, washed); ratio < design.Luma.ContrastMin {
		t.Fatalf("white on the stroke over a washed ground is %.2f:1 (%s)", ratio, washed)
	}
	// Without the stroke the same ground is unreadable, which is what V3 is for:
	// a style that lost its outline cannot stand on bright footage.
	bare := mark
	bare.Stroke = ""
	naked := bright.Background(canvas, bare, "bottom", copyAt(1270))
	if ratio, _ := design.Contrast(white, naked); ratio >= design.Luma.ContrastMin {
		t.Fatalf("an unstroked style on a white frame is not legible: %.2f:1 (%s)", ratio, naked)
	}
	// A plated style is never sampled at all (CDS-16), so it never gets here;
	// asked anyway, it reads the ground it was given rather than inventing one.
	if got := bright.Background(canvas, clean, "bottom", copyAt(1270)); got == "" {
		t.Fatal("a ground is always a colour")
	}
	// No band covers UPPER_MID, so there the scrim credits nothing.
	if got := bright.Background(canvas, bare, "upper_mid", copyAt(640)); got != "#FFFFFF" {
		t.Fatalf("no band covers UPPER_MID, so the ground stands: %s", got)
	}
	// A dark ground asks for no scrim at all, and needs none.
	dark := Luminance{Mean: 0.05, R: 0.05, G: 0.05, B: 0.05, Frames: []float64{0.05}}
	if got := dark.Background(canvas, bare, "bottom", copyAt(1270)); got != dark.Hex() {
		t.Fatalf("an unwashed ground is the footage itself: %s", got)
	}
}

// The sampler itself: three frames from the cut's own source, taken through the
// same cover-crop chain the render uses, read in Go (CDS-44).
func TestSamplerTakesThreeFramesThroughTheRenderChain(t *testing.T) {
	frames := []image.Image{fill(0xff), fill(0x80), fill(0x00)}
	var seeks, filters []string
	a := newAdapter(t, &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		input := slices.Index(c.Args, "-i")
		before := " " + strings.Join(c.Args[:input], " ") + " "
		// The decoder may use the cores the encode cannot; filter threads are
		// untouched by that split and the PNG is still written single-threaded.
		after := " " + strings.Join(c.Args[input:], " ") + " "
		if !strings.Contains(before, " -threads 2 ") || !strings.Contains(before, " -filter_threads 1 ") || !strings.Contains(after, " -threads 1 ") {
			t.Fatal("sampler uses the wrong decoder/filter/encoder threads", c.Args)
		}
		for i, arg := range c.Args {
			switch arg {
			case "-ss":
				seeks = append(seeks, c.Args[i+1])
			case "-vf":
				filters = append(filters, c.Args[i+1])
			}
		}
		out := c.Args[len(c.Args)-1]
		f, err := os.Create(out)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return nil, png.Encode(f, frames[min(len(seeks)-1, len(frames)-1)])
	}})
	r := testRenderer(t, a)
	canvas, _ := clip.ClipCanvas("vertical")
	cut := clip.EditCut{StartMS: 2000, EndMS: 9000, Focal: clip.Point{X: 0.25, Y: 0.75}}
	if err := a.WithWorkspace(t.Context(), "sample", func(ws clip.MediaWorkspace) error {
		ground, err := r.sample(t.Context(), ws, canvas, clip.MediaSource{Path: sourceFile(t, ws)}, cut,
			[2]int{120, 6880}, clip.Region{X: 300, Y: 1200, Width: 400, Height: 110}, 0)
		if err != nil {
			return err
		}
		// The first, middle and last frame of the copy window, on the SOURCE's
		// own clock: the cut's start plus the window's own offset.
		if strings.Join(seeks, " ") != "2.120 5.500 8.879" {
			return fmt.Errorf("seeks %q", seeks)
		}
		// Each through the render's scale-and-crop, so the luminance measured is
		// the luminance the viewer sees.
		for _, f := range filters {
			if f != coverChain(canvas, cut.Focal) {
				return fmt.Errorf("sampled raw source pixels: %q", f)
			}
		}
		// White, mid grey and black: the mean is their mean and σ their spread.
		if len(ground.Frames) != 3 || ground.Mean <= 0.2 || ground.Mean >= 0.5 || ground.Sigma < 0.3 {
			return fmt.Errorf("%+v", ground)
		}
		if !ground.Scrim() {
			return fmt.Errorf("footage swinging white to black is busy: %+v", ground)
		}
		// Nothing is left behind in the workspace.
		entries, err := os.ReadDir(ws.Path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "sample-") {
				return fmt.Errorf("left %s behind", e.Name())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Each unplated role samples the glyph union, including region slots with no
// stroke. A bright canvas outside that union must not trigger a scrim.
func TestDeclaredRolesSampleOnlyTheirTextBounds(t *testing.T) {
	body := `<clip version="1"><text id="caption" kind="fixed" role="caption" basis="whole">현재 장면</text><text id="intro" kind="fixed" role="hook" basis="output-start"><row>첫 장면</row><row>기록</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"><row>평점</row><row>4.5</row><row>또 올 곳</row></text><text id="info" kind="fixed" role="info" basis="whole"><row role="label">메뉴</row><row role="caption">된장찌개</row></text></clip>`
	layout := measuredDeclared(t, declaredPlan(t, body, "vertical"))
	canvas, _ := clip.ClipCanvas("vertical")
	for _, visual := range layout.visuals {
		t.Run(visual.manifest.Role, func(t *testing.T) {
			bounds := regionBounds(visual)
			banded := image.NewRGBA(image.Rect(0, 0, canvas.Width, canvas.Height))
			for y := 0; y < canvas.Height; y++ {
				for x := 0; x < canvas.Width; x++ {
					level := uint8(255)
					if x >= int(bounds.X) && x < int(bounds.X+bounds.Width) && y >= int(bounds.Y) && y < int(bounds.Y+bounds.Height) {
						level = 0
					}
					banded.SetRGBA(x, y, color.RGBA{level, level, level, 255})
				}
			}
			samples := 0
			a := newAdapter(t, &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
				if slices.Contains(c.Args, "-ss") {
					samples++
				}
				f, err := os.Create(c.Args[len(c.Args)-1])
				if err != nil {
					return nil, err
				}
				defer f.Close()
				return nil, png.Encode(f, banded)
			}})
			r := testRenderer(t, a)
			if err := a.WithWorkspace(t.Context(), "role-sample", func(ws clip.MediaWorkspace) error {
				_, err := r.declaredPlate(t.Context(), ws, canvas, &visual, clip.MediaSource{Path: sourceFile(t, ws), Info: clip.MediaInfo{DurationMS: 15000}}, 0)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if samples != 3 || visual.ground.Mean != 0 || visual.ground.Scrim() {
				t.Fatal(samples, visual.ground)
			}
		})
	}
}
