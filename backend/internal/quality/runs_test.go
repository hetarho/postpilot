package quality

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func textDoc(paragraphs ...string) Document {
	doc := Document{Title: "을지로 감자탕 노포 후기"}
	for _, paragraph := range paragraphs {
		doc.Blocks = append(doc.Blocks, Block{Type: BlockText, Content: paragraph})
	}
	return doc
}

func runeCount(tokens ...string) int {
	n := 0
	for _, token := range tokens {
		n += len([]rune(token))
	}
	return n
}

func TestRunShareCountsRunesInsideMatchedWindows(t *testing.T) {
	// Eight shared 어절, the comma on the post's side trimmed away before it could block them.
	post := textDoc(
		"을지로 골목 끝 노포에서, 뼈가 푸짐한 감자탕을 먹고 볶음밥까지 비웠다.",
		"다음에는 해장국을 먹어 볼 생각이다.",
	)
	other := textDoc("지난주에도 을지로 골목 끝 노포에서 뼈가 푸짐한 감자탕을 먹고 왔다.")
	overlap, ok := MatchRuns(post, []Document{other}, 8)
	if !ok {
		t.Fatal("the reading is absent")
	}
	shared := []string{"을지로", "골목", "끝", "노포에서", "뼈가", "푸짐한", "감자탕을", "먹고"}
	all := append(append([]string(nil), shared...), "볶음밥까지", "비웠다", "다음에는", "해장국을", "먹어", "볼", "생각이다")
	if want := float64(runeCount(shared...)) / float64(runeCount(all...)); math.Abs(overlap.Share-want) > 1e-12 {
		t.Fatalf("share = %v, want %v (runes of the covered 어절 over all of them)", overlap.Share, want)
	}
	want := []Run{{Tokens: shared, Start: 0, In: []int{0}}}
	if !reflect.DeepEqual(overlap.Runs, want) {
		t.Fatalf("runs = %+v, want %+v", overlap.Runs, want)
	}

	// Seven shared 어절 are not a run: nothing is covered, and that zero is real.
	short, ok := MatchRuns(post, []Document{textDoc("을지로 골목 끝 노포에서 뼈가 푸짐한 감자탕을 먹었다")}, 8)
	if !ok || short.Share != 0 || len(short.Runs) != 0 {
		t.Fatalf("a seven-어절 overlap = %+v, %v", short, ok)
	}
}

func TestAWindowNeverCrossesATextUnitEdge(t *testing.T) {
	paragraph := "성수동 골목 카페에서 고소한 라떼를 천천히 마시며 쉬었다"
	split := textDoc("성수동 골목 카페에서 고소한", "라떼를 천천히 마시며 쉬었다")
	whole := textDoc(paragraph)

	for name, pair := range map[string][2]Document{
		"two blocks against one paragraph": {split, whole},
		"one paragraph against two blocks": {whole, split},
		"two LIST items against one paragraph": {
			{Blocks: []Block{{Type: BlockList, Items: []string{"성수동 골목 카페에서 고소한", "라떼를 천천히 마시며 쉬었다"}}}},
			whole,
		},
	} {
		overlap, ok := MatchRuns(pair[0], []Document{pair[1]}, 8)
		if !ok || overlap.Share != 0 || len(overlap.Runs) != 0 {
			t.Errorf("%s matched across a unit edge: %+v, %v", name, overlap, ok)
		}
	}

	// A window may start on the document's first token and end on its unit's last one.
	overlap, ok := MatchRuns(whole, []Document{textDoc("오늘은 비가 왔다", paragraph)}, 8)
	if !ok || overlap.Share != 1 || len(overlap.Runs) != 1 || overlap.Runs[0].Start != 0 || len(overlap.Runs[0].Tokens) != 8 {
		t.Fatalf("an edge-to-edge window = %+v, %v", overlap, ok)
	}
	// A LIST item is a unit of its own, and a whole one matches.
	list := Document{Blocks: []Block{{Type: BlockList, Items: []string{"메뉴", paragraph}}}}
	if overlap, ok := MatchRuns(list, []Document{whole}, 8); !ok || len(overlap.Runs) != 1 || overlap.Runs[0].Start != 1 {
		t.Fatalf("a LIST item run = %+v, %v", overlap, ok)
	}
}

