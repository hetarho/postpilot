package media

import (
	"math"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestHeaderSharesOneRowAndReservesLongDisclosure(t *testing.T) {
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		canvas, _ := clip.ClipCanvas(ratio)
		phrase := design.Disclosure["paid"]
		answers := map[string]string{"위치": strings.Repeat("넓은 공간 ", 20)}
		bounds := furnitureBounds(answers, []string{"위치"})
		bounds[furnitureKey("badge", phrase)] = clip.Region{X: 2, Y: -71, Width: 1100, Height: 75}
		bounds[furnitureKey("label", "위치")] = clip.Region{X: 4, Y: -72, Width: 180, Height: 75}
		bounds[furnitureKey("caption", strings.TrimSpace(answers["위치"]))] = clip.Region{X: 3, Y: -80, Width: 12000, Height: 92}
		f, err := placeFurniture(canvas, ratio, phrase, []string{"위치"}, answers, bounds)
		if err != nil || len(f.Chips) != 1 {
			t.Fatalf("%s: %+v %v", ratio, f, err)
		}
		c := f.Chips[0]
		if c.Region.Y != f.Badge.Y || c.Region.Height != f.Badge.Height || c.Region.X+c.Region.Width+design.Spacing.GapStack > f.Badge.X {
			t.Fatalf("%s header is misaligned or overlaps: %+v", ratio, f)
		}
		v := furnitureView(canvas, f)
		centre := f.Badge.Y + f.Badge.Height/2
		for _, actual := range []float64{v.Label.Y + f.BadgeBounds.Y + f.BadgeBounds.Height/2, v.Chips[0].Label.Y + c.LabelBounds.Y + c.LabelBounds.Height/2, v.Chips[0].Value.Y + c.ValueBounds.Y + c.ValueBounds.Height/2} {
			if math.Abs(actual-centre) > 0.001 {
				t.Fatalf("%s glyph centre %v, header centre %v", ratio, actual, centre)
			}
		}
	}
}

// Measured glyph boxes at 100 px, the shape r.measure returns: every label, its
// value and the disclosure phrase.
func furnitureBounds(answers map[string]string, labels []string) map[string]clip.Region {
	out := map[string]clip.Region{}
	for _, phrase := range design.Disclosure {
		out[furnitureKey("badge", phrase)] = clip.Region{X: 2, Y: -70, Width: 300, Height: 90}
	}
	for _, label := range labels {
		out[furnitureKey("label", label)] = clip.Region{X: 2, Y: -70, Width: 200, Height: 90}
		out[furnitureKey("caption", strings.TrimSpace(answers[label]))] = clip.Region{X: 2, Y: -70, Width: 420, Height: 90}
	}
	return out
}

// The badge's right edge and top are the ratio's own (CDS-31: 888/290 on 9:16,
// 1824/112 on 16:9 and 1016/112 on 1:1), and the chip stack starts at the ratio's
// chip origin, stacked on 9:16 and 1:1 and two across on 16:9 (CDS-30, CDS-47).
func TestBadgeGeometryAndChipStackPerRatio(t *testing.T) {
	answers := map[string]string{"위치": "서울 연남동", "가격": "9,900원", "메뉴": "김밥"}
	labels := []string{"위치", "가격", "메뉴"}
	bounds := furnitureBounds(answers, labels)
	for ratio, badge := range map[string][2]float64{"vertical": {888, 290}, "horizontal": {1824, 112}, "square": {1016, 112}} {
		canvas, _ := clip.ClipCanvas(ratio)
		l, _ := design.Layout(ratio)
		f, err := placeFurniture(canvas, ratio, design.Disclosure["ad"], labels, answers, bounds)
		if err != nil {
			t.Fatalf("%s: %v", ratio, err)
		}
		if f.Badge.X+f.Badge.Width != badge[0] || f.Badge.Y != badge[1] {
			t.Fatalf("%s badge at %+v want right %v top %v", ratio, f.Badge, badge[0], badge[1])
		}
		// CDS-30 shows at most two chips at once, in the priority it was given.
		if len(f.Chips) != 2 || f.Chips[0].Label != "위치" || f.Chips[1].Label != "가격" {
			t.Fatalf("%s chips %+v", ratio, f.Chips)
		}
		if f.Chips[0].Region.X != l.Chip.X || f.Chips[0].Region.Y != l.Chip.Y {
			t.Fatalf("%s chip origin %+v", ratio, f.Chips[0].Region)
		}
		across := f.Chips[1].Region.Y == f.Chips[0].Region.Y
		if across != (l.Chip.Columns > 1) {
			t.Fatalf("%s stacked=%v with %d columns", ratio, !across, l.Chip.Columns)
		}
		if across && f.Chips[1].Region.X <= f.Chips[0].Region.X {
			t.Fatalf("%s two-across did not advance x", ratio)
		}
		if !across && f.Chips[1].Region.Y != f.Chips[0].Region.Y+f.Chips[0].Region.Height+design.Spacing.GapStack {
			t.Fatalf("%s gap.stack lost: %+v", ratio, f.Chips)
		}
		for _, c := range f.Chips {
			if c.Region.Width > l.Chip.MaxWidth {
				t.Fatalf("%s chip wider than %v: %+v", ratio, l.Chip.MaxWidth, c.Region)
			}
			if c.Region.X < canvas.Safe.X || c.Region.X+c.Region.Width > canvas.Safe.X+canvas.Safe.Width {
				t.Fatalf("%s chip left the safe area: %+v", ratio, c.Region)
			}
		}
		// The badge and the chips are on the manifest, the badge for the whole
		// clip and each chip for its own cut.
		m := f.Elements(20000, 2, 5000, 9000)
		if m[0].Kind != "badge" || m[0].StartMS != 0 || m[0].EndMS != 20000 || m[0].Text != "광고" {
			t.Fatalf("%s badge element %+v", ratio, m[0])
		}
		if len(m) != 3 || m[1].Kind != "chip" || m[1].Cut != 2 || m[1].StartMS != 5000 || m[1].EndMS != 9000 {
			t.Fatalf("%s chip elements %+v", ratio, m[1:])
		}
		svg := furnitureSVG(canvas, f)
		if !strings.Contains(svg, "광고") || !strings.Contains(svg, "서울 연남동") || !strings.Contains(svg, "9,900원") {
			t.Fatalf("%s missing text: %s", ratio, svg)
		}
		// textLength is the hard bound that keeps the value inside its pill.
		if !strings.Contains(svg, "lengthAdjust=") {
			t.Fatal("chip value is not bounded to its pill")
		}
		if strings.Count(svg, "<rect") != 1+len(f.Chips) {
			t.Fatalf("%s plate count: %s", ratio, svg)
		}
	}
}

