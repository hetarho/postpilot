package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

var testNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// newStore opens a throwaway SQLite database with the embedded migrations applied and seeds
// two accounts with two templates each — scope links are about real templates, and the account
// boundary is only provable with a second account present.
func newStore(t *testing.T) (*store.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	stamp := testNow.UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(context.Background(), auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: testNow}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
		for _, suffix := range []string{"p1", "p2"} {
			if _, err := handle.Writer.Exec(
				"INSERT INTO templates(id,user_id,name,description,body,created_at,updated_at) VALUES(?,?,?,'','<write>본문</write>',?,?)",
				id+"-"+suffix, id, suffix, stamp, stamp); err != nil {
				t.Fatalf("seed template: %v", err)
			}
		}
	}
	return store.New(handle.Writer, handle.Reader), handle
}

func newGuideline(id, userID, text string, scope guideline.Scope, at time.Time, templateIDs ...string) guideline.Guideline {
	return guideline.Guideline{
		ID: id, UserID: userID, Text: text, Scope: scope, TemplateIDs: templateIDs,
		CreatedAt: at, UpdatedAt: at,
	}
}

func TestInsertRefusesADuplicateTextWithinTheAccountOnly(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "CCTV 언급 금지", guideline.ScopeGlobal, testNow), 10); err != nil {
		t.Fatal(err)
	}
	err := s.Insert(ctx, newGuideline("g2", "alice", "CCTV 언급 금지", guideline.ScopeGlobal, testNow), 10)
	if !errors.Is(err, guideline.ErrDuplicateText) {
		t.Fatalf("duplicate err = %v", err)
	}
	// The same text is another account's business entirely.
	if err := s.Insert(ctx, newGuideline("g3", "bob", "CCTV 언급 금지", guideline.ScopeGlobal, testNow), 10); err != nil {
		t.Fatalf("cross-account duplicate refused: %v", err)
	}
}

// A1: the cap is enforced by the insert itself, so a refused create leaves no row behind.
func TestInsertRefusesPastTheAccountCap(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for i, text := range []string{"a", "b"} {
		if err := s.Insert(ctx, newGuideline("g"+text, "alice", text, guideline.ScopeGlobal, testNow.Add(time.Duration(i)*time.Minute)), 2); err != nil {
			t.Fatal(err)
		}
	}
	var atCap *guideline.AccountCapError
	err := s.Insert(ctx, newGuideline("gc", "alice", "c", guideline.ScopeGlobal, testNow), 2)
	if !errors.As(err, &atCap) || atCap.Max != 2 {
		t.Fatalf("cap err = %v", err)
	}
	listed, err := s.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("a refused insert changed the row count: %d", len(listed))
	}
	// The cap is per account, so the other account is unaffected by a full neighbour.
	if err := s.Insert(ctx, newGuideline("bob-g1", "bob", "c", guideline.ScopeGlobal, testNow), 2); err != nil {
		t.Fatalf("bob refused at alice's cap: %v", err)
	}
}

// A2: the composite foreign key, not a service check, is what stops a foreign template link.
func TestInsertRefusesAForeignTemplateLink(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	err := s.Insert(ctx, newGuideline("g1", "alice", "a", guideline.ScopeTemplates, testNow, "bob-p1"), 10)
	if !errors.Is(err, guideline.ErrTemplateNotFound) {
		t.Fatalf("foreign link err = %v", err)
	}
	if _, err := s.Get(ctx, "alice", "g1"); !errors.Is(err, guideline.ErrNotFound) {
		t.Fatalf("the refused insert left a row: %v", err)
	}
}

// A2: the list order IS the injection order — global group first, then scoped, each by
// creation time — so the screen and the prompt cannot disagree.
func TestListReturnsInjectionOrder(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for _, g := range []guideline.Guideline{
		newGuideline("s2", "alice", "scoped later", guideline.ScopeTemplates, testNow.Add(4*time.Minute), "alice-p1"),
		newGuideline("g2", "alice", "global later", guideline.ScopeGlobal, testNow.Add(3*time.Minute)),
		newGuideline("s1", "alice", "scoped early", guideline.ScopeTemplates, testNow.Add(2*time.Minute), "alice-p2"),
		newGuideline("g1", "alice", "global early", guideline.ScopeGlobal, testNow.Add(time.Minute)),
	} {
		if err := s.Insert(ctx, g, 10); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := s.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"g1", "g2", "s1", "s2"}
	if len(listed) != len(want) {
		t.Fatalf("listed %d guidelines", len(listed))
	}
	for i, id := range want {
		if listed[i].ID != id {
			t.Fatalf("order = %v at %d, want %v", listed[i].ID, i, want)
		}
	}
	if got := listed[2].TemplateIDs; len(got) != 1 || got[0] != "alice-p2" {
		t.Fatalf("scope links = %v", got)
	}
	if len(listed[0].TemplateIDs) != 0 {
		t.Fatalf("a global guideline carried links: %v", listed[0].TemplateIDs)
	}
}

// A3: a text-only patch must not touch the scope, even when a scope edit lands in between.
func TestUpdateTextLeavesAConcurrentScopeEditIntact(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "old", guideline.ScopeGlobal, testNow), 10); err != nil {
		t.Fatal(err)
	}
	// Another tab rescopes it while the text edit is being typed.
	if _, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p1"}},
	}, testNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	text := "new"
	updated, err := s.Update(ctx, "alice", "g1", guideline.Patch{Text: &text}, testNow.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Text != "new" {
		t.Fatalf("text = %q", updated.Text)
	}
	if updated.Scope != guideline.ScopeTemplates || len(updated.TemplateIDs) != 1 || updated.TemplateIDs[0] != "alice-p1" {
		t.Fatalf("the text edit disturbed the scope: %+v", updated)
	}
}

