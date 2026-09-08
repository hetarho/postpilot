package template

import (
	"errors"
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

func renderInterview(t *testing.T, filenames []string) Rendered {
	t.Helper()
	nodes, err := Parse(interviewBody, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("정보성 식당 리뷰", nodes, filenames, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rendered
}

func TestRenderExpandsOncePerPhoto(t *testing.T) {
	rendered := renderInterview(t, []string{"a.jpg", "b.jpg", "c.jpg"})

	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		if !strings.Contains(rendered.Body, PhotoToken(name)) {
			t.Fatalf("photo token for %s missing from:\n%s", name, rendered.Body)
		}
	}
	if got := strings.Count(rendered.Body, "<write>이 사진에 대한 설명</write>"); got != 3 {
		t.Fatalf("per-photo write rendered %d times, want 3", got)
	}
	// Literals survive expansion exactly; the separator appears once per authored occurrence.
	if got := strings.Count(rendered.Body, "========================="); got != 2 {
		t.Fatalf("separator rendered %d times, want 2", got)
	}
	if len(rendered.Slots) != 1 || rendered.Slots[0].Kind != SlotPlace || rendered.Slots[0].Label != "네이버 지도" {
		t.Fatalf("slots = %+v", rendered.Slots)
	}
	if !strings.Contains(rendered.Body, SlotToken(1)) {
		t.Fatalf("slot token missing from:\n%s", rendered.Body)
	}
	// The grammar's own tags stay, because the legend explains them; a raw photo filename
	// never appears without its token wrapper.
	if strings.Contains(rendered.Body, "<repeat") || strings.Contains(rendered.Body, "<slot") {
		t.Fatalf("container tags leaked into the render:\n%s", rendered.Body)
	}
}

func TestRenderWithNoPhotosDropsTheWholeRepeat(t *testing.T) {
	rendered := renderInterview(t, nil)

	if strings.Contains(rendered.Body, "이 사진에 대한 설명") {
		t.Fatalf("per-photo write survived a photoless post:\n%s", rendered.Body)
	}
	if strings.Contains(rendered.Body, photoTokenPrefix) {
		t.Fatalf("photo token survived a photoless post:\n%s", rendered.Body)
	}
	for _, keep := range []string{"인트로를 작성합니다.", "별점을 별 기호로", "총평 및 재방문 의사", "========================="} {
		if !strings.Contains(rendered.Body, keep) {
			t.Fatalf("%q was dropped with the repeat:\n%s", keep, rendered.Body)
		}
	}
	if len(rendered.Slots) != 1 {
		t.Fatalf("slots = %+v", rendered.Slots)
	}
}

func TestRenderRefusesAnExpansionOverTheBound(t *testing.T) {
	nodes, err := Parse(interviewBody, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	filenames := make([]string, 41)
	for i := range filenames {
		filenames[i] = "p.jpg"
	}
	if _, err := Render("x", nodes, filenames, 40, nil); !errors.Is(err, ErrExpansionTooLarge) {
		t.Fatalf("error = %v, want ErrExpansionTooLarge", err)
	}
	if _, err := Render("x", nodes, filenames[:40], 40, nil); err != nil {
		t.Fatalf("40 iterations must be allowed: %v", err)
	}
}

func TestRenderNumbersSlotsInDocumentOrder(t *testing.T) {
	nodes, err := Parse(`<slot kind="place" label="지도"/><write>a</write><slot kind="link" label="예약"/>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("x", nodes, nil, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered.Slots) != 2 || rendered.Slots[0].Kind != SlotPlace || rendered.Slots[1].Kind != SlotLink {
		t.Fatalf("slots = %+v", rendered.Slots)
	}
	if want := SlotToken(1) + "<write>a</write>" + SlotToken(2); rendered.Body != want {
		t.Fatalf("body = %q, want %q", rendered.Body, want)
	}
}

func TestRenderDecodesEscapesForThePrompt(t *testing.T) {
	nodes, err := Parse(`&lt;b&gt; 그리고 A &amp; B<write>&lt;write&gt; 설명</write>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("x", nodes, nil, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := "<b> 그리고 A & B<write><write> 설명</write>"; rendered.Body != want {
		t.Fatalf("body = %q, want %q", rendered.Body, want)
	}
}

// One position with a count takes that many photos and records the row it bound. The photos
// it did not take stay unbound — a position asks for a row, not for everything left.
func TestRenderBindsACountedPositionAsOneRow(t *testing.T) {
	nodes, err := Parse(`<slot kind="photo" count="3"/>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("x", nodes, []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"}, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := PhotoToken("a.jpg") + "\n" + PhotoToken("b.jpg") + "\n" + PhotoToken("c.jpg")
	if rendered.Body != want {
		t.Fatalf("body = %q, want %q", rendered.Body, want)
	}
	assertRows(t, rendered.Rows, []PhotoRow{{Count: 3, Filenames: []string{"a.jpg", "b.jpg", "c.jpg"}}})
}

// A repeat runs ⌈photos ÷ group⌉ times, and the last group may be short.
func TestRenderGroupsARepeatByItsPositionCount(t *testing.T) {
	nodes, err := Parse(`<repeat each="photo"><slot kind="photo" count="2"/>|</repeat>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("x", nodes, []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"}, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(rendered.Body, "|"); got != 3 {
		t.Fatalf("iterations = %d, want 3 for five photos in twos", got)
	}
	assertRows(t, rendered.Rows, []PhotoRow{
		{Count: 2, Filenames: []string{"a.jpg", "b.jpg"}},
		{Count: 2, Filenames: []string{"c.jpg", "d.jpg"}},
		{Count: 2, Filenames: []string{"e.jpg"}},
	})
}

// Two positions inside one repeat take three photos per iteration together, and the second
// position of a short last iteration renders nothing at all rather than repeating a photo.
func TestRenderShortensTheLastIterationInsteadOfRepeatingAPhoto(t *testing.T) {
	nodes, err := Parse(`<repeat each="photo"><slot kind="photo"/>-<slot kind="photo" count="2"/>|</repeat>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("x", nodes, []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg"}, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(rendered.Body, "|"); got != 2 {
		t.Fatalf("iterations = %d, want 2 for four photos in groups of three", got)
	}
	assertRows(t, rendered.Rows, []PhotoRow{
		{Count: 1, Filenames: []string{"a.jpg"}},
		{Count: 2, Filenames: []string{"b.jpg", "c.jpg"}},
		{Count: 1, Filenames: []string{"d.jpg"}},
	})
	// The last iteration's second position bound nothing, so it contributed no token and no
	// row — but its literals are still there, which is what makes the iteration count visible.
	if strings.Count(rendered.Body, "-") != 2 {
		t.Fatalf("the short iteration lost its literals:\n%s", rendered.Body)
	}
}

// Photos are consumed ONCE across the whole body: two bare positions are two different
// photos, not the first photo twice.
func TestRenderBindsEachPhotoOnlyOnce(t *testing.T) {
	nodes, err := Parse(`<slot kind="photo"/>|<slot kind="photo"/>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("x", nodes, []string{"a.jpg", "b.jpg"}, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := PhotoToken("a.jpg") + "|" + PhotoToken("b.jpg"); rendered.Body != want {
		t.Fatalf("body = %q, want %q", rendered.Body, want)
	}
}

// A position with nothing left renders nothing, and a photoless post keeps no rows at all.
func TestRenderRecordsNoRowForAPositionThatBoundNothing(t *testing.T) {
	nodes, err := Parse(`<slot kind="photo" count="2"/>|<slot kind="photo"/>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render("x", nodes, []string{"a.jpg"}, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := PhotoToken("a.jpg") + "|"; rendered.Body != want {
		t.Fatalf("body = %q, want %q", rendered.Body, want)
	}
	assertRows(t, rendered.Rows, []PhotoRow{{Count: 2, Filenames: []string{"a.jpg"}}})

	empty, err := Render("x", nodes, nil, 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Rows) != 0 {
		t.Fatalf("a photoless post recorded rows: %+v", empty.Rows)
	}
}

// The bound counts ITERATIONS, so a wide row divides the work: forty-one photos through a
// count-2 position is 21 iterations, well inside a bound that forty single photos exceed.
func TestRenderBoundsIterationsRatherThanPhotos(t *testing.T) {
	nodes, err := Parse(`<repeat each="photo"><slot kind="photo" count="2"/></repeat>`, fixtureParseOptions)
	if err != nil {
		t.Fatal(err)
	}
	filenames := make([]string, 41)
	for i := range filenames {
		filenames[i] = "p.jpg"
	}
	if _, err := Render("x", nodes, filenames, 21, nil); err != nil {
		t.Fatalf("21 iterations must be allowed: %v", err)
	}
	if _, err := Render("x", nodes, filenames, 20, nil); !errors.Is(err, ErrExpansionTooLarge) {
		t.Fatalf("error = %v, want ErrExpansionTooLarge", err)
	}
}

func assertRows(t *testing.T, got, want []PhotoRow) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("rows = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Count != want[i].Count || strings.Join(got[i].Filenames, ",") != strings.Join(want[i].Filenames, ",") {
			t.Fatalf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
