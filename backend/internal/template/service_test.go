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
	return Limits{
		NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000,
		MaxPerAccount: 3, MaxRepeatExpansion: 40,
	}
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

func (f *fakeStore) Update(_ context.Context, userID, id string, patch Patch, at time.Time) (Template, error) {
	row, ok := f.rows[id]
	if !ok || row.UserID != userID {
		return Template{}, ErrNotFound
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
			_, err := svc.Create(context.Background(), "alice", "리뷰", "", tc.body)
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

	if _, err := svc.Create(ctx, "alice", "  ", "", okBody); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("empty name error = %v", err)
	}
	if _, err := svc.Create(ctx, "alice", "리뷰", "", "   "); !errors.Is(err, ErrBodyRequired) {
		t.Fatalf("empty body error = %v", err)
	}
	var tooLong *FieldTooLongError
	if _, err := svc.Create(ctx, "alice", strings.Repeat("가", 41), "", okBody); !errors.As(err, &tooLong) || tooLong.Field != "name" {
		t.Fatalf("long name error = %v", err)
	}
	// A Hangul syllable counts as ONE character on both sides of the wire.
	if _, err := svc.Create(ctx, "alice", strings.Repeat("가", 40), "", okBody); err != nil {
		t.Fatalf("a 40-syllable name must fit: %v", err)
	}

	created, err := svc.Create(ctx, "alice", "다른 이름", "", okBody)
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
	if _, err := svc.Create(ctx, "alice", "리뷰", "", okBody); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "alice", " 리뷰 ", "", okBody); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate after trim error = %v", err)
	}
	// Another account may hold the same name.
	if _, err := svc.Create(ctx, "bob", "리뷰", "", okBody); err != nil {
		t.Fatalf("a foreign account's name collided: %v", err)
	}
	for _, name := range []string{"둘", "셋"} {
		if _, err := svc.Create(ctx, "alice", name, "", okBody); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Create(ctx, "alice", "넷", "", okBody); !errors.Is(err, ErrTooMany) {
		t.Fatalf("cap error = %v", err)
	}
}

// A13: a foreign id is NotFound and indistinguishable from an unknown one.
func TestAForeignIDIsIndistinguishableFromAnUnknownOne(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	bobs, err := svc.Create(ctx, "bob", "리뷰", "", okBody)
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
		if _, ok, err := svc.RenderedFor(ctx, "alice", id, nil); ok || err != nil {
			t.Fatalf("%s render = ok:%v err:%v", label, ok, err)
		}
	}
}

// A6/A11: the render expands for the attachments it is GIVEN, and refuses past the bound
// rather than sending an unbounded prompt.
func TestRenderedForExpandsAndBounds(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "alice", "리뷰", "", okBody)
	if err != nil {
		t.Fatal(err)
	}

	rendered, ok, err := svc.RenderedFor(ctx, "alice", created.ID, []string{"a.jpg", "b.jpg"})
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
	if _, ok, err := svc.RenderedFor(ctx, "alice", "", nil); ok || err != nil {
		t.Fatalf("empty id = ok:%v err:%v", ok, err)
	}

	many := make([]string, 41)
	for i := range many {
		many[i] = "p.jpg"
	}
	if _, _, err := svc.RenderedFor(ctx, "alice", created.ID, many); !errors.Is(err, ErrExpansionTooLarge) {
		t.Fatalf("over-bound render error = %v", err)
	}
}

// A12: the delete reports how many posts it detached.
func TestDeleteReportsTheDetachCount(t *testing.T) {
	svc, store := newService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "alice", "리뷰", "", okBody)
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
	created, err := svc.Create(ctx, "alice", "리뷰", "", okBody)
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
