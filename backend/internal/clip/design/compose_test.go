package design_test

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// CDS-39 in its own priority order, and the edges the boundaries create.
func TestClassifyFollowsCDS39Priority(t *testing.T) {
	for text, want := range map[string]string{
		// NUM wins first: an Arabic number with one of the seven units.
		"9,900원":     design.ClassNum,
		"웨이팅 30분":    design.ClassNum,
		"2인 기준":      design.ClassNum,
		"매장 평점 4.5":  design.ClassFact, // no unit, so not a NUM
		"할인 20%":     design.ClassNum,
		"오후 6시에 갔어요": design.ClassNum,
		"3개 남았어요":    design.ClassNum,
		"가격이 2배":     design.ClassNum,
		// HOOK: a question or exclamation, or a marker, within 11 characters.
		"여기 왜 유명할까?": design.ClassHook,
		"진짜 맛있었다":    design.ClassHook,
		"솔직히 별로":     design.ClassHook,
		"이거 꼭 드세요":   design.ClassHook,
		// Over eleven characters a hook marker is no longer a hook.
		"진짜 솔직히 말하면 여기는 좀 아쉬웠어요": design.ClassDesc,
		// FACT: noun-led, at most fifteen characters, no verb ending.
		"서울 연남동":       design.ClassFact,
		"대표 메뉴 김치말이국수": design.ClassFact,
		// A verb ending makes it a description, not a fact.
		"조용하고 아늑했어요": design.ClassDesc,
		"분위기가 좋다":    design.ClassDesc,
		// Fourteen characters, no verb ending: still a fact.
		"열네 글자가 되는 노란 간판 가게": design.ClassFact,
		"": design.ClassDesc,
	} {
		if got := design.Classify(text); got != want {
			t.Fatalf("%q classified %s want %s", text, got, want)
		}
	}
	// The eleven-character hook boundary, counted the CDS way.
	// Past eleven characters a question is no longer a HOOK, and the priority
	// order then offers it to FACT before DESC — which is what CDS-39 says.
	eleven := strings.Repeat("가", 11) + "?"
	twelve := strings.Repeat("가", 12) + "?"
	if design.Classify(eleven) != design.ClassHook || design.Classify(twelve) != design.ClassFact {
		t.Fatalf("hook length boundary: %s %s", design.Classify(eleven), design.Classify(twelve))
	}
	// And the fifteen-character FACT boundary.
	if design.Classify(strings.Repeat("가", 15)) != design.ClassFact || design.Classify(strings.Repeat("가", 16)) != design.ClassDesc {
		t.Fatal("fact length boundary")
	}
}

// The whole CDS-40 table, scene by scene and class by class.
func TestSelectStyleMatchesTheCDS40Table(t *testing.T) {
	all := []string{"clean", "memo", "bold", "mark"}
	for scene, row := range map[string]map[string]string{
		"food":     {"NUM": "mark", "HOOK": "bold", "FACT": "memo", "DESC": "clean"},
		"interior": {"NUM": "mark", "HOOK": "bold", "FACT": "memo", "DESC": "clean"},
		"product":  {"NUM": "mark", "HOOK": "bold", "FACT": "memo", "DESC": "clean"},
		"exterior": {"NUM": "memo", "HOOK": "bold", "FACT": "memo", "DESC": "clean"},
		"menu":     {"NUM": "memo", "HOOK": "clean", "FACT": "memo", "DESC": "clean"},
		"person":   {"NUM": "clean", "HOOK": "bold", "FACT": "clean", "DESC": "clean"},
		"scenery":  {"NUM": "clean", "HOOK": "bold", "FACT": "memo", "DESC": "clean"},
	} {
		for class, want := range row {
			got := design.SelectStyle(scene, class, all, nil, 1)
			if got != want {
				t.Fatalf("%s/%s = %s want %s", scene, class, got, want)
			}
		}
	}
	// An analysis with no scene is read as the default row, never as no row.
	if design.SelectStyle("", "FACT", all, nil, 1) != design.SceneStyles["scenery"]["FACT"] {
		t.Fatal("a scene-less segment lost its row")
	}
	if design.Scene("bathroom") != "scenery" || design.Scene("menu") != "menu" {
		t.Fatal("scene fallback")
	}
}

