package template

import (
	"reflect"
	"strings"
	"testing"
)

const askBody = `방문 기록
<ask label="방문일"/>
<write>인트로</write>
<ask label="총평 별점">별점과 한 줄 총평을 쓰세요</ask>
<slot kind="photo"/>`

func renderAsks(t *testing.T, body string, hasPhotos bool, answers []Answer) Rendered {
	t.Helper()
	nodes, err := Parse(body, fixtureParseOptions)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Render("리뷰", nodes, hasPhotos, answers)
}

// The guarantee TMPL-45 buys: an off, blank or unanswered field leaves a body byte for
// byte identical to the same template with that node and its lead deleted from the source.
// Nothing names the field, and nothing leaves a gap where it was. 방문 기록 is written directly
// onto 방문일, so it is that field's lead and leaves with it.
func TestAnUnusableFieldRendersAsIfItWereNotInTheBody(t *testing.T) {
	withoutBoth := "\n<write>인트로</write>\n\n<slot kind=\"photo\"/>"

	cases := []struct {
		name    string
		answers []Answer
	}{
		{"no answer at all", nil},
		{"both switched off", []Answer{
			{Label: "방문일", Text: "2026-03-01", Enabled: false},
			{Label: "총평 별점", Text: "4.5점", Enabled: false},
		}},
		{"both blank", []Answer{
			{Label: "방문일", Text: "", Enabled: true},
			{Label: "총평 별점", Text: "   ", Enabled: true},
		}},
		{"a blank the shared blank set covers", []Answer{
			{Label: "방문일", Text: "\ufeff", Enabled: true},
			{Label: "총평 별점", Text: "　", Enabled: true},
		}},
		{"answers for labels this body never declared", []Answer{
			{Label: "다른 칸", Text: "값", Enabled: true},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := renderAsks(t, askBody, true, tc.answers)
			want := renderAsks(t, withoutBoth, true, nil)
			if rendered.Body != want.Body {
				t.Fatalf("body is not the node-deleted equivalent:\n got %q\nwant %q", rendered.Body, want.Body)
			}
			if len(rendered.Facts) != 0 {
				t.Errorf("facts = %+v, want none", rendered.Facts)
			}
			// Nothing may hint that the field existed.
			for _, hint := range []string{"방문일", "총평", "<facts", "<ask"} {
				if strings.Contains(rendered.Body, hint) {
					t.Errorf("the dropped field left %q in the body:\n%s", hint, rendered.Body)
				}
			}
		})
	}
}

func TestBothFlavorsRenderWhatTheirAnswerSupplies(t *testing.T) {
	rendered := renderAsks(t, askBody, true, []Answer{
		{Label: "방문일", Text: "  2026-03-01  ", Enabled: true},
		{Label: "총평 별점", Text: "4.5점, 재방문 의사 있음", Enabled: true},
	})

	// The verbatim flavor IS literal text on the page: no tag, and trimmed.
	if !strings.Contains(rendered.Body, "방문 기록\n2026-03-01\n") {
		t.Errorf("the verbatim field did not render as literal text:\n%s", rendered.Body)
	}
	if strings.Contains(rendered.Body, "<facts label=\"방문일\"") {
		t.Errorf("the verbatim field fenced its value as fact:\n%s", rendered.Body)
	}
	// The write flavor keeps the author's instruction and fences the value beside it.
	if !strings.Contains(rendered.Body, "<write>별점과 한 줄 총평을 쓰세요</write>\n<facts label=\"총평 별점\">4.5점, 재방문 의사 있음</facts>") {
		t.Errorf("the write field did not fence its value:\n%s", rendered.Body)
	}
	if len(rendered.Facts) != 1 || rendered.Facts[0].Label != "총평 별점" || rendered.Facts[0].Value != "4.5점, 재방문 의사 있음" {
		t.Errorf("facts = %+v", rendered.Facts)
	}
}

// Facts are recorded in BODY order, which is the order the prompt reads them in.
func TestFactsFollowBodyOrder(t *testing.T) {
	body := `<ask label="둘째">둘째 지시</ask>
<ask label="첫째">첫째 지시</ask>`
	rendered := renderAsks(t, body, false, []Answer{
		{Label: "첫째", Text: "1", Enabled: true},
		{Label: "둘째", Text: "2", Enabled: true},
	})
	if len(rendered.Facts) != 2 || rendered.Facts[0].Label != "둘째" || rendered.Facts[1].Label != "첫째" {
		t.Fatalf("facts = %+v", rendered.Facts)
	}
}

