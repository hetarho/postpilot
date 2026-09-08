package template

import (
	"strings"
	"testing"
)

const askBody = `방문 기록
<ask label="방문일"/>
<write>인트로</write>
<ask label="총평 별점">별점과 한 줄 총평을 쓰세요</ask>
<slot kind="photo"/>`

func renderAsks(t *testing.T, body string, filenames []string, answers []Answer) Rendered {
	t.Helper()
	nodes, err := Parse(body, fixtureParseOptions)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rendered, err := Render("리뷰", nodes, filenames, 40, answers)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return rendered
}

// The guarantee TEMPLATE-45 buys: an off, blank or unanswered field leaves a body byte for
// byte identical to the same template with that node deleted from the source. Nothing names
// the field, and nothing leaves a gap where it was.
func TestAnUnusableFieldRendersAsIfItWereNotInTheBody(t *testing.T) {
	withoutBoth := "방문 기록\n\n<write>인트로</write>\n\n<slot kind=\"photo\"/>"

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
			rendered := renderAsks(t, askBody, []string{"a.jpg"}, tc.answers)
			want := renderAsks(t, withoutBoth, []string{"a.jpg"}, nil)
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
	rendered := renderAsks(t, askBody, []string{"a.jpg"}, []Answer{
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
	rendered := renderAsks(t, body, nil, []Answer{
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
	rendered := renderAsks(t, `<ask label="네이버 &quot;별점&quot;">지시</ask>`, nil, []Answer{
		{Label: `네이버 "별점"`, Text: "4.5", Enabled: true},
	})
	if !strings.Contains(rendered.Body, `<facts label="네이버 &quot;별점&quot;">4.5</facts>`) {
		t.Errorf("the attribute lost its escape:\n%s", rendered.Body)
	}
	if len(rendered.Facts) != 1 || rendered.Facts[0].Label != `네이버 "별점"` {
		t.Errorf("facts = %+v, want the decoded title", rendered.Facts)
	}
}

// Resolution runs BEFORE expansion, so a dropped field cannot be priced by the bound. This
// body would exceed a bound of 1 only if the dropped node still counted.
func TestADroppedFieldIsNotPricedByTheExpansionBound(t *testing.T) {
	body := "<ask label=\"총평\">지시</ask>\n<repeat each=\"photo\">\n<slot kind=\"photo\"/>\n</repeat>"
	nodes, err := Parse(body, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	// Two photos, one photo per iteration: two iterations, which is the whole budget.
	if _, err := Render("리뷰", nodes, []string{"a.jpg", "b.jpg"}, 2, nil); err != nil {
		t.Fatalf("a dropped field was counted against the bound: %v", err)
	}
	// And the bound still refuses what it should.
	if _, err := Render("리뷰", nodes, []string{"a.jpg", "b.jpg", "c.jpg"}, 2, nil); err == nil {
		t.Error("the expansion bound stopped refusing")
	}
}
