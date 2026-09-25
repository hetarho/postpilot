package template

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const okBody = `<write>인트로</write>
=========================
<slot kind="place" label="네이버 지도"/>
<repeat each="photo">
<slot kind="photo"/>
<write>사진 설명</write>
</repeat>`

func testLimits() Limits {
	return NewLimits(Ceilings{
		NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000, TitleAreaMaxChars: 200,
		MaxPerAccount: 3, MaxRepeatExpansion: 40, PhotoRowMax: fixtureParseOptions.PhotoRowMax,
		AskLabelMaxChars: 40, AskMaxPerBody: fixtureParseOptions.AskMaxPerBody,
	},
		// The POST option's bounds, explicit: this package never imports post. The length has a
		// floor and no ceiling, the tag count both.
		NumberBounds{TargetLengthMin: 1, TagCountMin: 1, TagCountMax: 10})
}

// fakeStore is the persistence port. It keeps ownership the way the real SQL does — every
// method scopes by account — so a service test can assert that a foreign id reads as missing.
type fakeStore struct {
	rows    map[string]Template
	inserts int
}

func newFakeStore() *fakeStore { return &fakeStore{rows: map[string]Template{}} }

func (f *fakeStore) Insert(_ context.Context, t Template, maxPerAccount int) error {
	held := 0
	for _, row := range f.rows {
		if row.UserID == t.UserID {
			held++
		}
	}
	if held >= maxPerAccount {
		return ErrTooMany
	}
	for _, row := range f.rows {
		if row.UserID == t.UserID && row.Name == t.Name {
			return ErrDuplicateName
		}
	}
	f.inserts++
	f.rows[t.ID] = t
	return nil
}

func (f *fakeStore) List(_ context.Context, userID string) ([]Template, error) {
	out := []Template{}
	for _, row := range f.rows {
		if row.UserID == userID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeStore) Get(_ context.Context, userID, id string) (Template, error) {
	row, ok := f.rows[id]
	if !ok || row.UserID != userID {
		return Template{}, ErrNotFound
	}
	return row, nil
}

func (f *fakeStore) Update(_ context.Context, userID, id string, patch Patch, at time.Time, check func(current Template) error) (Template, error) {
	row, ok := f.rows[id]
	if !ok || row.UserID != userID {
		return Template{}, ErrNotFound
	}
	// As the real store does: the check reads the stored row before anything is written, and
	// a refusal leaves the row exactly as it was.
	if check != nil {
		if err := check(row); err != nil {
			return Template{}, err
		}
	}
	if patch.Name != nil {
		row.Name = *patch.Name
	}
	if patch.Description != nil {
		row.Description = *patch.Description
	}
	if patch.Body != nil {
		row.Body = *patch.Body
	}
	if patch.TitleArea != nil {
		row.TitleArea = *patch.TitleArea
	}
	row.UpdatedAt = at
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) Delete(_ context.Context, userID, id string) (int, error) {
	row, ok := f.rows[id]
	if !ok || row.UserID != userID {
		return 0, ErrNotFound
	}
	delete(f.rows, id)
	return row.PostCount, nil
}

func newService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	return NewService(store, testLimits()), store
}

// A7: a body that does not parse cannot be saved, and the refusal names the line and reason.
func TestCreateRefusesABodyThatDoesNotParse(t *testing.T) {
	svc, store := newService(t)
	cases := map[string]struct {
		body   string
		line   int
		reason string
	}{
		"unknown tag":   {"<section>x</section>", 1, ReasonUnknownTag},
		"typo":          {"<repaet each=\"photo\">\n<write>a</write>\n</repaet>", 1, ReasonUnknownTag},
		"unclosed":      {"<write>인트로", 1, ReasonUnclosedTag},
		"nested repeat": {"<repeat each=\"photo\">\n<repeat each=\"photo\">\n<write>a</write>\n</repeat>\n</repeat>", 2, ReasonNestedRepeat},
		"empty write":   {"<write>  </write>", 1, ReasonEmptyWrite},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), "alice", Authored{Name: "리뷰", Body: tc.body})
			var parseErr *ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error = %v, want a ParseError", err)
			}
			if parseErr.Line != tc.line || parseErr.Reason != tc.reason {
				t.Fatalf("error = %s on line %d, want %s on line %d",
					parseErr.Reason, parseErr.Line, tc.reason, tc.line)
			}
		})
	}
	if store.inserts != 0 {
		t.Fatalf("a refused body still wrote %d rows", store.inserts)
	}
}