// A title holding a quote stays ESCAPED inside the fact tag's attribute — it is a quoted
// attribute, so the alternative is a tag that stops being readable where the legend told the
// model to read. The frozen Fact record keeps the decoded title, which is what a person reads.
func TestAskLabelKeepsItsEscapeInTheAttribute(t *testing.T) {
	rendered := renderAsks(t, `<ask label="네이버 &quot;별점&quot;">지시</ask>`, false, []Answer{
		{Label: `네이버 "별점"`, Text: "4.5", Enabled: true},
	})
	if !strings.Contains(rendered.Body, `<facts label="네이버 &quot;별점&quot;">4.5</facts>`) {
		t.Errorf("the attribute lost its escape:\n%s", rendered.Body)
	}
	if len(rendered.Facts) != 1 || rendered.Facts[0].Label != `네이버 "별점"` {
		t.Errorf("facts = %+v, want the decoded title", rendered.Facts)
	}
}

// A wizard section is a heading, maybe fixed text, then its field and photo place (TMPL-74). A
// skipped field takes the lines written directly onto it (TMPL-45), so no empty heading is left.
func TestASkippedFieldTakesTheLinesWrittenDirectlyOntoIt(t *testing.T) {
	const wizard = "📦 상자 안에는 뭐가 들었냐면요...\n<ask label=\"구성품\">구성품</ask>\n<slot kind=\"photo\" count=\"2\"/>\n" +
		"⭐ 별점\n평가 기준\n⭐ 다시는 안 갈 것 같음\n<ask label=\"별점\">직접 매긴 별점과 그 이유</ask>\n" +
		"🛒 어디서 사냐구요?\n<ask label=\"구매처\">가격과 살 수 있는 곳</ask>"
	cases := []struct {
		name, body, want string
		answers          []Answer
	}{
		{"the area's first heading", wizard, "\n<slot kind=\"photo\" count=\"2\"/>\n" +
			"⭐ 별점\n평가 기준\n⭐ 다시는 안 갈 것 같음\n<ask label=\"별점\">직접 매긴 별점과 그 이유</ask>\n" +
			"🛒 어디서 사냐구요?\n<ask label=\"구매처\">가격과 살 수 있는 곳</ask>",
			[]Answer{{Label: "별점", Text: "4점", Enabled: true}, {Label: "구매처", Text: "온라인", Enabled: true}}},
		{"a heading and its fixed text after a photo place", wizard, "📦 상자 안에는 뭐가 들었냐면요...\n<ask label=\"구성품\">구성품</ask>\n<slot kind=\"photo\" count=\"2\"/>\n\n" +
			"🛒 어디서 사냐구요?\n<ask label=\"구매처\">가격과 살 수 있는 곳</ask>",
			[]Answer{{Label: "구성품", Text: "본체", Enabled: true}, {Label: "별점", Text: "4점", Enabled: false}, {Label: "구매처", Text: "온라인", Enabled: true}}},
		{"every section skipped", wizard, "\n<slot kind=\"photo\" count=\"2\"/>\n\n", nil},
		{"a blank line keeps the line", "머리말\n\n<ask label=\"a\">주제</ask>\n끝", "머리말\n\n\n끝", nil},
		{"text before the field on its line stays", "가격: <ask label=\"a\"/>\n끝", "가격: \n끝", nil},
		{"a line holding a tag is never lead", "<write>도입</write> 소개\n<ask label=\"a\">주제</ask>\n끝", "<write>도입</write> 소개\n\n끝", nil},
		{"stacked fields keep the lead while one has a value", "정보\n<ask label=\"a\"/>\n<ask label=\"b\"/>\n끝",
			"정보\n\n<ask label=\"b\"/>\n끝", []Answer{{Label: "b", Text: "값", Enabled: true}}},
		{"stacked fields take the lead when all leave", "앞\n\n정보\n<ask label=\"a\"/>\n<ask label=\"b\"/>\n끝", "앞\n\n\n\n끝", nil},
		{"a required field keeps its lead", "머리말\n<ask label=\"a\" required=\"true\">주제</ask>", "머리말\n<ask label=\"a\" required=\"true\">주제</ask>",
			[]Answer{{Label: "a", Text: "값", Enabled: true}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderAsks(t, tc.body, true, tc.answers)
			// The expectation is the template as if the skipped fields and their leads had never
			// been written, rendered with the same answers.
			want := renderAsks(t, tc.want, true, tc.answers)
			if got.Body != want.Body {
				t.Fatalf("body:\n got %q\nwant %q", got.Body, want.Body)
			}
			if !reflectEqualParts(got.BodyParts, want.BodyParts) {
				t.Fatalf("material parts differ:\n got %+v\nwant %+v", got.BodyParts, want.BodyParts)
			}
		})
	}
}

func reflectEqualParts(a, b []MaterialPart) bool {
	return reflect.DeepEqual(a, b)
}