// A3: a scope patch replaces kind and set together, and a refused link rolls the whole
// replacement back rather than leaving a half-applied scope.
func TestUpdateScopeReplacesAtomically(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "a", guideline.ScopeTemplates, testNow, "alice-p1", "alice-p2"), 10); err != nil {
		t.Fatal(err)
	}
	updated, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p2"}},
	}, testNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.TemplateIDs) != 1 || updated.TemplateIDs[0] != "alice-p2" {
		t.Fatalf("scope was not replaced: %v", updated.TemplateIDs)
	}

	if _, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p1", "bob-p1"}},
	}, testNow.Add(2*time.Minute)); !errors.Is(err, guideline.ErrTemplateNotFound) {
		t.Fatalf("foreign link in a replacement err = %v", err)
	}
	rolled, err := s.Get(ctx, "alice", "g1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rolled.TemplateIDs) != 1 || rolled.TemplateIDs[0] != "alice-p2" {
		t.Fatalf("a refused replacement left a partial scope: %v", rolled.TemplateIDs)
	}

	// Going global clears the whole set in the same transaction.
	global, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeGlobal},
	}, testNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if global.Scope != guideline.ScopeGlobal || len(global.TemplateIDs) != 0 {
		t.Fatalf("global scope kept links: %+v", global)
	}
}

func TestUpdateAndDeleteTreatForeignIdsAsUnknown(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "bob", "a", guideline.ScopeGlobal, testNow), 10); err != nil {
		t.Fatal(err)
	}
	text := "x"
	if _, err := s.Update(ctx, "alice", "g1", guideline.Patch{Text: &text}, testNow); !errors.Is(err, guideline.ErrNotFound) {
		t.Fatalf("foreign update err = %v", err)
	}
	if err := s.Delete(ctx, "alice", "g1"); !errors.Is(err, guideline.ErrNotFound) {
		t.Fatalf("foreign delete err = %v", err)
	}
	if _, err := s.Get(ctx, "bob", "g1"); err != nil {
		t.Fatalf("the owner lost the row: %v", err)
	}
}

// A5/A12: scope resolution is exact, and a template deletion leaves an orphaned scoped
// guideline that reaches no prompt at all until it is rescoped.
func TestApplicableTextsResolvesScopeExactly(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	for _, g := range []guideline.Guideline{
		newGuideline("g1", "alice", "전역", guideline.ScopeGlobal, testNow.Add(time.Minute)),
		newGuideline("s1", "alice", "p1 전용", guideline.ScopeTemplates, testNow.Add(2*time.Minute), "alice-p1"),
		newGuideline("s2", "alice", "p2 전용", guideline.ScopeTemplates, testNow.Add(3*time.Minute), "alice-p2"),
		newGuideline("gb", "bob", "밥의 전역", guideline.ScopeGlobal, testNow),
	} {
		if err := s.Insert(ctx, g, 10); err != nil {
			t.Fatal(err)
		}
	}

	for name, tc := range map[string]struct {
		templateID string
		want       []string
	}{
		"with p1":     {templateID: "alice-p1", want: []string{"전역", "p1 전용"}},
		"with p2":     {templateID: "alice-p2", want: []string{"전역", "p2 전용"}},
		"no template": {templateID: "", want: []string{"전역"}},
		// A template id that is not the account's reaches no link row.
		"foreign template": {templateID: "bob-p1", want: []string{"전역"}},
	} {
		got, err := s.ApplicableTexts(ctx, "alice", tc.templateID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s: texts = %v, want %v", name, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%s: texts = %v, want %v", name, got, tc.want)
			}
		}
	}

	// Deleting the template orphans s1: the row survives and applies nowhere.
	if _, err := handle.Writer.Exec("DELETE FROM templates WHERE id='alice-p1'"); err != nil {
		t.Fatal(err)
	}
	got, err := s.ApplicableTexts(ctx, "alice", "alice-p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "전역" || got[1] != "p2 전용" {
		t.Fatalf("after the template delete texts = %v", got)
	}
	orphan, err := s.Get(ctx, "alice", "s1")
	if err != nil {
		t.Fatalf("the orphaned guideline was deleted: %v", err)
	}
	if orphan.Scope != guideline.ScopeTemplates || len(orphan.TemplateIDs) != 0 {
		t.Fatalf("orphan = %+v, want a templates scope with no links", orphan)
	}
}

// A12: deleting a guideline cascades only its own links.
func TestDeleteRemovesOnlyItsOwnLinks(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "a", guideline.ScopeTemplates, testNow, "alice-p1"), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, newGuideline("g2", "alice", "b", guideline.ScopeTemplates, testNow.Add(time.Minute), "alice-p1"), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "alice", "g1"); err != nil {
		t.Fatal(err)
	}
	var links, templates int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM guideline_templates WHERE user_id='alice'").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow("SELECT count(*) FROM templates WHERE user_id='alice'").Scan(&templates); err != nil {
		t.Fatal(err)
	}
	if links != 1 || templates != 2 {
		t.Fatalf("delete cascaded wrong: links=%d templates=%d", links, templates)
	}
}
