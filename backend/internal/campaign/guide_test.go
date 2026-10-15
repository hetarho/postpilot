package campaign

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func intPtr(v int) *int { return &v }

// POST-118: the text is trimmed and bounded, items are trimmed and de-duplicated ignoring case and
// spaces (a hashtag loses its '#'), counted items stay inside their ranges, and a clip takes no
// character or photo minimum (CLIP-210).
func TestGuideBoundsAndNormalization(t *testing.T) {
	got, err := Normalize(Guide{
		Text:      "  필수 키워드를 넣어 주세요  ",
		Keywords:  []string{" 답십리맛집 ", "답십리 맛집", "분식"},
		Hashtags:  []string{"#협찬", "협찬", " #답십리 "},
		MinChars:  intPtr(1500),
		MinPhotos: intPtr(15),
	}, PostOptions)
	if err != nil {
		t.Fatal(err)
	}
	want := Guide{Text: "필수 키워드를 넣어 주세요", Keywords: []string{"답십리맛집", "분식"}, Hashtags: []string{"협찬", "답십리"}, MinChars: intPtr(1500), MinPhotos: intPtr(15)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized = %+v", got)
	}
	var tooLong *TooLongError
	if _, err := Normalize(Guide{Text: strings.Repeat("가", GuideMaxChars+1)}, PostOptions); !errors.As(err, &tooLong) || tooLong.Field != FieldText || tooLong.Max != GuideMaxChars {
		t.Fatalf("over-long text = %v", err)
	}
	if _, err := Normalize(Guide{Text: strings.Repeat("가", GuideMaxChars)}, PostOptions); err != nil {
		t.Fatalf("text at the bound = %v", err)
	}
	if _, err := Normalize(Guide{Keywords: []string{strings.Repeat("가", ItemMaxChars+1)}}, PostOptions); !errors.As(err, &tooLong) || tooLong.Field != FieldKeywords {
		t.Fatalf("over-long keyword = %v", err)
	}
	many := make([]string, KeywordMax+1)
	for i := range many {
		many[i] = strings.Repeat("가", i+1)
	}
	for name, guide := range map[string]Guide{
		"too many keywords": {Keywords: many},
		"blank keyword":     {Keywords: []string{"  "}},
		"blank hashtag":     {Hashtags: []string{"#"}},
		"min chars low":     {MinChars: intPtr(MinCharsFloor - 1)},
		"min chars high":    {MinChars: intPtr(MinCharsCeiling + 1)},
		"min photos zero":   {MinPhotos: intPtr(0)},
		"min photos high":   {MinPhotos: intPtr(MinPhotosCeiling + 1)},
	} {
		if _, err := Normalize(guide, PostOptions); !errors.Is(err, ErrItemInvalid) {
			t.Errorf("%s = %v, want ErrItemInvalid", name, err)
		}
	}
	for name, guide := range map[string]Guide{"clip min chars": {MinChars: intPtr(1000)}, "clip min photos": {MinPhotos: intPtr(5)}} {
		if _, err := Normalize(guide, ClipOptions); !errors.Is(err, ErrItemInvalid) {
			t.Errorf("%s = %v, want ErrItemInvalid", name, err)
		}
	}
	if !(Guide{Text: "  "}).Empty() || (Guide{Hashtags: []string{"a"}}).Empty() {
		t.Fatal("Empty misread a guide")
	}
	ticks, err := NormalizeTicks([]string{" 주차 정보 넣기 ", "주차 정보 넣기", "", "매장 분위기 강조"})
	if err != nil || !reflect.DeepEqual(ticks, []string{"주차 정보 넣기", "매장 분위기 강조"}) {
		t.Fatalf("ticks = %q, %v", ticks, err)
	}
	if _, err := NormalizeTicks([]string{strings.Repeat("가", TickMaxChars+1)}); !errors.Is(err, ErrItemInvalid) {
		t.Fatalf("over-long tick = %v", err)
	}
}
