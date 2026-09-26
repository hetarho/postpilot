package design_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// The browser preview lays regions out with a TS port of LayoutRegion. This
// fixture is the Go layout of every preset on every ratio for short, shrinking
// and wrapping rows; the frontend copy must stay byte-identical and the port
// must reproduce it (CDS-86, CDS-87). Re-record with UPDATE_REGION_LAYOUTS=1.
const (
	layoutFixture       = "testdata/region-layouts.json"
	layoutFixtureMirror = "../../../../frontend/src/entities/clip-design/model/region-layouts.fixture.json"
	slotFixture         = "testdata/region-slots.json"
	slotFixtureMirror   = "../../../../frontend/src/entities/clip-design/model/region-slots.fixture.json"
)

// fixtureSlotAt is one preset slot's fit spec and width on a ratio, which the
// template editor's field budgets read through the port (CLIP-116).
type fixtureSlotAt struct {
	Kind   string  `json:"kind"`
	ID     string  `json:"id"`
	Ratio  string  `json:"ratio"`
	Index  int     `json:"index"`
	Role   string  `json:"role"`
	Size   float64 `json:"size"`
	Floor  float64 `json:"floor"`
	Lines  int     `json:"lines"`
	Width  float64 `json:"width"`
	Budget int     `json:"budget"`
}

