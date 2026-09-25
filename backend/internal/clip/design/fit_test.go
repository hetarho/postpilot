package design

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func spec(role string) SlotSpec {
	r := Type[role]
	floors := map[string]float64{"display": 80, "headline": 60, "hook": 56, "title": 52, "body": 48, "caption": 40, "label": 34}
	return SlotSpec{Role: role, Size: r.Size, Floor: floors[role], Face: r.Face, Weight: r.Weight, Tracking: r.Tracking}
}

// The frontend preview and answer counter fit with the same table (CDS-86).
func TestFrontendMetricsMirrorIsByteIdentical(t *testing.T) {
	mirror, err := os.ReadFile("../../../../frontend/src/entities/clip-design/config/clip-metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mirror, MetricsJSON()) {
		t.Fatal("frontend clip-metrics.json drifted from backend metrics.json")
	}
}

func TestMetricsCarryEveryRegionFace(t *testing.T) {
	for _, key := range []string{"paperlogy:800", "wantedsans:600", "wantedsans:700", "nanummyeongjo:800", "jua:400"} {
		m, ok := metrics.Faces[key]
		if !ok {
			t.Fatal("missing face", key)
		}
		if m.Ink.Top <= 0.7 || m.Ink.Top >= 1 || m.Ink.Bottom < 0 || m.Ink.Bottom > 0.2 {
			t.Fatalf("%s: implausible Hangul ink %+v", key, m.Ink)
		}
		if m.Glyphs[" "] <= 0 || m.Glyphs["A"] <= 0 || m.Fallback < m.Glyphs["A"] {
			t.Fatalf("%s: incomplete Latin advances", key)
		}
	}
	// Wanted Sans and NanumMyeongjo set every syllable at one width; Paperlogy
	// and Jua draw only some, so they list each one they draw.
	for _, key := range []string{"wantedsans:600", "wantedsans:700", "nanummyeongjo:800"} {
		if metrics.Faces[key].Hangul <= 0 {
			t.Fatal(key, "lost its uniform Hangul advance")
		}
	}
	for _, key := range []string{"paperlogy:800", "jua:400"} {
		if metrics.Faces[key].Hangul != 0 || metrics.Faces[key].Glyphs["한"] <= 0 {
			t.Fatal(key, "must list the syllables it draws")
		}
	}
}

func TestTextWidthScalesAndTracks(t *testing.T) {
	w100 := TextWidth("wantedsans", 700, 0, 100, "한우")
	if w := TextWidth("wantedsans", 700, 0, 50, "한우"); w != w100/2 {
		t.Fatalf("width does not scale with size: %v vs %v", w, w100)
	}
	// Tracking sits between characters only.
	if w := TextWidth("wantedsans", 700, 0.1, 100, "한우"); w != w100+10 {
		t.Fatalf("tracking between two characters: %v", w)
	}
	if TextWidth("wantedsans", 700, 0, 100, "") != 0 {
		t.Fatal("an empty line has width")
	}
}

func TestCoversFollowsWhatTheFaceDraws(t *testing.T) {
	if !Covers("paperlogy", 800, "해미 한우 4.8") {
		t.Fatal("Paperlogy draws common syllables")
	}
	// Paperlogy maps every syllable but draws nothing for most rare ones.
	if Covers("paperlogy", 800, "갂") {
		t.Fatal("a syllable Paperlogy draws empty must not count as covered")
	}
	if Covers("jua", 400, "갂") || !Covers("jua", 400, "해미 한우") {
		t.Fatal("Jua covers exactly the syllables it lists")
	}
	if !Covers("wantedsans", 600, "갂") {
		t.Fatal("Wanted Sans covers every syllable")
	}
}

func TestFitRegionSlotKeepsShrinksThenWraps(t *testing.T) {
	headline := spec("headline")
	if f := FitRegionSlot(headline, "해미 한우", 856); f.Size != 96 || len(f.Lines) != 1 || f.Over {
		t.Fatalf("short text keeps its size: %+v", f)
	}
	f := FitRegionSlot(headline, "연남동 숯불 한우 오마카세", 856)
	if len(f.Lines) != 1 || f.Size < 82 || f.Size > 88 || f.Over {
		t.Fatalf("11 syllables shrink on one line: %+v", f)
	}
	if w := TextWidth(headline.Face, headline.Weight, headline.Tracking, f.Size, f.Lines[0]); w > 856 {
		t.Fatalf("a fitted line is %v wide", w)
	}
	f = FitRegionSlot(headline, "연남동 골목에서 30년째 숯불 한우만 굽는 집", 856)
	if len(f.Lines) != 2 || f.Size < 86 || f.Size > 96 || f.Over {
		t.Fatalf("19 syllables wrap into two balanced lines: %+v", f)
	}
	if strings.Join(f.Lines, " ") != "연남동 골목에서 30년째 숯불 한우만 굽는 집" {
		t.Fatal("wrapping lost words", f.Lines)
	}
	display := spec("display")
	if f := FitRegionSlot(display, "연남동 골목 30년 해미 한우", 856); len(f.Lines) != 2 || f.Size < 80 || f.Over {
		t.Fatalf("display wraps below its floor: %+v", f)
	}
}

func TestFitRegionSlotOverflows(t *testing.T) {
	label := spec("label")
	long := "서울 마포구 연남동 동교로 골목 안쪽 이층 투뿔 한우 숯불 구이 전문점"
	f := FitRegionSlot(label, long, 856)
	if !f.Over || len(f.Lines) != 1 || f.Size != 34 {
		t.Fatalf("a label never wraps and overflows past its 34 px floor: %+v", f)
	}
	if f := FitRegionSlot(label, "서울 연남동 · 숯불 한우 구이", 856); f.Over || f.Size != 36 {
		t.Fatalf("a short label keeps 36: %+v", f)
	}
	one := spec("display")
	one.Lines = 1
	if f := FitRegionSlot(one, "연남동 골목 30년 해미 한우", 856); len(f.Lines) != 1 || !f.Over {
		t.Fatalf("a one-line slot never wraps: %+v", f)
	}
	jua := SlotSpec{Role: "headline", Size: 128, Floor: 78, Face: "jua", Weight: 400}
	if f := FitRegionSlot(jua, "갂 한우", 816); !f.Over {
		t.Fatalf("a syllable Jua lacks overflows: %+v", f)
	}
	if f := FitRegionSlot(spec("headline"), "   ", 856); len(f.Lines) != 0 || f.Over {
		t.Fatalf("an empty slot draws nothing: %+v", f)
	}
}

func TestRegionSlotBudgetCountsSyllablesAtTheFloor(t *testing.T) {
	// 856 px at a 60 px Paperlogy headline holds 16 syllables a line, two lines.
	if n := RegionSlotBudget(spec("headline"), 856); n < 30 || n > 34 {
		t.Fatalf("headline budget %d", n)
	}
	if n := RegionSlotBudget(spec("label"), 856); n < 26 || n > 30 {
		t.Fatalf("label budget %d", n)
	}
}