// A7: the bounds and the required fields, per field, so an edit of one is never refused for
// another it did not send.
func TestFieldRules(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, "alice", Authored{Name: "  ", Body: okBody}); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("empty name error = %v", err)
	}
	if _, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: "   "}); !errors.Is(err, ErrBodyRequired) {
		t.Fatalf("empty body error = %v", err)
	}
	var tooLong *FieldTooLongError
	if _, err := svc.Create(ctx, "alice", Authored{Name: strings.Repeat("가", 41), Body: okBody}); !errors.As(err, &tooLong) || tooLong.Field != "name" {
		t.Fatalf("long name error = %v", err)
	}
	// A Hangul syllable counts as ONE character on both sides of the wire.
	if _, err := svc.Create(ctx, "alice", Authored{Name: strings.Repeat("가", 40), Body: okBody}); err != nil {
		t.Fatalf("a 40-syllable name must fit: %v", err)
	}

	created, err := svc.Create(ctx, "alice", Authored{Name: "다른 이름", Body: okBody})
	if err != nil {
		t.Fatal(err)
	}
	// A present empty description clears it; a present empty body is refused.
	empty := ""
	if _, err := svc.Update(ctx, "alice", created.ID, Patch{Description: &empty}); err != nil {
		t.Fatalf("clearing the description was refused: %v", err)
	}
	if _, err := svc.Update(ctx, "alice", created.ID, Patch{Body: &empty}); !errors.Is(err, ErrBodyRequired) {
		t.Fatalf("clearing the body error = %v", err)
	}
}

func TestCreateRefusesADuplicateNameAndTheCap(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: okBody}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "alice", Authored{Name: " 리뷰 ", Body: okBody}); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate after trim error = %v", err)
	}
	// Another account may hold the same name.
	if _, err := svc.Create(ctx, "bob", Authored{Name: "리뷰", Body: okBody}); err != nil {
		t.Fatalf("a foreign account's name collided: %v", err)
	}
	for _, name := range []string{"둘", "셋"} {
		if _, err := svc.Create(ctx, "alice", Authored{Name: name, Body: okBody}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Create(ctx, "alice", Authored{Name: "넷", Body: okBody}); !errors.Is(err, ErrTooMany) {
		t.Fatalf("cap error = %v", err)
	}
}

// A13: a foreign id is NotFound and indistinguishable from an unknown one.
func TestAForeignIDIsIndistinguishableFromAnUnknownOne(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	bobs, err := svc.Create(ctx, "bob", Authored{Name: "리뷰", Body: okBody})
	if err != nil {
		t.Fatal(err)
	}
	name := "새 이름"
	for label, id := range map[string]string{"foreign": bobs.ID, "unknown": "nobody"} {
		if _, err := svc.Update(ctx, "alice", id, Patch{Name: &name}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s update error = %v", label, err)
		}
		if _, err := svc.Delete(ctx, "alice", id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s delete error = %v", label, err)
		}
		if _, ok, err := svc.RenderedFor(ctx, "alice", id, nil, nil); ok || err != nil {
			t.Fatalf("%s render = ok:%v err:%v", label, ok, err)
		}
	}
}

// A6/A11: the render expands for the attachments it is GIVEN, and refuses past the bound
// rather than sending an unbounded prompt.
func TestRenderedForExpandsAndBounds(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: okBody})
	if err != nil {
		t.Fatal(err)
	}

	rendered, ok, err := svc.RenderedFor(ctx, "alice", created.ID, []string{"a.jpg", "b.jpg"}, nil)
	if err != nil || !ok {
		t.Fatalf("render: ok=%v err=%v", ok, err)
	}
	if got := strings.Count(rendered.Body, "<write>사진 설명</write>"); got != 2 {
		t.Fatalf("the repeat expanded %d times, want 2", got)
	}
	if len(rendered.Slots) != 1 || rendered.Slots[0].Kind != SlotPlace {
		t.Fatalf("slots = %+v", rendered.Slots)
	}
	if rendered.Name != "리뷰" {
		t.Fatalf("name = %q", rendered.Name)
	}

	// An empty id is a post with no template: absence, not an error.
	if _, ok, err := svc.RenderedFor(ctx, "alice", "", nil, nil); ok || err != nil {
		t.Fatalf("empty id = ok:%v err:%v", ok, err)
	}

	many := make([]string, 41)
	for i := range many {
		many[i] = "p.jpg"
	}
	if _, _, err := svc.RenderedFor(ctx, "alice", created.ID, many, nil); !errors.Is(err, ErrExpansionTooLarge) {
		t.Fatalf("over-bound render error = %v", err)
	}
}