func TestSelectStyleGuards(t *testing.T) {
	all := []string{"clean", "memo", "bold", "mark"}
	// 형광펜 needs exactly one number or keyword; otherwise it is 메모.
	for keywords, want := range map[int]string{0: "memo", 1: "mark", 2: "memo"} {
		if got := design.SelectStyle("food", "NUM", all, nil, keywords); got != want {
			t.Fatalf("%d keywords → %s want %s", keywords, got, want)
		}
	}
	// 크게 강조 at most twice per clip.
	if got := design.SelectStyle("food", "HOOK", all, design.StyleHistory{"bold", "clean", "bold"}, 1); got != "clean" {
		t.Fatalf("a third bold was allowed: %s", got)
	}
	if got := design.SelectStyle("food", "HOOK", all, design.StyleHistory{"bold", "clean"}, 1); got != "bold" {
		t.Fatalf("the second bold was refused: %s", got)
	}
	// The FIRST bold only on cut 1 or 2.
	if got := design.SelectStyle("food", "HOOK", all, nil, 1); got != "bold" {
		t.Fatal("cut 1")
	}
	if got := design.SelectStyle("food", "HOOK", all, design.StyleHistory{"clean"}, 1); got != "bold" {
		t.Fatal("cut 2")
	}
	if got := design.SelectStyle("food", "HOOK", all, design.StyleHistory{"clean", "clean"}, 1); got != "clean" {
		t.Fatalf("a first bold landed on cut 3: %s", got)
	}
	// A fourth consecutive same style alternates between the plated two.
	if got := design.SelectStyle("scenery", "DESC", all, design.StyleHistory{"clean", "clean", "clean"}, 1); got != "memo" {
		t.Fatalf("a fourth clean in a row: %s", got)
	}
	if got := design.SelectStyle("food", "FACT", all, design.StyleHistory{"memo", "memo", "memo"}, 1); got != "clean" {
		t.Fatalf("a fourth memo in a row: %s", got)
	}
	if got := design.SelectStyle("scenery", "DESC", all, design.StyleHistory{"clean", "clean"}, 1); got != "clean" {
		t.Fatal("the third in a row is still allowed")
	}
	// The template's approved set is the last word, with clean the fallback.
	// 깔끔하게 is the universal fallback, not the style's own alternative: the
	// template approved 메모 but the design system's fallback is clean.
	if got := design.SelectStyle("food", "NUM", []string{"clean", "memo"}, nil, 1); got != "clean" {
		t.Fatalf("mark was used outside the approved set: %s", got)
	}
	if got := design.SelectStyle("food", "NUM", []string{"memo", "mark"}, nil, 1); got != "mark" {
		t.Fatalf("an approved mark was refused: %s", got)
	}
	// A set without clean at all still gets an answer from what it does have.
	if got := design.SelectStyle("food", "HOOK", []string{"memo"}, nil, 1); got != "memo" {
		t.Fatalf("%s", got)
	}
	if got := design.SelectStyle("food", "HOOK", []string{"clean"}, nil, 1); got != "clean" {
		t.Fatalf("bold was used outside the approved set: %s", got)
	}
}

func candidate(anchor, align string, x, y, w, h float64) design.Candidate {
	return design.Candidate{Anchor: anchor, Align: align, Plate: design.Region{X: x, Y: y, Width: w, Height: h}, Fits: true}
}

