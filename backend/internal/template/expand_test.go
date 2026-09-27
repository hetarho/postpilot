package template

import (
	"strings"
	"testing"
)

const interviewBody = `<write>인트로를 작성합니다.</write>

=========================
별 <write>별점을 별 기호로</write>

<slot kind="place" label="네이버 지도"/>

=========================
<repeat each="photo">
<slot kind="photo"/>
<write>이 사진에 대한 설명</write>
</repeat>

<write>총평 및 재방문 의사</write>`

func renderInterview(t *testing.T, hasPhotos bool) Rendered {
	t.Helper()
	nodes, err := Parse(interviewBody, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	return Render("정보성 식당 리뷰", nodes, hasPhotos, nil)
}

func renderBody(t *testing.T, body string, hasPhotos bool) string {
	t.Helper()
	nodes, err := Parse(body, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	return Render("x", nodes, hasPhotos, nil).Body
}

// TMPL-21: a photo place binds no photo and a repeat renders once, marked as the part the writer
// repeats per photo group. No attachment is named anywhere in the frozen body.
func TestRenderMarksPlacesAndRendersARepeatOnce(t *testing.T) {
	rendered := renderInterview(t, true)

	want := "<repeat>\n" + PhotoPlace(1) + "\n<write>이 사진에 대한 설명</write>\n</repeat>"
	if strings.Count(rendered.Body, want) != 1 {
		t.Fatalf("the repeat did not render once as its marked part:\n%s", rendered.Body)
	}
	if got := strings.Count(rendered.Body, "<write>이 사진에 대한 설명</write>"); got != 1 {
		t.Fatalf("the repeated write rendered %d times, want once", got)
	}
	// Literals render exactly; the separator appears once per authored occurrence.
	if got := strings.Count(rendered.Body, "========================="); got != 2 {
		t.Fatalf("separator rendered %d times, want 2", got)
	}
	// The stored place position reads as its label's text (TMPL-37).
	if !strings.Contains(rendered.Body, "\n네이버 지도\n") {
		t.Fatalf("the place label is missing from:\n%s", rendered.Body)
	}
	for _, leak := range []string{"{{photo:", "<slot", "<repeat each"} {
		if strings.Contains(rendered.Body, leak) {
			t.Fatalf("%q reached the render:\n%s", leak, rendered.Body)
		}
	}
}

// A post with no photo drops every repeat whole, literals included, and marks no photo place.
func TestRenderWithNoPhotosDropsTheWholeRepeat(t *testing.T) {
	rendered := renderInterview(t, false)

	for _, gone := range []string{"이 사진에 대한 설명", "{{사진 자리", "<repeat>"} {
		if strings.Contains(rendered.Body, gone) {
			t.Fatalf("%q survived a photoless post:\n%s", gone, rendered.Body)
		}
	}
	for _, keep := range []string{"인트로를 작성합니다.", "별점을 별 기호로", "총평 및 재방문 의사", "=========================", "네이버 지도"} {
		if !strings.Contains(rendered.Body, keep) {
			t.Fatalf("%q was dropped with the repeat:\n%s", keep, rendered.Body)
		}
	}
}

// TMPL-40: each place carries its own row size, and a place outside a repeat marks nothing when
// the post has no photo to stand there.
func TestRenderMarksEachPlaceWithItsRowSize(t *testing.T) {
	body := `<slot kind="photo"/>|<slot kind="photo" count="3"/>|<slot kind="photo" count="4"/>`
	if got, want := renderBody(t, body, true), "{{사진 자리 · 한 줄 1장}}|{{사진 자리 · 한 줄 3장}}|{{사진 자리 · 한 줄 4장}}"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if got := renderBody(t, body, false); got != "||" {
		t.Fatalf("a photoless post marked a place: %q", got)
	}
}

// The repeat marker carries no attribute, so the legend can name it exactly, and its places
// render inside it once.
func TestRenderWrapsARepeatOnceWithABareMarker(t *testing.T) {
	body := `<repeat each="photo"><slot kind="photo"/>-<slot kind="photo" count="2"/>|</repeat>`
	if got, want := renderBody(t, body, true), "<repeat>"+PhotoPlace(1)+"-"+PhotoPlace(2)+"|</repeat>"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if got := renderBody(t, body, false); got != "" {
		t.Fatalf("a photoless post kept the repeat: %q", got)
	}
}

// TMPL-37: a stored place/link position is literal text — its label, or 지도 · 링크 when it has
// none — so no run carries a slot token, and nothing downstream has a slot to resolve.
func TestRenderWritesAPlaceOrLinkSlotAsItsLabel(t *testing.T) {
	nodes, err := Parse(`<slot kind="place" label="가게 &amp; 지도"/><write>a</write><slot kind="link" label="예약"/>|<slot kind="place"/>|<slot kind="link" label="  "/>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered := Render("x", nodes, false, nil)
	if want := "가게 & 지도<write>a</write>예약|지도|링크"; rendered.Body != want {
		t.Fatalf("body = %q, want %q", rendered.Body, want)
	}
	if strings.Contains(rendered.Body, "{{") {
		t.Fatalf("a token reached the render: %q", rendered.Body)
	}
}

func TestRenderDecodesEscapesForThePrompt(t *testing.T) {
	nodes, err := Parse(`&lt;b&gt; 그리고 A &amp; B<write>&lt;write&gt; 설명</write>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered := Render("x", nodes, false, nil)
	if want := "<b> 그리고 A & B<write><write> 설명</write>"; rendered.Body != want {
		t.Fatalf("body = %q, want %q", rendered.Body, want)
	}
}