// A12: the delete reports how many posts it detached.
func TestDeleteReportsTheDetachCount(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: okBody})
	if err != nil {
		t.Fatal(err)
	}
	row := store.rows[created.ID]
	row.PostCount = 2
	store.rows[created.ID] = row

	detached, err := svc.Delete(ctx, "alice", created.ID)
	if err != nil || detached != 2 {
		t.Fatalf("detached = %d err = %v", detached, err)
	}
}

// An update that carries nothing reads the row back rather than writing anything.
func TestAnEmptyPatchWritesNothing(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: okBody})
	if err != nil {
		t.Fatal(err)
	}
	before := store.rows[created.ID].UpdatedAt

	got, err := svc.Update(ctx, "alice", created.ID, Patch{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.Equal(before) {
		t.Fatalf("an empty patch moved updated_at: %v then %v", before, got.UpdatedAt)
	}
}

// titleForm is a title area whose one data field is the write flavor, so it renders an
// instruction plus a fenced fact; plainTitle is the verbatim flavor alone.
const (
	titleForm  = `<ask label="가게 이름">가게 이름을 넣어 쓰세요</ask> 방문 후기`
	plainTitle = `<ask label="가게 이름"></ask>`
	reviewBody = `<ask label="총평">총평을 쓰세요</ask>`
)

// TMPL-50: the title area is stored edge-trimmed, may be empty, is bounded, and is parsed with
// the body as one document — a title error names its area and line, and nothing is written.
func TestCreateStoresATitleAreaAndRefusesOneThatDoesNotParse(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: okBody, TitleArea: "  " + titleForm + "\n"})
	if err != nil {
		t.Fatal(err)
	}
	if created.TitleArea != titleForm || store.rows[created.ID].TitleArea != titleForm {
		t.Fatalf("title area = %q, stored %q", created.TitleArea, store.rows[created.ID].TitleArea)
	}
	blank, err := svc.Create(ctx, "alice", Authored{Name: "제목 없음", Body: okBody, TitleArea: " \n "})
	if err != nil || blank.TitleArea != "" {
		t.Fatalf("a blank title area = %q, %v", blank.TitleArea, err)
	}
	inserted := store.inserts

	for name, tc := range map[string]struct {
		title  string
		line   int
		reason string
	}{
		"a photo position": {"[후기]\n<slot kind=\"photo\"/>", 2, ReasonNotInTitle},
		"a note":           {"<note>메모</note>", 1, ReasonNotInTitle},
		"unclosed":         {"<write>제목", 1, ReasonUnclosedTag},
	} {
		_, err := svc.Create(ctx, "alice", Authored{Name: "거부 " + name, Body: okBody, TitleArea: tc.title})
		var parseErr *ParseError
		if !errors.As(err, &parseErr) || parseErr.Area != AreaTitle || parseErr.Line != tc.line || parseErr.Reason != tc.reason {
			t.Errorf("%s: error = %v, want %s on title line %d", name, err, tc.reason, tc.line)
		}
	}
	// A body error keeps naming the body.
	_, err = svc.Create(ctx, "alice", Authored{Name: "본문 오류", Body: "<write>인트로", TitleArea: titleForm})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Area != AreaBody || parseErr.Reason != ReasonUnclosedTag {
		t.Fatalf("a body error = %v", err)
	}

	// Bounded in Unicode scalar values after the trim: exactly the bound fits.
	var tooLong *FieldTooLongError
	_, err = svc.Create(ctx, "alice", Authored{Name: "긴 제목", Body: okBody, TitleArea: strings.Repeat("가", 201)})
	if !errors.As(err, &tooLong) || tooLong.Field != "title_area" || tooLong.Max != 200 || tooLong.Chars != 201 {
		t.Fatalf("an over-long title area = %v", err)
	}
	if store.inserts != inserted {
		t.Fatalf("a refused title area wrote %d rows", store.inserts-inserted)
	}
	if _, err := svc.Create(ctx, "alice", Authored{Name: "꽉 찬 제목", Body: okBody, TitleArea: strings.Repeat("가", 200)}); err != nil {
		t.Fatalf("a title area at the bound was refused: %v", err)
	}
}