func TestOverlappingWindowsMergeIntoOneMaximalRun(t *testing.T) {
	shared := "제주 협재 해변은 에메랄드빛 물이 얕고 넓게 펼쳐져서 아이와 걷기에 좋았다"
	post := textDoc("둘째 날 아침에 " + shared + " 점심은 근처에서 먹었다")
	other := textDoc("작년에도 " + shared + " 그때도 날씨가 맑았다")
	overlap, ok := MatchRuns(post, []Document{other}, 8)
	if !ok {
		t.Fatal("the reading is absent")
	}
	// Eleven shared 어절 hold four overlapping windows, and they are one run, not four.
	want := []Run{{Tokens: Tokens(shared), Start: 3, In: []int{0}}}
	if !reflect.DeepEqual(overlap.Runs, want) {
		t.Fatalf("runs = %+v, want %+v", overlap.Runs, want)
	}
	if len(want[0].Tokens) != 11 {
		t.Fatalf("fixture error: %d shared 어절", len(want[0].Tokens))
	}
}

func TestARunNamesEveryOtherItStandsIn(t *testing.T) {
	long := "을지로 골목 끝 노포에서 뼈가 푸짐한 감자탕을 먹고 볶음밥까지 싹 비우고 나왔다"
	tokens := Tokens(long)
	eight := strings.Join(tokens[:8], " ")
	post := textDoc(long)
	others := []Document{
		textDoc("지난달 " + eight + " 끝"),    // the first eight 어절
		textDoc("오늘은 성수동 카페에서 라떼를 마셨다"),   // nothing
		textDoc("또 " + long),              // all twelve
		textDoc("친구와 " + eight + " 돌아왔다"), // the first eight again
	}
	overlap, ok := MatchRuns(post, others, 8)
	if !ok {
		t.Fatal("the reading is absent")
	}
	// Both runs start at 0; the longer comes first. The eight-어절 run stands whole in the
	// twelve-어절 document too, so it names all three documents that hold it.
	want := []Run{
		{Tokens: tokens, Start: 0, In: []int{2}},
		{Tokens: tokens[:8], Start: 0, In: []int{0, 2, 3}},
	}
	if !reflect.DeepEqual(overlap.Runs, want) {
		t.Fatalf("runs =\n%+v\nwant\n%+v", overlap.Runs, want)
	}
	if overlap.Share != 1 {
		t.Fatalf("share = %v, want every rune covered", overlap.Share)
	}
}

func TestRunsAreAbsentWithoutOthersOrEnoughTokens(t *testing.T) {
	eight := textDoc("성수동 골목 카페에서 고소한 라떼를 천천히 마시며 쉬었다")
	for name, test := range map[string]struct {
		post   Document
		others []Document
	}{
		"no other document":          {eight, nil},
		"an empty other list":        {eight, []Document{}},
		"seven post tokens":          {textDoc("성수동 골목 카페에서 고소한 라떼를 천천히 마셨다"), []Document{eight}},
		"a post with no text at all": {Document{Blocks: []Block{{Type: BlockImage, File: "IMG_1.jpg"}}}, []Document{eight}},
	} {
		if overlap, ok := MatchRuns(test.post, test.others, 8); ok {
			t.Errorf("%s: a reading is present: %+v", name, overlap)
		}
	}
	// Enough tokens and nothing shared is a real zero, not an absent value.
	overlap, ok := MatchRuns(eight, []Document{textDoc("제주 협재 해변은 물빛이 맑았다")}, 8)
	if !ok || overlap.Share != 0 || overlap.Runs == nil || len(overlap.Runs) != 0 {
		t.Fatalf("an unshared post = %+v, %v", overlap, ok)
	}
}