func regionSlotFixture(t *testing.T) []byte {
	t.Helper()
	var out []fixtureSlotAt
	for _, kind := range []string{"intro", "outro"} {
		for _, id := range design.RegionIDs(kind) {
			preset, _ := design.Region(kind, id)
			for _, ratio := range []string{"vertical", "horizontal", "square"} {
				for i := range preset.Slots() {
					spec, width, ok := design.RegionSlotAt(kind, id, ratio, i)
					if !ok {
						t.Fatal(kind, id, i)
					}
					out = append(out, fixtureSlotAt{Kind: kind, ID: id, Ratio: ratio, Index: i, Role: spec.Role, Size: spec.Size, Floor: spec.Floor, Lines: spec.MaxLines(), Width: r3(width), Budget: design.RegionSlotBudget(spec, width)})
				}
			}
		}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type fixtureArc struct {
	CX    float64 `json:"cx"`
	CY    float64 `json:"cy"`
	R     float64 `json:"r"`
	Lower bool    `json:"lower"`
}
type fixtureLine struct {
	Text     string      `json:"text"`
	Size     float64     `json:"size"`
	Baseline float64     `json:"baseline"`
	Width    float64     `json:"width"`
	X        float64     `json:"x"`
	Align    string      `json:"align"`
	Arc      *fixtureArc `json:"arc,omitempty"`
	Rotated  bool        `json:"rotated,omitempty"`
}
type fixtureShape struct {
	Kind        string  `json:"kind"`
	Slot        int     `json:"slot"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	W           float64 `json:"w"`
	H           float64 `json:"h"`
	Radius      float64 `json:"radius"`
	Circle      bool    `json:"circle,omitempty"`
	Fill        string  `json:"fill,omitempty"`
	FillAlpha   float64 `json:"fillAlpha,omitempty"`
	Stroke      string  `json:"stroke,omitempty"`
	StrokeAlpha float64 `json:"strokeAlpha,omitempty"`
	StrokeWidth float64 `json:"strokeWidth,omitempty"`
	Shadow      bool    `json:"shadow,omitempty"`
	Rotated     bool    `json:"rotated,omitempty"`
}
type fixtureRotate struct {
	Deg float64 `json:"deg"`
	CX  float64 `json:"cx"`
	CY  float64 `json:"cy"`
}
type fixtureSlot struct {
	Index int           `json:"index"`
	Floor float64       `json:"floor"`
	Over  bool          `json:"over"`
	Lines []fixtureLine `json:"lines"`
}
type fixtureRule struct {
	Kind string  `json:"kind"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
}
type fixtureCase struct {
	Kind    string         `json:"kind"`
	ID      string         `json:"id"`
	Ratio   string         `json:"ratio"`
	Rows    []string       `json:"rows"`
	AnchorX float64        `json:"anchorX"`
	Scrim   string         `json:"scrim"`
	Slots   []fixtureSlot  `json:"slots"`
	Rules   []fixtureRule  `json:"rules"`
	Shapes  []fixtureShape `json:"shapes"`
	Rotate  *fixtureRotate `json:"rotate,omitempty"`
}

func r3(v float64) float64 { return math.Round(v*1000) / 1000 }

func regionLayoutFixture(t *testing.T) []byte {
	t.Helper()
	rowSets := map[string][][]string{
		"intro.a":       {{"해미 한우", "서울 연남동 · 숯불 한우 구이"}, {"연남동 숯불 한우 오마카세", "서울 마포구 연남동 · 투뿔 한우 숯불 구이"}, {"연남동 골목에서 30년째 숯불 한우만 굽는 집", ""}},
		"intro.b":       {{"해미 한우", "서울 연남동 · 숯불 한우 구이"}, {"연남동 골목 30년 숯불 한우 구이", "서울 마포구 연남동 · 투뿔 한우 숯불 구이"}, {"", "서울 연남동"}},
		"outro.b":       {{"다시 가고 싶은 불판", "자세한 후기는 블로그에"}, {"연남동에서 다시 가고 싶은 불판 1순위", "메뉴와 가격은 블로그 후기에 정리해 뒀어요"}, {"다시 가고 싶은 불판", ""}},
		"outro.e":       {{"직접 먹어본 평점", "4.8", "다시 가고 싶은 불판"}, {"세 번 가 보고 매긴 솔직한 평점", "해미 한우 연남 본점", "연남동에서 다시 가고 싶은 불판 1순위"}, {"", "연남동 골목 30년 해미 한우", "다시 가고 싶은 불판"}},
		"intro.cover":   {{"성수 로컬 가이드", "성수동 골목 곱창집", "서울 성동구 · 저녁 영업"}, {"성수 로컬 가이드 이번 주 추천", "성수동 골목에서 30년째 곱창만 굽는 노포 한 곳", "서울 성동구 성수이로 · 매일 저녁 영업"}, {"", "성수동 곱창집", ""}},
		"intro.serif":   {{"이번 주의 식탁", "연남동 숯불 한우", "서울 마포구 연남동"}, {"이번 주 식탁에 오른 한 끼", "연남동 골목 숯불 한우 오마카세 코스", "서울 마포구 연남동 · 예약제"}, {"", "연남동 숯불 한우", "서울 마포구"}},
		"intro.frame":   {{"해미 한우", "서울 연남동 · 숯불 구이"}, {"연남동 골목 숯불 한우 오마카세", "서울 마포구"}, {"한우", ""}},
		"intro.outline": {{"한우", "연남동 숯불 구이", "서울 마포구"}, {"숯불 한우", "연남동 골목 30년 숯불 한우 구이", "서울 마포구 연남동"}, {"", "연남동 숯불 구이", ""}},
		"intro.lower":   {{"오늘의 기록", "연남동 골목 한우집", "서울 마포구 연남동"}, {"오늘의 기록 · 세 번째 방문", "연남동 골목에서 30년째 숯불 한우만 굽는 집", "서울 마포구 연남동 · 매일 저녁"}, {"", "연남동 한우집", ""}},
		"intro.sticker": {{"여기 진짜 맛집", "연남동 숯불 한우"}, {"연남동 한우 여기 진짜 맛집이에요", "서울 마포구 연남동 골목 한우"}, {"", "연남동"}},
		"outro.credits": {{"오늘의 한 끼", "해미 한우", "서울 마포구 연남동", "매일 11시부터 22시까지"}, {"오늘 소개한 한 끼", "연남동 골목 숯불 한우 해미 본점", "서울 마포구 연남동 골목 안쪽", "평일 11시부터 22시까지 · 주말 예약"}, {"", "해미 한우", "", "매일 영업"}},
		"outro.sidebar": {{"다시 가고 싶은 집", "메뉴와 가격은 블로그에", "해미 한우 연남점"}, {"연남동에서 다시 가고 싶은 불판 1순위", "메뉴와 가격은 블로그 후기에 정리해 뒀어요", "해미 한우 연남 본점"}, {"다시 가고 싶은 집", "", ""}},
		"outro.chips":   {{"이런 분께 추천해요", "데이트", "회식", "혼밥", "주차 가능", "예약 필수"}, {"추천해요", "주차 가능한 넓은 매장", "단체 예약 가능한 룸", "혼밥하기 좋은 바 좌석", "반려견 동반 가능 테라스", "늦게까지 여는 곳"}, {"", "데이트", "", "혼밥"}},
		"outro.list":    {{"오늘의 정리", "숯불 향이 진한 한우", "두 명이면 5만 원대", "주말 저녁은 예약 필수"}, {"오늘 먹어 보고 정리한 것", "짧은 줄", "숯불 향이 오래 남는 두툼한 한우 등심과 된장", "두 명이면 오만 원"}, {"오늘의 정리", "", "두 명이면 5만 원대"}},
		"outro.stamp":   {{"해미 한우 · 연남동", "다시 올 집", "2026 오늘의 기록", "자세한 후기는 블로그에"}, {"해미 한우 연남 본점 · 서울 마포구", "연남동에서 다시 올 집", "2026년 가을 세 번째 방문", "메뉴와 가격은 블로그 후기에"}, {"", "다시 올 집", "", ""}},
	}
	var cases []fixtureCase
	for _, name := range []string{"intro.a", "intro.b", "intro.cover", "intro.serif", "intro.frame", "intro.outline", "intro.lower", "intro.sticker", "outro.b", "outro.e", "outro.credits", "outro.sidebar", "outro.chips", "outro.list", "outro.stamp"} {
		kind, id := name[:5], name[6:]
		for _, ratio := range []string{"vertical", "horizontal", "square"} {
			for _, rows := range rowSets[name] {
				l, err := design.LayoutRegion(kind, id, ratio, rows)
				if err != nil {
					t.Fatal(err)
				}
				c := fixtureCase{Kind: kind, ID: id, Ratio: ratio, Rows: rows, AnchorX: r3(l.AnchorX), Scrim: l.Scrim, Slots: []fixtureSlot{}, Rules: []fixtureRule{}, Shapes: []fixtureShape{}}
				for _, s := range l.Slots {
					fs := fixtureSlot{Index: s.Index, Floor: s.Type.Floor, Over: s.Over}
					for _, line := range s.Lines {
						fl := fixtureLine{Text: line.Text, Size: line.Size, Baseline: r3(line.Baseline), Width: r3(line.Width), X: r3(line.X), Align: line.Align, Rotated: line.Rotated}
						if a := line.Arc; a != nil {
							fl.Arc = &fixtureArc{CX: r3(a.CX), CY: r3(a.CY), R: r3(a.R), Lower: a.Lower}
						}
						fs.Lines = append(fs.Lines, fl)
					}
					c.Slots = append(c.Slots, fs)
				}
				for _, sh := range l.Shapes {
					c.Shapes = append(c.Shapes, fixtureShape{Kind: sh.Kind, Slot: sh.Slot, X: r3(sh.Box.X), Y: r3(sh.Box.Y), W: r3(sh.Box.Width), H: r3(sh.Box.Height), Radius: r3(sh.Radius), Circle: sh.Circle, Fill: sh.Fill, FillAlpha: sh.FillAlpha, Stroke: sh.Stroke, StrokeAlpha: sh.StrokeAlpha, StrokeWidth: sh.StrokeWidth, Shadow: sh.Shadow, Rotated: sh.Rotated})
				}
				if l.Rotate.Deg != 0 {
					c.Rotate = &fixtureRotate{Deg: l.Rotate.Deg, CX: r3(l.Rotate.CX), CY: r3(l.Rotate.CY)}
				}
				for _, rule := range l.Rules {
					c.Rules = append(c.Rules, fixtureRule{Kind: rule.Kind, X: r3(rule.Box.X), Y: r3(rule.Box.Y), W: rule.Box.Width, H: rule.Box.Height})
				}
				cases = append(cases, c)
			}
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cases); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// The two copies stay byte-identical, but numbers are matched within one r3
// step: arm64 fuses the layout's multiply-adds and amd64 does not, so a value
// sitting on a rounding boundary records one step apart on a Mac and on CI.
// The frontend port compares at 0.01, well above that step.
func TestRegionLayoutFixtureIsCurrent(t *testing.T) {
	for paths, want := range map[[2]string][]byte{{layoutFixture, layoutFixtureMirror}: regionLayoutFixture(t), {slotFixture, slotFixtureMirror}: regionSlotFixture(t)} {
		if os.Getenv("UPDATE_REGION_LAYOUTS") == "1" {
			for _, path := range paths {
				if err := os.WriteFile(path, want, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		var recorded []byte
		for _, path := range paths {
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if recorded != nil && !bytes.Equal(got, recorded) {
				t.Fatalf("%s differs from %s; re-record with UPDATE_REGION_LAYOUTS=1", path, paths[0])
			}
			recorded = got
		}
		var got, laid any
		if err := json.Unmarshal(recorded, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(want, &laid); err != nil {
			t.Fatal(err)
		}
		if !sameFixture(got, laid) {
			t.Fatalf("%s is stale; re-record with UPDATE_REGION_LAYOUTS=1", paths[0])
		}
	}
}

func sameFixture(got, want any) bool {
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		return ok && math.Abs(g-w) < 0.0015
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !sameFixture(g[i], w[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for k, v := range w {
			if gv, ok := g[k]; !ok || !sameFixture(gv, v) {
				return false
			}
		}
		return true
	default:
		return got == want
	}
}