// TMPL-55: a title-only patch is checked against the body as stored. The duplicate is reported
// where the parser meets it second — in the body, at the body's line — which is TMPL-55's
// order, not the field the request touched.
func TestUpdateChecksTheTitleAreaAgainstTheStoredBody(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: "<write>인트로</write>\n" + reviewBody})
	if err != nil {
		t.Fatal(err)
	}
	before := store.rows[created.ID]

	reuse := `<ask label="총평"></ask> 후기`
	_, err = svc.Update(ctx, "alice", created.ID, Patch{TitleArea: &reuse})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Reason != ReasonDuplicateAskLabel || parseErr.Area != AreaBody || parseErr.Line != 2 {
		t.Fatalf("a title reusing a stored body label = %v", err)
	}
	notInTitle := "<note>메모</note>"
	if _, err := svc.Update(ctx, "alice", created.ID, Patch{TitleArea: &notInTitle}); !errors.As(err, &parseErr) || parseErr.Area != AreaTitle {
		t.Fatalf("a title that does not parse = %v", err)
	}
	if after := store.rows[created.ID]; after.TitleArea != before.TitleArea || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("a refused update wrote something: %+v", after)
	}

	title := titleForm
	updated, err := svc.Update(ctx, "alice", created.ID, Patch{TitleArea: &title})
	if err != nil || updated.TitleArea != titleForm {
		t.Fatalf("a title with its own label = %q, %v", updated.TitleArea, err)
	}
	// A present empty title area clears it, and the body is untouched throughout.
	empty := ""
	cleared, err := svc.Update(ctx, "alice", created.ID, Patch{TitleArea: &empty})
	if err != nil || cleared.TitleArea != "" || cleared.Body != before.Body {
		t.Fatalf("clearing the title area = %+v, %v", cleared, err)
	}
}

// The mirror case: a body-only patch is checked against the title area as stored, while an
// edit that names neither area parses nothing at all, as it never has.
func TestUpdateChecksTheBodyAgainstTheStoredTitleArea(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: "<write>본문</write>", TitleArea: plainTitle})
	if err != nil {
		t.Fatal(err)
	}

	reuse := `<ask label="가게 이름">가게를 소개하세요</ask>`
	_, err = svc.Update(ctx, "alice", created.ID, Patch{Body: &reuse})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Reason != ReasonDuplicateAskLabel || parseErr.Area != AreaBody {
		t.Fatalf("a body reusing the stored title's label = %v", err)
	}
	if got := store.rows[created.ID].Body; got != "<write>본문</write>" {
		t.Fatalf("a refused body was written: %q", got)
	}
	body := reviewBody
	if updated, err := svc.Update(ctx, "alice", created.ID, Patch{Body: &body}); err != nil || updated.Body != reviewBody || updated.TitleArea != plainTitle {
		t.Fatalf("a body with its own label = %+v, %v", updated, err)
	}

	// A row edited outside the service no longer parses; renaming it still works.
	row := store.rows[created.ID]
	row.TitleArea = `<slot kind="photo"/>`
	store.rows[created.ID] = row
	name := "새 이름"
	if renamed, err := svc.Update(ctx, "alice", created.ID, Patch{Name: &name}); err != nil || renamed.Name != name {
		t.Fatalf("a name-only edit parsed the stored areas: %+v, %v", renamed, err)
	}
}

// TMPL-20, TMPL-43: one ceiling and one label bound across both areas, title first.
func TestTheAskCeilingCountsBothAreas(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	asks := func(labels ...string) string {
		var out strings.Builder
		for _, label := range labels {
			out.WriteString(`<ask label="` + label + `"></ask>`)
		}
		return out.String()
	}
	// The ceiling is three: two in the title and one in the body fit, a fourth anywhere does not.
	if _, err := svc.Create(ctx, "alice", Authored{Name: "셋", Body: asks("c"), TitleArea: asks("a", "b")}); err != nil {
		t.Fatalf("three asks across both areas were refused: %v", err)
	}
	_, err := svc.Create(ctx, "alice", Authored{Name: "넷", Body: asks("c", "d"), TitleArea: asks("a", "b")})
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Reason != ReasonTooManyAsks || parseErr.Area != AreaBody {
		t.Fatalf("a fourth ask in the body = %v", err)
	}
	if _, err := svc.Create(ctx, "alice", Authored{Name: "넷 제목", Body: "<write>본문</write>", TitleArea: asks("a", "b", "c", "d")}); !errors.As(err, &parseErr) || parseErr.Area != AreaTitle {
		t.Fatalf("a fourth ask in the title = %v", err)
	}

	// A title-area data field's title is bounded like a body one.
	var tooLong *FieldTooLongError
	long := strings.Repeat("가", 41)
	if _, err := svc.Create(ctx, "alice", Authored{Name: "긴 라벨", Body: "<write>본문</write>", TitleArea: asks(long)}); !errors.As(err, &tooLong) || tooLong.Field != "ask_label" || tooLong.Chars != 41 {
		t.Fatalf("an over-long title-area label = %v", err)
	}
}

