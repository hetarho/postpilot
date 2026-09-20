package memory

import (
	"context"
	"strings"
	"testing"
)

// listed builds memories in the order the store's List returns them — most recently used
// first, then most recently created — because that order IS retrieval's tie-break.
func listed(entries ...Memory) []Memory { return entries }

func mem(id string, kind Kind, tags ...string) Memory {
	return Memory{ID: id, Text: "사실 " + id, Kind: kind, Tags: tags}
}

// MEM-6 in one table: the two halves of the enum, and nothing else deciding candidacy.
func TestSelectSplitsTheEnumIntoItsTwoHalves(t *testing.T) {
	memories := listed(
		mem("pref", KindPreference),
		mem("persona", KindPersona),
		mem("place-hit", KindPlace, "연남동"),
		mem("place-miss", KindPlace, "성수동"),
		mem("person-miss", KindPerson, "지민"),
		mem("history-hit", KindHistory, "이사"),
	)
	// The key is the post's own words: an agglutinated 연남동에서 and a plain 이사.
	selected := Select(memories, []string{"연남동에서 점심을 먹었다", "이사 후 첫 주말"}, 10)

	got := make([]string, 0, len(selected))
	for _, m := range selected {
		got = append(got, m.ID)
	}
	want := []string{"place-hit", "history-hit", "pref", "persona"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("selected %v, want %v — tagged hits first, then the always-candidates", got, want)
	}
}

// The score is the count of DISTINCT matching tags, the first tie-break is the incoming
// order (last use, then creation), and the cap is the last thing applied.
func TestSelectScoresByOverlapAndBreaksTiesByTheListOrder(t *testing.T) {
	memories := listed(
		mem("one-tag", KindPlace, "카페"),
		mem("two-tags", KindPlace, "카페", "연남동"),
		mem("recent-pref", KindPreference),
		mem("older-pref", KindPreference),
	)
	key := []string{"연남동 카페에서"}

	selected := Select(memories, key, 10)
	got := make([]string, 0, len(selected))
	for _, m := range selected {
		got = append(got, m.ID)
	}
	if want := "two-tags,one-tag,recent-pref,older-pref"; strings.Join(got, ",") != want {
		t.Fatalf("order = %v, want %s", got, want)
	}

	// The cap takes the top of that order and nothing else (MEM-7).
	capped := Select(memories, key, 2)
	if len(capped) != 2 || capped[0].ID != "two-tags" || capped[1].ID != "one-tag" {
		t.Fatalf("capped = %v", capped)
	}
	if Select(memories, key, 0) != nil {
		t.Fatal("a zero cap selected something")
	}
}

// The matching is substring-over-folded-text and nothing else (MEM-8): no stemming, no
// morphology, no similarity. The table states both what that buys and what it costs.
func TestTagMatchingIsFoldedSubstringOnly(t *testing.T) {
	for name, test := range map[string]struct {
		tag  string
		key  []string
		want bool
	}{
		"agglutinated Korean":   {tag: "연남동", key: []string{"연남동에서 밥을 먹었다"}, want: true},
		"case folded":           {tag: "Cafe", key: []string{"동네 cafe 방문"}, want: true},
		"punctuation collapsed": {tag: "맥북", key: []string{"맥북(M4)을 샀다"}, want: true},
		// The memo ends and a template answer begins: a tag may not be assembled across that
		// seam out of words neither part contains.
		"across a part seam": {tag: "연남동", key: []string{"연", "남동"}, want: false},
		"absent":             {tag: "제주", key: []string{"연남동에서 밥을 먹었다"}, want: false},
		// The accepted cost of substring matching, stated rather than hidden: a short tag
		// matches inside a longer word. It is why MemoryTagsMax is small and the user writes
		// the tags themselves.
		"over-matches a short tag": {tag: "물", key: []string{"물건을 샀다"}, want: true},
	} {
		selected := Select(listed(mem("m", KindPlace, test.tag)), test.key, 5)
		if got := len(selected) == 1; got != test.want {
			t.Errorf("%s: matched = %v, want %v", name, got, test.want)
		}
	}
}

// A tag that folds to nothing cannot match everything: the empty string is a substring of
// every key, so it is skipped rather than counted.
func TestATagOfPunctuationAloneMatchesNothing(t *testing.T) {
	selected := Select(listed(mem("m", KindPlace, "!!!")), []string{"아무 말"}, 5)
	if len(selected) != 0 {
		t.Fatalf("a punctuation-only tag matched: %v", selected)
	}
}

// TextsForPost is what the generation context sees: texts, in injection order, and never a
// row. The port is what keeps the kind, the tags and the id out of the prompt builder.
func TestTextsForPostAnswersTextsInInjectionOrder(t *testing.T) {
	store := &fakeStore{list: listed(
		Memory{ID: "a", Text: "매운 음식을 못 먹는다", Kind: KindPreference},
		Memory{ID: "b", Text: "연남동에 자주 간다", Kind: KindPlace, Tags: []string{"연남동"}},
		Memory{ID: "c", Text: "성수동 카페를 좋아한다", Kind: KindPlace, Tags: []string{"성수동"}},
	)}
	service := newTestService(store)

	texts, err := service.TextsForPost(context.Background(), "alice", []string{"연남동에서 점심"})
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 2 || texts[0] != "연남동에 자주 간다" || texts[1] != "매운 음식을 못 먹는다" {
		t.Fatalf("texts = %v", texts)
	}

	// The configured InjectMax is the service's, never the caller's.
	store.list = append(store.list, Memory{ID: "d", Text: "네 번째", Kind: KindPersona}, Memory{ID: "e", Text: "다섯 번째", Kind: KindPersona})
	texts, err = service.TextsForPost(context.Background(), "alice", []string{"연남동"})
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 3 {
		t.Fatalf("texts = %d, want the configured InjectMax of 3", len(texts))
	}
}

// An account with no memories answers none rather than an error: a prompt with no memories
// is a valid prompt, and MEM-1's promise rests on it.
func TestTextsForPostOnAnEmptyAccount(t *testing.T) {
	texts, err := newTestService(&fakeStore{}).TextsForPost(context.Background(), "alice", []string{"무엇이든"})
	if err != nil || len(texts) != 0 {
		t.Fatalf("texts = %v, err = %v", texts, err)
	}
}

// fold is the whole normalization, so it is worth stating exactly.
func TestFoldLowercasesAndCollapses(t *testing.T) {
	for input, want := range map[string]string{
		"  Hello,   World!  ": "hello world",
		"맥북(M4)":              "맥북 m4",
		"!!!":                 "",
		"연남동":                 "연남동",
	} {
		if got := fold(input); got != want {
			t.Errorf("fold(%q) = %q, want %q", input, got, want)
		}
	}
	// The key joins its parts with a separator, so nothing matches across the seam.
	if got := foldKey([]string{"연", "남동"}); got != "연 남동" {
		t.Fatalf("foldKey = %q", got)
	}
}
