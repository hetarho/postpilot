package design_test

import (
	"math"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// WCAG 2.1's own reference points, then the promises CDS-16 and CDS-25 make with
// them: they are the reason the tokens are the values they are, so they are
// asserted rather than assumed.
func TestContrastArithmeticAndTheTokenPromises(t *testing.T) {
	white, ok := design.Relative("#FFFFFF")
	black, okb := design.Relative("#000000")
	if !ok || !okb || white != 1 || black != 0 {
		t.Fatalf("relative luminance %v %v", white, black)
	}
	if _, ok := design.Relative("fff"); ok {
		t.Fatal("a short hex is not a colour")
	}
	if _, ok := design.Relative("#GGGGGG"); ok {
		t.Fatal("a non-hex is not a colour")
	}
	if ratio, _ := design.Contrast("#FFFFFF", "#000000"); ratio != 21 {
		t.Fatalf("black on white is 21:1, got %v", ratio)
	}
	if ratio, _ := design.Contrast("#777777", "#777777"); ratio != 1 {
		t.Fatalf("a colour on itself is 1:1, got %v", ratio)
	}
	if _, ok := design.Contrast("#FFFFFF", ""); ok {
		t.Fatal("an unknown background has no ratio")
	}
	// Compositing is per channel in sRGB, which is what an SVG does.
	if got, _ := design.Over("#000000", 0.55, "#FFFFFF"); got != "#737373" {
		t.Fatalf("scrim over white %s", got)
	}
	if got, _ := design.Over("#123456", 1, "#FFFFFF"); got != "#123456" {
		t.Fatalf("opaque over anything is itself: %s", got)
	}
	if got, _ := design.Over("#000000", 0, "#FFFCF5"); got != "#FFFCF5" {
		t.Fatalf("nothing over a colour is that colour: %s", got)
	}

	// CDS-16: a plated element needs no sampling, because the plate over even a
	// WHITE frame keeps its text above the 4.5:1 floor.
	for style, rule := range design.Styles {
		if rule.Plate == "" {
			continue
		}
		plate := design.Color[rule.Plate]
		over, _ := design.Over(plate.Hex, plate.Alpha, "#FFFFFF")
		fill := design.Color["text_white"].Hex
		if rule.Plate == "paper_50" {
			fill = design.Color["text_ink"].Hex
		}
		ratio, _ := design.Contrast(fill, over)
		if ratio < design.Luma.ContrastMin {
			t.Fatalf("%s: %s on %s over white is %.2f:1", style, fill, rule.Plate, ratio)
		}
	}
	// CDS-25: one word may take the accent on a DARK ground, which is the only
	// ground that leaves the accent legible. The category chip is the same
	// pairing the other way round: ink on the accent (CDS-28).
	for name, hex := range design.Accent {
		if hex == "" {
			continue
		}
		ratio, ok := design.Contrast(hex, design.Color["ink_900"].Hex)
		if !ok || ratio < design.Luma.ContrastMin {
			t.Fatalf("accent %s on ink is %.2f:1", name, ratio)
		}
		// CDS-28 asks for #111 on the chip, not the slightly lighter text.ink:
		// #1A1A1A on violet is 4.40:1 and would fail V3.
		if ink, _ := design.Contrast(design.Color["ink_900"].Hex, hex); ink < design.Luma.ContrastMin {
			t.Fatalf("ink on accent %s is %.2f:1", name, ink)
		}
	}
	// And the reason CDS-44 turns it white instead: on a bright ground the same
	// accent is the one that fails.
	worst := 21.0
	for _, hex := range design.Accent {
		if hex == "" {
			continue
		}
		if ratio, _ := design.Contrast(hex, "#FFFFFF"); ratio < worst {
			worst = ratio
		}
	}
	if worst >= design.Luma.ContrastMin {
		t.Fatalf("an accent on white would not need replacing: %.2f:1", worst)
	}
	if math.Abs(design.Luma.ContrastMin-4.5) > 1e-9 {
		t.Fatal("V3's floor is WCAG 2.1's 4.5:1")
	}
}