// A value too wide for its 600 px pill is cut and given an ellipsis, and the
// manifest records what was actually drawn (CDS-30).
func TestLongChipValueIsCutWithAnEllipsis(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	long := strings.TrimSpace(strings.Repeat("서울특별시 마포구 연남동 ", 4))
	answers := map[string]string{"위치": long}
	bounds := furnitureBounds(answers, []string{"위치"})
	bounds[furnitureKey("caption", long)] = clip.Region{X: 2, Y: -70, Width: 4000, Height: 90}
	f, err := placeFurniture(canvas, "vertical", design.Disclosure["ad"], []string{"위치"}, answers, bounds)
	if err != nil {
		t.Fatal(err)
	}
	l, _ := design.Layout("vertical")
	c := f.Chips[0]
	if c.Region.Width > l.Chip.MaxWidth {
		t.Fatalf("chip grew past %v: %+v", l.Chip.MaxWidth, c.Region)
	}
	if !strings.HasSuffix(c.Value, "…") || c.Value == "…" || len([]rune(c.Value)) >= len([]rune(long)) {
		t.Fatalf("value not cut: %q", c.Value)
	}
	// The manifest carries the drawn value, not the whole answer.
	m := f.Elements(20000, 0, 0, 4000)
	if !strings.Contains(m[1].Text, c.Value) || strings.Contains(m[1].Text, long) {
		t.Fatalf("manifest text = %q", m[1].Text)
	}
}

// 평점 shows only from the owner's own input, and a fact with no answer shows no
// chip at all (CDS-30).
func TestChipsOnlyFromAnswersThatExist(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	answers := map[string]string{"가격": "9,900원", "평점": "  "}
	f, err := placeFurniture(canvas, "vertical", design.Disclosure["self"], []string{"가격", "평점"}, answers, furnitureBounds(answers, []string{"가격", "평점"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Chips) != 1 || f.Chips[0].Label != "가격" {
		t.Fatalf("chips %+v", f.Chips)
	}
	rated := map[string]string{"평점": "4.5"}
	f, err = placeFurniture(canvas, "vertical", design.Disclosure["self"], []string{"평점"}, rated, furnitureBounds(rated, []string{"평점"}))
	if err != nil || len(f.Chips) != 1 || f.Chips[0].Value != "4.5" {
		t.Fatalf("the owner's own rating was dropped: %+v %v", f.Chips, err)
	}
}

// A plan chooses a cut's chips from the preset's priority, and only labels with
// an answer become a chip.
func TestPlanChipLabelsFollowThePresetPriority(t *testing.T) {
	plan := clip.EditPlan{Preset: "beauty", Facts: []clip.Answer{
		{Label: "메뉴", Text: "속눈썹"}, {Label: "가격", Text: "45,000원"}, {Label: "위치", Text: " "},
	}}
	// beauty's priority is 가격 → 메뉴 → 평점, so the order is the preset's, not
	// the cut's.
	long := clip.Cut{EndMS: 4000, Chips: []string{"메뉴", "가격", "위치"}}
	got := plan.ChipLabels(long)
	if strings.Join(got, ",") != "가격,메뉴" {
		t.Fatalf("labels = %v", got)
	}
	// A label the cut does not name is not shown even with an answer.
	if got := plan.ChipLabels(clip.Cut{EndMS: 4000, Chips: []string{"메뉴"}}); strings.Join(got, ",") != "메뉴" {
		t.Fatalf("labels = %v", got)
	}
	if got := plan.ChipLabels(clip.Cut{EndMS: 4000}); len(got) != 0 {
		t.Fatalf("labels = %v", got)
	}
	// A cut too short to hold a chip for 2.0 s carries none (CDS-30).
	short := long
	short.EndMS = int(design.Timing.ChipMinS*1000) - 1
	if got := plan.ChipLabels(short); len(got) != 0 {
		t.Fatalf("a %d ms cut carried %v", short.EndMS, got)
	}
	short.EndMS = int(design.Timing.ChipMinS * 1000)
	if got := plan.ChipLabels(short); len(got) != 2 {
		t.Fatalf("a %d ms cut carried %v", short.EndMS, got)
	}
}
