package media

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Every line measures the same fixed box, so the geometry below is the card's
// own arithmetic rather than the font's.
func cardRenderer(t *testing.T) (*Adapter, *Rendering) {
	t.Helper()
	a := newAdapter(t, &fakeRunner{run: func(context.Context, Command) ([]byte, error) {
		return []byte("m0,1,120,500,100\n"), nil
	}})
	return a, testRenderer(t, a)
}

func answers(extra ...[2]string) map[string]string {
	out := map[string]string{"상호": "해람 베이커리", "위치": "서울 성수동"}
	for _, e := range extra {
		out[e[0]] = e[1]
	}
	return out
}

// The two cards on the three ratios, with and without a price line (CDS-28,
// CDS-29, CDS-46 through CDS-48).
func TestCardGeometryAndGoldensPerRatio(t *testing.T) {
	a, r := cardRenderer(t)
	if err := a.WithWorkspace(t.Context(), "cards", func(ws clip.MediaWorkspace) error {
		for _, ratio := range []string{"vertical", "horizontal", "square"} {
			canvas, _ := clip.ClipCanvas(ratio)
			l, _ := design.Layout(ratio)
			for _, priced := range []bool{false, true} {
				name := "plain"
				facts := answers()
				if priced {
					name, facts = "priced", answers([2]string{"가격", "9900원"})
				}
				cards := map[string]cardLayout{
					"hook": hookCard(ratio, "갓 구운 빵", "cafe", facts["상호"], "coral", 19800),
					"end":  endingCard(ratio, "cafe", "", facts, "coral", 19800),
				}
				for kind, card := range cards {
					measured, err := r.measureCard(t.Context(), ws, canvas, ratio, card)
					if err != nil {
						return fmt.Errorf("%s/%s/%s: %w", ratio, kind, name, err)
					}
					golden(t, fmt.Sprintf("card-%s-%s-%s.svg", kind, ratio, name), cardSVG(canvas, measured)+"\n")
					box := l.HookCard
					if kind == "end" {
						box = l.EndCard
					}
					// The ratio's own width, centred on the frame and on its
					// card line, with CDS-28's 40 px padding.
					region := measured.Region
					if region.Width != box.Width || region.X != (float64(canvas.Width)-box.Width)/2 {
						return fmt.Errorf("%s/%s width %+v want %v", ratio, kind, region, box.Width)
					}
					if centre := region.Y + region.Height/2; centre != box.CenterY {
						return fmt.Errorf("%s/%s centred on %v want %v", ratio, kind, centre, box.CenterY)
					}
					// The plate may reach past the safe area (CDS-28 states
					// x 144–936 on 9:16); its text may not.
					for _, e := range measured.Elements(0) {
						if e.Kind == "card" {
							continue
						}
						if e.Region.X < canvas.Safe.X || e.Region.X+e.Region.Width > canvas.Safe.X+canvas.Safe.Width {
							return fmt.Errorf("%s/%s %s left the safe area: %+v", ratio, kind, e.Kind, e.Region)
						}
					}
					if err := clip.VerifyLayout(ratio, withBadge(measured.Elements(0), ratio, 19800)); err != nil {
						return fmt.Errorf("%s/%s/%s: %w", ratio, kind, name, err)
					}
				}
				// The hook card's window is the first 1.5 s and the ending
				// card's the last 2.5 s (CDS-28, CDS-29).
				if h := cards["hook"]; h.StartMS != 0 || h.EndMS != 1500 {
					return fmt.Errorf("hook window %d..%d", h.StartMS, h.EndMS)
				}
				if e := cards["end"]; e.StartMS != 17300 || e.EndMS != 19800 {
					return fmt.Errorf("ending window %d..%d", e.StartMS, e.EndMS)
				}
				// The price line appears only when the owner gave one, and the
				// preset's own note rides with it (CDS-29, CDS-51).
				texts := []string{}
				for _, line := range cards["end"].Lines {
					texts = append(texts, line.Text)
				}
				joined := strings.Join(texts, "|")
				if strings.Contains(joined, "9900원") != priced {
					return fmt.Errorf("%s priced=%v: %s", ratio, priced, joined)
				}
				if priced && !strings.Contains(joined, design.Presets["cafe"].PriceNote) {
					return fmt.Errorf("the preset's price note is missing: %s", joined)
				}
				// The CTA is the last line, in the accent, and the location is
				// the muted one (CDS-29).
				last := cards["end"].Lines[len(cards["end"].Lines)-1]
				if last.Fill != design.Accent["coral"] || last.Text != design.CTA[design.DefaultCTA("cafe", "")] {
					return fmt.Errorf("CTA line %+v", last)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A clip that cannot say whose place it was has no card to show (CDS-28,
// CDS-29), and neither does one whose hook the compiler dropped.
func TestACardWithoutItsOwnAnswersIsNotDrawn(t *testing.T) {
	a, r := cardRenderer(t)
	nameless := answers()
	delete(nameless, "상호")
	for name, card := range map[string]cardLayout{
		"no hook":      hookCard("vertical", "", "cafe", "해람 베이커리", "coral", 19800),
		"no name":      hookCard("vertical", "갓 구운 빵", "cafe", "", "coral", 19800),
		"no name, end": endingCard("vertical", "cafe", "", nameless, "coral", 19800),
		"bad ratio":    hookCard("2:3", "갓 구운 빵", "cafe", "해람 베이커리", "coral", 19800),
	} {
		if !card.empty() {
			t.Fatalf("%s drew a card anyway: %+v", name, card)
		}
		if card.Elements(0) != nil {
			t.Fatalf("%s put an empty card in the manifest", name)
		}
	}
	// An empty card rasterizes to nothing at all: no plate, no input, no layer.
	if err := a.WithWorkspace(t.Context(), "cards", func(ws clip.MediaWorkspace) error {
		canvas, _ := clip.ClipCanvas("vertical")
		empty := cardLayout{Kind: "hook"}
		path, err := r.cardPlate(t.Context(), ws, canvas, empty, 0)
		if err != nil || path != "" {
			return fmt.Errorf("%q %w", path, err)
		}
		if svg := cardSVG(canvas, empty); strings.Contains(svg, "<rect") {
			return fmt.Errorf("an empty card painted something: %s", svg)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Without a category chip there is no accent pill, and the hook still shows.
	plain := hookCard("vertical", "갓 구운 빵", "cafe", "해람 베이커리", "", 19800)
	for _, line := range plain.Lines {
		if line.Chip {
			t.Fatal("a card with no accent drew a chip")
		}
	}
	if len(plain.Lines) != 2 {
		t.Fatalf("the hook and the name still stand: %+v", plain.Lines)
	}
	// A clip shorter than the card's own window keeps the window inside it.
	short := hookCard("vertical", "갓 구운 빵", "cafe", "해람 베이커리", "coral", 1000)
	if short.EndMS != 1000 {
		t.Fatalf("hook window %d past the clip", short.EndMS)
	}
}

// A manifest of card elements needs the clip's badge to satisfy V6, which is
// about the disclosure and not about cards.
func withBadge(m clip.Manifest, ratio string, duration int) clip.Manifest {
	l, _ := design.Layout(ratio)
	badge := design.Element{
		Kind: "badge", Text: design.Disclosure["ad"], FontSize: design.Type["badge"].Size,
		Background: design.Color["badge_ad"].Hex,
		Region:     design.Region{X: l.Badge.Right - 120, Y: l.Badge.Top, Width: 120, Height: 60},
		StartMS:    0, EndMS: duration,
	}
	return append(clip.Manifest{badge}, m...)
}