// CDS-38, rule by rule.
func TestSelectAnchorFollowsCDS38(t *testing.T) {
	bottom := candidate("bottom", "center", 300, 1270, 400, 110)
	upper := candidate("upper_mid", "center", 300, 645, 400, 110)
	lower := candidate("lower_mid", "center", 300, 1045, 400, 110)
	top := candidate("top", "left", 96, 290, 400, 110)
	none := design.Region{}

	// Without a subject box the first (default) candidate wins.
	if got := design.SelectAnchor([]design.Candidate{bottom, lower}, none, nil, false, ""); got != 0 {
		t.Fatalf("default anchor = %d", got)
	}
	// A candidate that does not fit its safe area is dropped.
	unfit := bottom
	unfit.Fits = false
	if got := design.SelectAnchor([]design.Candidate{unfit, lower}, none, nil, false, ""); got != 1 {
		t.Fatalf("an unfit candidate was chosen: %d", got)
	}
	if got := design.SelectAnchor([]design.Candidate{unfit}, none, nil, false, ""); got != -1 {
		t.Fatalf("a copy with nowhere to go = %d", got)
	}
	// Copy yields to the badge and the chips, never the other way round.
	badge := []design.Element{{Kind: "badge", Region: design.Region{X: 280, Y: 1250, Width: 200, Height: 80}}}
	if got := design.SelectAnchor([]design.Candidate{bottom, lower}, none, badge, false, ""); got != 1 {
		t.Fatalf("copy sat on the badge: %d", got)
	}
	// The highest-ranked candidate covering ≤ 15 % of the subject wins.
	subject := design.Region{X: 280, Y: 1200, Width: 500, Height: 400}
	if got := design.SelectAnchor([]design.Candidate{bottom, lower}, subject, nil, false, ""); got != 1 {
		t.Fatalf("the covering candidate was kept: %d", got)
	}
	// When EVERY candidate covers more than 15 %, the least covering wins.
	tall := design.Region{X: 300, Y: 1000, Width: 200, Height: 400}
	narrower := candidate("lower_mid", "center", 350, 1045, 300, 110)
	if got := design.SelectAnchor([]design.Candidate{bottom, narrower}, tall, nil, false, ""); got != 1 {
		t.Fatalf("the least covering candidate was not chosen: %d", got)
	}
	// And the first candidate still wins when it is the one that covers least.
	if got := design.SelectAnchor([]design.Candidate{narrower, bottom}, tall, nil, false, ""); got != 0 {
		t.Fatalf("ranking lost: %d", got)
	}
	// Consecutive cuts move at most one anchor step: BOTTOM → TOP is three.
	if got := design.SelectAnchor([]design.Candidate{top, lower}, none, nil, false, "bottom"); got != 1 {
		t.Fatalf("a three-step jump was allowed: %d", got)
	}
	// The step is a preference, not a veto: with nowhere within one step the
	// copy is still placed rather than lost (see SelectAnchor's note).
	if got := design.SelectAnchor([]design.Candidate{top}, none, nil, false, "bottom"); got != 0 {
		t.Fatalf("a copy was dropped to obey the step rule: %d", got)
	}
	if got := design.SelectAnchor([]design.Candidate{upper}, none, nil, false, "lower_mid"); got != 0 {
		t.Fatal("one step was refused")
	}
	// Readable footage text allows only TOP or BOTTOM — and the step rule does
	// not apply there, because obeying both is impossible.
	if got := design.SelectAnchor([]design.Candidate{lower, top}, none, nil, true, "bottom"); got != 1 {
		t.Fatalf("readable text allowed a middle anchor: %d", got)
	}
	if got := design.SelectAnchor([]design.Candidate{lower, upper}, none, nil, true, ""); got != -1 {
		t.Fatal("readable text with no TOP or BOTTOM candidate")
	}
}

// CDS-42's voice: no emoji, no ㅋㅋ or ㄹㅇ, no superlatives.
func TestBannedVoice(t *testing.T) {
	for _, text := range []string{"진짜 맛있어요 🙂", "ㅋㅋ 웃겼어요", "ㄹㅇ 맛집", "역대급 맛", "최고의 하루"} {
		if !design.Banned(text) {
			t.Fatalf("%q was accepted", text)
		}
	}
	for _, text := range []string{"조용하고 아늑했어요", "9,900원", "서울 연남동 — 김밥", "O'Brien's 10:30"} {
		if design.Banned(text) {
			t.Fatalf("%q was refused", text)
		}
	}
}
