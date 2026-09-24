package quality

import (
	"reflect"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestTokensNormalizeSplitAndTrimEdgePunctuation(t *testing.T) {
	for name, test := range map[string]struct {
		text string
		want []string
	}{
		"edge punctuation and symbols": {
			text: "“을지로 노포에서 감자탕을!! (성수동) 들렀다가 맛있었어요~^^",
			want: []string{"을지로", "노포에서", "감자탕을", "성수동", "들렀다가", "맛있었어요"},
		},
		"inner punctuation stays": {
			text: "라떼 한 잔 6,500원, 역에서 3.5km.",
			want: []string{"라떼", "한", "잔", "6,500원", "역에서", "3.5km"},
		},
		"an 어절 that trims to nothing is dropped": {
			text: "협재 해변 — 물빛이 ^^ 맑았다 !!",
			want: []string{"협재", "해변", "물빛이", "맑았다"},
		},
		"any Unicode whitespace splits": {
			text: "제주\u00a0협재\t해변\n물빛",
			want: []string{"제주", "협재", "해변", "물빛"},
		},
		"an emoji or a hashtag at the edge never blocks a match": {
			text: "#성수동카페 라떼☕ 최고👍",
			want: []string{"성수동카페", "라떼", "최고"},
		},
	} {
		if got := Tokens(test.text); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: Tokens = %q, want %q", name, got, test.want)
		}
	}

	// Decomposed Hangul composes, so it compares equal to what the composed noun tokenizes to.
	decomposed := norm.NFD.String("감자탕을 먹었다")
	if decomposed == "감자탕을 먹었다" {
		t.Fatal("fixture error: the NFD form equals the NFC one")
	}
	if got := Tokens(decomposed); !reflect.DeepEqual(got, []string{"감자탕을", "먹었다"}) {
		t.Errorf("NFD tokens = %q", got)
	}
	if got := Tokens(" \n\t "); len(got) != 0 {
		t.Errorf("blank text tokenized to %q", got)
	}
}

func TestUnitsFollowBlockOrderAndSkipUnfilledSlots(t *testing.T) {
	doc := Document{
		Title: "을지로 감자탕 노포 후기",
		Blocks: []Block{
			{Type: BlockHeading, Content: "을지로 감자탕"},
			{Type: BlockText, Content: "뼈가 푸짐한 감자탕을 먹었다."},
			{Type: BlockImage, File: "IMG_1.jpg", Content: "사진 설명"},
			{Type: BlockText, Content: "{{slot:1}}"},
			{Type: BlockList, Items: []string{"감자탕 대 38,000원", "", "볶음밥 3,000원"}},
			{Type: BlockVideo, File: "clip.mp4"},
			{Type: BlockText, Content: "지도는 {{slot:2}}에서 확인하세요."},
			{Type: BlockQuote, Content: "국물이 진했다"},
			{Type: BlockText, Content: "   "},
		},
	}
	want := []Unit{
		{Type: BlockHeading, Text: "을지로 감자탕"},
		{Type: BlockText, Text: "뼈가 푸짐한 감자탕을 먹었다."},
		{Type: BlockList, Text: "감자탕 대 38,000원"},
		{Type: BlockList, Text: "볶음밥 3,000원"},
		{Type: BlockText, Text: "지도는  에서 확인하세요."},
		{Type: BlockQuote, Text: "국물이 진했다"},
	}
	if got := Units(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("Units =\n%+v\nwant\n%+v", got, want)
	}
	// The slot token never reaches a token, and removing it joins no two words.
	if got := Tokens(Units(doc)[4].Text); !reflect.DeepEqual(got, []string{"지도는", "에서", "확인하세요"}) {
		t.Errorf("tokens around a removed slot = %q", got)
	}
	if got := Units(Document{Title: "제목만 있는 글"}); len(got) != 0 {
		t.Errorf("a document with no blocks has units %+v", got)
	}
}

// Review F10: an emoji's invisible parts — variation selectors, ZWJ and other format runes,
// enclosing marks — trim at an 어절's edges like its visible symbol, and a keycap goes whole, so
// an emoji-tailed noun still matches and a lone emoji is no word.
func TestTokensTrimEmojiAtTheEdges(t *testing.T) {
	for input, want := range map[string][]string{
		"맛있어요❤\ufe0f latte☕\ufe0f":                   {"맛있어요", "latte"},
		"☕\ufe0f":                                    nil,
		"1\ufe0f\u20e3":                              nil,
		"추천1\ufe0f\u20e3":                            {"추천"},
		"#\ufe0f\u20e3맛집":                            {"맛집"},
		"!#\ufe0f\u20e3추천":                           {"추천"},
		"추천1\ufe0f\u20e3!":                           {"추천"},
		"\U0001f468\u200d\U0001f469\u200d\U0001f467": nil,
		"\U0001f44d\U0001f3fb":                       nil,
		"\U0001f1f0\U0001f1f7서울":                     {"서울"},
		"카페\u200b":                                   {"카페"},
		"\ufeff카페":                                   {"카페"},
		"10":                                         {"10"},
		"#1":                                         {"1"},
		"café":                                       {"café"},
		"3.5km.":                                     {"3.5km"},
		"6,500원,":                                    {"6,500원"},
	} {
		if got := Tokens(input); !reflect.DeepEqual(got, want) && !(len(got) == 0 && len(want) == 0) {
			t.Errorf("Tokens(%q) = %q, want %q", input, got, want)
		}
	}
}
