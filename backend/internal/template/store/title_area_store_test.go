package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/template"
)

const titleArea = `[맛집] <write>가게 이름과 대표 메뉴</write>`

// TMPL-2, TMPL-8: the title area is stored, read back by every read, and follows the presence
// rule on update — absent is named by no statement, present empty clears it.
func TestTitleAreaRoundTripsAndAPresentEmptyClearsIt(t *testing.T) {
	s, handle := newStore(t)
	ctx := context.Background()
	shaped := row("t1", "alice", "리뷰")
	shaped.TitleArea = titleArea
	if err := s.Insert(ctx, shaped, 10); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "alice", "t1")
	if err != nil || got.TitleArea != titleArea {
		t.Fatalf("get = %q, %v", got.TitleArea, err)
	}

	// A template stored before the title area existed reads as none.
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	if _, err := handle.Writer.Exec(
		"INSERT INTO templates(id,user_id,name,description,body,created_at,updated_at) VALUES('legacy','alice','예전',?,?,?,?)",
		"", body, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	listed, err := s.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]string{}
	for _, value := range listed {
		titles[value.ID] = value.TitleArea
	}
	if len(titles) != 2 || titles["t1"] != titleArea || titles["legacy"] != "" {
		t.Fatalf("listed title areas = %v", titles)
	}

	// Absent: a name edit leaves the title area alone.
	name := "리뷰 2"
	renamed, err := s.Update(ctx, "alice", "t1", template.Patch{Name: &name}, testNow.Add(time.Minute), nil)
	if err != nil || renamed.TitleArea != titleArea {
		t.Fatalf("a name edit touched the title area: %+v, %v", renamed, err)
	}
	// Present, in one transaction with another present field.
	next, newBody := "<write>새 제목</write>", "<write>새 본문</write>"
	both, err := s.Update(ctx, "alice", "t1", template.Patch{Body: &newBody, TitleArea: &next}, testNow.Add(2*time.Minute), nil)
	if err != nil || both.TitleArea != next || both.Body != newBody || !both.UpdatedAt.Equal(testNow.Add(2*time.Minute)) {
		t.Fatalf("body and title area together = %+v, %v", both, err)
	}
	// Present and empty clears it.
	empty := ""
	cleared, err := s.Update(ctx, "alice", "t1", template.Patch{TitleArea: &empty}, testNow.Add(3*time.Minute), nil)
	if err != nil || cleared.TitleArea != "" || cleared.Body != newBody {
		t.Fatalf("clearing the title area = %+v, %v", cleared, err)
	}
	// A title area alone still answers for ownership like every other statement.
	if _, err := s.Update(ctx, "bob", "t1", template.Patch{TitleArea: &next}, testNow, nil); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("a foreign title-area edit = %v", err)
	}
}

// The check sees the row as the update's own transaction reads it, before any statement, and
// its refusal leaves everything as it was, updated_at included. An unknown or foreign id is
// not found before the check is ever asked.
func TestUpdateRunsTheCheckOnTheStoredRowBeforeAnyStatement(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	shaped := row("t1", "alice", "리뷰")
	shaped.TitleArea = titleArea
	if err := s.Insert(ctx, shaped, 10); err != nil {
		t.Fatal(err)
	}

	refused := errors.New("the counterpart does not fit")
	var seen []template.Template
	check := func(current template.Template) error {
		seen = append(seen, current)
		return refused
	}
	name, next := "새 이름", "<write>새 제목</write>"
	if _, err := s.Update(ctx, "alice", "t1", template.Patch{Name: &name, TitleArea: &next}, testNow.Add(time.Hour), check); !errors.Is(err, refused) {
		t.Fatalf("a refusing check = %v", err)
	}
	if len(seen) != 1 || seen[0].Body != body || seen[0].TitleArea != titleArea || seen[0].Name != "리뷰" {
		t.Fatalf("the check saw %+v, want the stored row", seen)
	}
	stored, err := s.Get(ctx, "alice", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "리뷰" || stored.TitleArea != titleArea || !stored.UpdatedAt.Equal(testNow) {
		t.Fatalf("a refused update wrote something: %+v", stored)
	}

	for label, target := range map[string]struct{ user, id string }{
		"unknown": {"alice", "nobody"}, "foreign": {"bob", "t1"},
	} {
		seen = nil
		if _, err := s.Update(ctx, target.user, target.id, template.Patch{TitleArea: &next}, testNow, check); !errors.Is(err, template.ErrNotFound) || len(seen) != 0 {
			t.Fatalf("%s id = %v with %d checks run", label, err, len(seen))
		}
	}

	accept := func(template.Template) error { return nil }
	updated, err := s.Update(ctx, "alice", "t1", template.Patch{TitleArea: &next}, testNow.Add(time.Hour), accept)
	if err != nil || updated.TitleArea != next {
		t.Fatalf("an accepting check = %+v, %v", updated, err)
	}
}