// TMPL-50, TMPL-55: the title area renders with the post's answers and no photo, a title
// left blank once its asks drop is no title form, and the facts read title first.
func TestRenderedForRendersTheTitleAreaFirst(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	shaped, err := svc.Create(ctx, "alice", Authored{Name: "리뷰", Body: "<repeat each=\"photo\">\n<slot kind=\"photo\"/>\n</repeat>\n" + reviewBody, TitleArea: titleForm})
	if err != nil {
		t.Fatal(err)
	}
	answered := []Answer{
		{Label: "총평", Text: "뼈가 푸짐했다", Enabled: true},
		{Label: "가게 이름", Text: " 을지로 노포 ", Enabled: true},
	}
	rendered, ok, err := svc.RenderedFor(ctx, "alice", shaped.ID, []string{"a.jpg", "b.jpg"}, answered)
	if err != nil || !ok {
		t.Fatalf("render: ok=%v err=%v", ok, err)
	}
	wantTitle := "<write>가게 이름을 넣어 쓰세요</write>\n<facts label=\"가게 이름\">을지로 노포</facts> 방문 후기"
	if rendered.TitleArea != wantTitle {
		t.Fatalf("title area = %q, want %q", rendered.TitleArea, wantTitle)
	}
	if len(rendered.Facts) != 2 || rendered.Facts[0] != (Fact{Label: "가게 이름", Value: "을지로 노포"}) || rendered.Facts[1] != (Fact{Label: "총평", Value: "뼈가 푸짐했다"}) {
		t.Fatalf("facts = %+v, want the title's first", rendered.Facts)
	}
	// The title binds no photo: both went to the body.
	if !strings.Contains(rendered.Body, "{{photo:a.jpg}}") || !strings.Contains(rendered.Body, "{{photo:b.jpg}}") || strings.Contains(rendered.TitleArea, "photo") {
		t.Fatalf("photos bound = body %q, title %q", rendered.Body, rendered.TitleArea)
	}

	// The title's field off: its position drops and the rest of the title stays verbatim.
	off := []Answer{answered[0], {Label: "가게 이름", Text: "을지로 노포", Enabled: false}}
	rendered, _, _ = svc.RenderedFor(ctx, "alice", shaped.ID, nil, off)
	if rendered.TitleArea != " 방문 후기" || len(rendered.Facts) != 1 || rendered.Facts[0].Label != "총평" {
		t.Fatalf("with the title field off = %q, %+v", rendered.TitleArea, rendered.Facts)
	}

	// A title that is nothing but a verbatim field: answered it is the answer, while off or
	// blank it is blank, and a blank title is no title form at all.
	plain, err := svc.Create(ctx, "alice", Authored{Name: "가게 이름만", Body: "<write>본문</write>", TitleArea: plainTitle})
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		answers []Answer
		want    string
	}{
		"answered":   {[]Answer{{Label: "가게 이름", Text: "을지로 노포", Enabled: true}}, "을지로 노포"},
		"off":        {[]Answer{{Label: "가게 이름", Text: "을지로 노포", Enabled: false}}, ""},
		"blank":      {[]Answer{{Label: "가게 이름", Text: "   ", Enabled: true}}, ""},
		"unanswered": {nil, ""},
	} {
		rendered, ok, err := svc.RenderedFor(ctx, "alice", plain.ID, nil, test.answers)
		if err != nil || !ok || rendered.TitleArea != test.want {
			t.Errorf("%s: title area = %q (ok=%v err=%v), want %q", name, rendered.TitleArea, ok, err, test.want)
		}
	}

	// A template without a title area renders exactly as before, with none.
	bare, err := svc.Create(ctx, "alice", Authored{Name: "제목 없음", Body: okBody})
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := Parse(okBody, svc.parseOptions())
	if err != nil {
		t.Fatal(err)
	}
	want, err := Render("제목 없음", nodes, []string{"a.jpg"}, svc.limits.MaxRepeatExpansion, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := svc.RenderedFor(ctx, "alice", bare.ID, []string{"a.jpg"}, nil)
	if err != nil || !ok || got.TitleArea != "" || got.Body != want.Body || len(got.Facts) != 0 {
		t.Fatalf("no title area = %+v (ok=%v err=%v), want %+v", got, ok, err, want)
	}

	// A title area edited outside the service into one that no longer parses: no shape, no error.
	row := store.rows[shaped.ID]
	row.TitleArea = "<repeat each=\"photo\"></repeat>"
	store.rows[shaped.ID] = row
	if _, ok, err := svc.RenderedFor(ctx, "alice", shaped.ID, nil, answered); ok || err != nil {
		t.Fatalf("an unparsable title area = ok:%v err:%v", ok, err)
	}
}
