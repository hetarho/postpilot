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
			text: "제주 협재\t해변\n물빛",
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
