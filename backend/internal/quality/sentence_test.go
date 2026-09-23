package quality

import (
	"reflect"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestSentencesSplitOnTerminatorsAndLineBreaksButNotDecimals(t *testing.T) {
	for name, test := range map[string]struct {
		text string
		want []string
	}{
		"each terminator ends one": {
			text: "을지로 노포에서 감자탕을 먹었다. 국물이 진했다! 또 갈까? 다음엔 볶음밥까지…",
			want: []string{"을지로 노포에서 감자탕을 먹었다.", "국물이 진했다!", "또 갈까?", "다음엔 볶음밥까지…"},
		},
		"a decimal stays whole": {
			text: "역에서 3.5km를 걸었다. 가격은 작년의 1.5배였다.",
			want: []string{"역에서 3.5km를 걸었다.", "가격은 작년의 1.5배였다."},
		},
		"a run of terminators is one boundary": {
			text: "정말 맛있었다... 진짜?! 음……그래도 또 온다",
			want: []string{"정말 맛있었다...", "진짜?!", "음……", "그래도 또 온다"},
		},
		"every line break is one": {
			text: "협재 해변\n물빛이 맑았다\r\n모래가 고왔다",
			want: []string{"협재 해변", "물빛이 맑았다", "모래가 고왔다"},
		},
		"a piece with no letter or digit is dropped": {
			text: "^^. !!! 좋았다.\n\n~~",
			want: []string{"좋았다."},
		},
		"English": {
			text: "We ordered two lattes. The latte art was lovely! Would we go back? Yes.",
			want: []string{"We ordered two lattes.", "The latte art was lovely!", "Would we go back?", "Yes."},
		},
		"no abbreviation is special": {
			text: "Mr. Kim runs the café.",
			want: []string{"Mr.", "Kim runs the café."},
		},
		"a decimal needs a digit on both sides": {
			text: "버전 2. 다음은 .5부터",
			want: []string{"버전 2.", "다음은 .", "5부터"},
		},
	} {
		if got := Sentences(test.text); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: Sentences = %q, want %q", name, got, test.want)
		}
	}
	// NFC first, so a decomposed input splits into composed sentences.
	if got := Sentences(norm.NFD.String("감자탕을 먹었다. 맛있었다.")); !reflect.DeepEqual(got, []string{"감자탕을 먹었다.", "맛있었다."}) {
		t.Errorf("NFD sentences = %q", got)
	}
	if got := Sentences(""); len(got) != 0 {
		t.Errorf("empty text split into %q", got)
	}
}
