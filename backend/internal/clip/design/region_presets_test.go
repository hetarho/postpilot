package design_test

import (
	"encoding/json"
	"math"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

func near(a, b float64) bool { return math.Abs(a-b) < .001 }

func mustLayout(t *testing.T, kind, id, ratio string, rows ...string) design.RegionLayout {
	t.Helper()
	l, err := design.LayoutRegion(kind, id, ratio, rows)
	if err != nil || l.Over {
		t.Fatal(kind, id, ratio, err, l.Over)
	}
	return l
}

func shapesOf(l design.RegionLayout, kind string) []design.RegionShape {
	var out []design.RegionShape
	for _, s := range l.Shapes {
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out
}

// CDS-70: the adopted presets, and nothing else, are selectable.
func TestRegionPresetCatalogueIsCDS70(t *testing.T) {
	if got := design.RegionIDs("intro"); !slices.Equal(got, []string{"a", "b", "cover", "frame", "lower", "outline", "serif", "sticker"}) {
		t.Fatal(got)
	}
	if got := design.RegionIDs("outro"); !slices.Equal(got, []string{"b", "chips", "credits", "e", "list", "sidebar", "stamp"}) {
		t.Fatal(got)
	}
}

// A misspelt preset key is refused rather than drawn as a default.
func TestRegionItemsDecodeStrictly(t *testing.T) {
	var item design.RegionItem
	for _, bad := range []string{`{"slot":{"role":"label","colour":"text_white"}}`, `{"chips":{"slots":[],"gapp":14}}`, `{"slot":{},"gap":3}`, `{"bar":{}}`} {
		if err := json.Unmarshal([]byte(bad), &item); err == nil {
			t.Fatal("accepted", bad)
		}
	}
}

// CDS-89 and CDS-79: 매거진 커버 is set left at LEFT with its top edge at y 200,
// moved to the same fraction of height on 1:1, and its bar leads the block.
func TestCoverHangsFromItsTopEdge(t *testing.T) {
	for _, ratio := range []string{"vertical", "square"} {
		l := mustLayout(t, "intro", "cover", ratio, "성수 로컬 가이드", "성수동 곱창집", "서울 성동구")
		canvas, _ := design.Canvas(ratio)
		anchors, _ := design.Anchors(ratio)
		bar := l.Rules[0]
		if bar.Kind != "bar" || !near(bar.Box.Width, 64) || !near(bar.Box.Y, 200*float64(canvas.Height)/1920) || !near(bar.Box.X, anchors.Left) || l.Align != "left" || l.Scrim != "top" {
			t.Fatal(ratio, bar, l.Align, l.Scrim)
		}
		for _, s := range l.Slots {
			if !near(s.Lines[0].X, anchors.Left) || s.Lines[0].Align != "left" {
				t.Fatal("a cover line is not set at LEFT", s.Lines[0])
			}
		}
	}
}

// CDS-93 and CDS-96: the lower thirds stand on the BOTTOM anchor, 로어서드's
// label after a dot and the side bar over the whole block at LEFT.
func TestLowerThirdsStandOnTheBottomAnchor(t *testing.T) {
	lower := mustLayout(t, "intro", "lower", "vertical", "오늘의 기록", "연남동 한우집", "서울 마포구")
	anchors, _ := design.Anchors("vertical")
	if !near(lower.Bounds.Y+lower.Bounds.Height, anchors.Bottom) || lower.Scrim != "bottom" {
		t.Fatal(lower.Bounds)
	}
	dot := shapesOf(lower, "dot")
	if len(dot) != 1 || !dot[0].Circle || !near(dot[0].Radius, 8) || !near(dot[0].Box.X, anchors.Left) || !near(lower.Slots[0].Lines[0].X, anchors.Left+36) {
		t.Fatal(dot, lower.Slots[0].Lines[0])
	}
	if hair := lower.Rules[0]; hair.Kind != "hair" || !near(hair.Box.Width, 360) || !near(hair.Alpha, .8) {
		t.Fatal(hair)
	}
	side := mustLayout(t, "outro", "sidebar", "vertical", "다시 가고 싶은 집", "메뉴는 블로그에", "해미 한우")
	bar := shapesOf(side, "side_bar")
	if len(bar) != 1 || !near(bar[0].Box.X, anchors.Left) || !near(bar[0].Box.Width, 6) || !near(bar[0].Box.Y, side.Slots[0].Box.Y) || !near(bar[0].Box.Y+bar[0].Box.Height, anchors.Bottom) || !near(side.Width, 822) {
		t.Fatal(bar, side.Width)
	}
	for _, s := range side.Slots {
		if !near(s.Lines[0].X, anchors.Left+34) {
			t.Fatal("a side-bar line is not 34 px right of LEFT", s.Lines[0])
		}
	}
}

// CDS-90, CDS-91 and CDS-73: slot-bound decoration follows its text and leaves
// with it, while the block keeps its centre.
func TestSlotDecorationFollowsAndLeavesWithItsSlot(t *testing.T) {
	serif := mustLayout(t, "intro", "serif", "vertical", "이번 주의 식탁", "연남동 한우", "서울 마포구")
	flanks := shapesOf(serif, "flank")
	label := serif.Slots[0].Box
	if len(flanks) != 2 || !near(flanks[0].Box.Width, 56) || !near(flanks[0].Box.X+56+22, label.X) || !near(flanks[1].Box.X, label.X+label.Width+22) {
		t.Fatal(flanks, label)
	}
	bare := mustLayout(t, "intro", "serif", "vertical", "", "연남동 한우", "서울 마포구")
	if len(bare.Shapes) != 0 || len(bare.Slots) != 2 {
		t.Fatal("the flanks outlived their label", bare.Shapes)
	}
	short := mustLayout(t, "intro", "frame", "vertical", "한우", "서울")
	long := mustLayout(t, "intro", "frame", "vertical", "연남동 골목 숯불 한우", "서울")
	sf, lf := shapesOf(short, "frame")[0], shapesOf(long, "frame")[0]
	if !near(sf.Box.Width, 420) || !near(lf.Box.Width, long.Slots[0].Box.Width+112) || !near(lf.StrokeWidth, 3) || !near(lf.StrokeAlpha, .95) || lf.Fill != "" {
		t.Fatal(sf.Box, lf.Box, long.Slots[0].Box)
	}
	// The frame's padding is part of the stack: the label sits 34 px below the
	// frame, not below the headline's ink.
	if !near(long.Slots[1].Box.Y, lf.Box.Y+lf.Box.Height+34) {
		t.Fatal(long.Slots[1].Box, lf.Box)
	}
}

// CDS-92 and CDS-94: the outline numeral is one line; the sticker turns −4°
// about its centre and its label sits on a rounded plate.
func TestOutlineAndStickerKeepTheirDrawing(t *testing.T) {
	outline := mustLayout(t, "intro", "outline", "vertical", "숯불 한우", "연남동", "서울")
	if len(outline.Slots[0].Lines) != 1 || outline.Slots[0].Lines[0].Size >= 260 || outline.Slots[0].Spec.Outline != 4 {
		t.Fatal(outline.Slots[0])
	}
	sticker := mustLayout(t, "intro", "sticker", "vertical", "여기 진짜 맛집", "연남동 한우")
	plate := shapesOf(sticker, "plate")
	if sticker.Rotate != (design.RegionRotation{Deg: -4, CX: 540, CY: 960}) || len(plate) != 1 || !near(plate[0].Radius, 12) || !plate[0].Rotated || !sticker.Slots[0].Lines[0].Rotated {
		t.Fatal(sticker.Rotate, plate)
	}
	if !near(plate[0].Box.Width, sticker.Slots[1].Box.Width+56) || plate[0].Fill != design.Color["badge_ad"].Hex {
		t.Fatal(plate[0], sticker.Slots[1].Box)
	}
}

// CDS-97: chips flow into rows inside the measure, 14 px apart, and an empty
// chip slot draws no pill.
func TestChipsWrapInsideTheMeasure(t *testing.T) {
	few := mustLayout(t, "outro", "chips", "vertical", "추천해요", "데이트", "", "혼밥")
	if pills := shapesOf(few, "pill"); len(pills) != 2 || !near(pills[0].Box.Y, pills[1].Box.Y) || !near(pills[1].Box.X, pills[0].Box.X+pills[0].Box.Width+14) {
		t.Fatal(pills)
	}
	many := mustLayout(t, "outro", "chips", "vertical", "추천해요", "주차 가능한 넓은 매장", "단체 예약 가능한 룸", "혼밥하기 좋은 바 좌석", "반려견 동반 가능 테라스", "늦게까지 여는 곳")
	pills := shapesOf(many, "pill")
	rows := map[float64]float64{}
	for _, p := range pills {
		rows[p.Box.Y] += p.Box.Width
		if p.Box.X < 540-428 || p.Box.X+p.Box.Width > 540+428 || !near(p.Radius, p.Box.Height/2) {
			t.Fatal("a pill left the measure", p.Box)
		}
	}
	if len(pills) != 5 || len(rows) < 2 {
		t.Fatal("the chips did not wrap", len(pills), rows)
	}
}

// CDS-98: the list rows share the smallest size any of them needs, left-aligned
// as one group after their squares.
func TestListRowsShareOneSize(t *testing.T) {
	// One row just wide enough to shrink below 50 without reaching its floor.
	spec, width, _ := design.RegionSlotAt("outro", "list", "vertical", 1)
	long := "숯불 향이 진한 두툼한 한우 등심"
	for design.FitRegionSlot(spec, long, width).Size >= 50 {
		long += "과"
	}
	l := mustLayout(t, "outro", "list", "vertical", "오늘의 정리", "짧은 줄", long, "두 명이면 오만 원")
	var sizes []float64
	var xs []float64
	for _, s := range l.Slots[1:] {
		sizes = append(sizes, s.Lines[0].Size)
		xs = append(xs, s.Lines[0].X)
	}
	if len(sizes) != 3 || sizes[0] != sizes[1] || sizes[1] != sizes[2] || sizes[0] >= 50 || xs[0] != xs[1] || xs[1] != xs[2] {
		t.Fatal(sizes, xs)
	}
	for _, sq := range shapesOf(l, "square") {
		if !near(sq.Box.Width, 14) || !near(sq.Box.X+14+42, xs[0]) {
			t.Fatal(sq.Box)
		}
	}
}

// CDS-99: the stamp's rings and arcs turn −8° about its centre at y 900; the
// slot below it stays upright 64 px under the outer ring.
func TestStampLaysItsSlotsAroundTheRings(t *testing.T) {
	l := mustLayout(t, "outro", "stamp", "vertical", "해미 한우 · 연남동", "다시 올 집", "2026 오늘의 기록", "후기는 블로그에")
	if l.Rotate != (design.RegionRotation{Deg: -8, CX: 540, CY: 900}) {
		t.Fatal(l.Rotate)
	}
	rings := shapesOf(l, "ring")
	if len(rings) != 2 || !near(rings[0].Radius, 250) || !near(rings[1].Radius, 192) || !near(rings[0].FillAlpha, .22) || rings[1].Fill != "" || len(shapesOf(l, "ring_dot")) != 2 {
		t.Fatal(rings)
	}
	upper, lower := l.Slots[0].Lines[0].Arc, l.Slots[2].Lines[0].Arc
	if upper == nil || lower == nil || upper.Lower || !lower.Lower || !near(upper.R, 206) || !near(lower.R, 236) {
		t.Fatal(upper, lower)
	}
	below := l.Slots[3]
	if below.Lines[0].Rotated || !near(below.Box.Y, 900+250+64) || !l.Slots[1].Lines[0].Rotated {
		t.Fatal(below)
	}
	if spec, width, _ := design.RegionSlotAt("outro", "stamp", "vertical", 1); !near(width, 320) || spec.Floor != 56 {
		t.Fatal(spec, width)
	}
	if spec, width, _ := design.RegionSlotAt("outro", "stamp", "vertical", 0); !near(width, 560) || spec.MaxLines() != 1 {
		t.Fatal(spec, width)
	}
	empty := mustLayout(t, "outro", "stamp", "vertical", "", "", "", "")
	if len(empty.Shapes) != 0 || len(empty.Slots) != 0 {
		t.Fatal("an empty stamp drew its rings")
	}
}

// CDS-84 and CDS-77: a character the preset's face lacks overflows the slot.
func TestAMissingGlyphOverflowsItsSlot(t *testing.T) {
	missing := ""
	for r := rune(0xAC00); r <= 0xD7A3; r++ {
		if !design.Covers("jua", 400, string(r)) {
			missing = string(r)
			break
		}
	}
	if missing == "" {
		t.Fatal("Jua covers every syllable")
	}
	l, err := design.LayoutRegion("intro", "sticker", "vertical", []string{"맛집 " + missing, "연남동"})
	if err != nil || !l.Over || !l.Slots[0].Over || l.Slots[1].Over {
		t.Fatal(err, l.Over)
	}
}
