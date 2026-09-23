package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
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

func newFieldsGuideline(id, userID, text string, at time.Time, fields ...string) guideline.Guideline {
	return guideline.Guideline{
		ID: id, UserID: userID, Text: text, Scope: guideline.ScopeFields, Fields: fields,
		CreatedAt: at, UpdatedAt: at,
	}
}

func count(t *testing.T, handle *db.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := handle.Reader.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestInsertRefusesADuplicateTextWithinTheAccountOnly(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "CCTV 언급 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{}); err != nil {
		t.Fatal(err)
	}
	err := s.Insert(ctx, newGuideline("g2", "alice", "CCTV 언급 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{})
	if !errors.Is(err, guideline.ErrDuplicateText) {
		t.Fatalf("duplicate err = %v", err)
	}
	// The same text is another account's business entirely.
	if err := s.Insert(ctx, newGuideline("g3", "bob", "CCTV 언급 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{}); err != nil {
		t.Fatalf("cross-account duplicate refused: %v", err)
	}
}

// A1: the cap is enforced by the insert itself, so a refused create leaves no row behind.
func TestInsertRefusesPastTheAccountCap(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for i, text := range []string{"a", "b"} {
		if err := s.Insert(ctx, newGuideline("g"+text, "alice", text, guideline.ScopeGlobal, testNow.Add(time.Duration(i)*time.Minute)), 2, guideline.CandidateApproval{}); err != nil {
			t.Fatal(err)
		}
	}
	var atCap *guideline.AccountCapError
	err := s.Insert(ctx, newGuideline("gc", "alice", "c", guideline.ScopeGlobal, testNow), 2, guideline.CandidateApproval{})
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
	if err := s.Insert(ctx, newGuideline("bob-g1", "bob", "c", guideline.ScopeGlobal, testNow), 2, guideline.CandidateApproval{}); err != nil {
		t.Fatalf("bob refused at alice's cap: %v", err)
	}
}

// A2: the composite foreign key, not a service check, is what stops a foreign template link.
func TestInsertRefusesAForeignTemplateLink(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	err := s.Insert(ctx, newGuideline("g1", "alice", "a", guideline.ScopeTemplates, testNow, "bob-p1"), 10, guideline.CandidateApproval{})
	if !errors.Is(err, guideline.ErrTemplateNotFound) {
		t.Fatalf("foreign link err = %v", err)
	}
	if _, err := s.Get(ctx, "alice", "g1"); !errors.Is(err, guideline.ErrNotFound) {
		t.Fatalf("the refused insert left a row: %v", err)
	}
}

// A2: the list order IS the injection order — the global group, then the template group,
// then the 분야 group, each by creation time (GUIDE-14) — so the screen and the prompt cannot
// disagree. The creation times interleave across the groups, and the group still wins.
func TestListReturnsInjectionOrder(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for _, g := range []guideline.Guideline{
		newFieldsGuideline("f2", "alice", "field later", testNow.Add(6*time.Minute), "domestic_travel"),
		newGuideline("s2", "alice", "scoped later", guideline.ScopeTemplates, testNow.Add(5*time.Minute), "alice-p1"),
		newFieldsGuideline("f1", "alice", "field early", testNow, "restaurant", "cafe"),
		newGuideline("g2", "alice", "global later", guideline.ScopeGlobal, testNow.Add(4*time.Minute)),
		newGuideline("s1", "alice", "scoped early", guideline.ScopeTemplates, testNow.Add(2*time.Minute), "alice-p2"),
		newGuideline("g1", "alice", "global early", guideline.ScopeGlobal, testNow.Add(time.Minute)),
	} {
		if err := s.Insert(ctx, g, 10, guideline.CandidateApproval{}); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := s.List(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"g1", "g2", "s1", "s2", "f1", "f2"}
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
	if len(listed[0].TemplateIDs) != 0 || len(listed[0].Fields) != 0 {
		t.Fatalf("a global guideline carried links: %+v", listed[0])
	}
	// 분야 come back in id order; display order is the client's.
	if got := listed[4]; !reflect.DeepEqual(got.Fields, []string{"cafe", "restaurant"}) || len(got.TemplateIDs) != 0 {
		t.Fatalf("fields guideline = %+v", got)
	}
	if len(listed[2].Fields) != 0 {
		t.Fatalf("a templates guideline carried 분야: %v", listed[2].Fields)
	}
	got, err := s.Get(ctx, "alice", "f1")
	if err != nil || !reflect.DeepEqual(got.Fields, []string{"cafe", "restaurant"}) {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

// The 분야 links are written by the insert's own transaction: a link the table refuses rolls
// the whole create back rather than leaving a fields guideline with part of its set.
func TestInsertWritesFieldLinksInItsOwnTransaction(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	err := s.Insert(ctx, newFieldsGuideline("f1", "alice", "a", testNow, "cafe", "cafe"), 10, guideline.CandidateApproval{})
	if err == nil {
		t.Fatal("a repeated 분야 link was accepted")
	}
	if _, err := s.Get(ctx, "alice", "f1"); !errors.Is(err, guideline.ErrNotFound) {
		t.Fatalf("the refused insert left a row: %v", err)
	}
	if n := count(t, handle, "SELECT count(*) FROM guideline_fields"); n != 0 {
		t.Fatalf("the refused insert left %d 분야 links", n)
	}
}

// A3: a text-only patch must not touch the scope, even when a scope edit lands in between.
func TestUpdateTextLeavesAConcurrentScopeEditIntact(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "old", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{}); err != nil {
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
	s, handle := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "a", guideline.ScopeTemplates, testNow, "alice-p1", "alice-p2"), 10, guideline.CandidateApproval{}); err != nil {
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

	// A rescope in either direction between templates and 분야 leaves no link of the kind it left.
	if _, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p1"}},
	}, testNow.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	fields, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeFields, Fields: []string{"restaurant", "cafe"}},
	}, testNow.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if fields.Scope != guideline.ScopeFields || !reflect.DeepEqual(fields.Fields, []string{"cafe", "restaurant"}) || len(fields.TemplateIDs) != 0 {
		t.Fatalf("templates to 분야 = %+v", fields)
	}
	if n := count(t, handle, "SELECT count(*) FROM guideline_templates WHERE guideline_id = 'g1'"); n != 0 {
		t.Fatalf("the rescope to 분야 left %d template links", n)
	}
	back, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"alice-p2"}},
	}, testNow.Add(6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if back.Scope != guideline.ScopeTemplates || !reflect.DeepEqual(back.TemplateIDs, []string{"alice-p2"}) || len(back.Fields) != 0 {
		t.Fatalf("분야 to templates = %+v", back)
	}
	if n := count(t, handle, "SELECT count(*) FROM guideline_fields WHERE guideline_id = 'g1'"); n != 0 {
		t.Fatalf("the rescope to templates left %d 분야 links", n)
	}

	// A refused template link rolls the 분야 back with everything else.
	if _, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeFields, Fields: []string{"cafe"}},
	}, testNow.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, "alice", "g1", guideline.Patch{
		Scope: &guideline.ScopePatch{Scope: guideline.ScopeTemplates, TemplateIDs: []string{"bob-p1"}},
	}, testNow.Add(8*time.Minute)); !errors.Is(err, guideline.ErrTemplateNotFound) {
		t.Fatalf("foreign link in a rescope from 분야 err = %v", err)
	}
	kept, err := s.Get(ctx, "alice", "g1")
	if err != nil {
		t.Fatal(err)
	}
	if kept.Scope != guideline.ScopeFields || !reflect.DeepEqual(kept.Fields, []string{"cafe"}) || len(kept.TemplateIDs) != 0 {
		t.Fatalf("a refused rescope left %+v", kept)
	}
}

func TestUpdateAndDeleteTreatForeignIdsAsUnknown(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "bob", "a", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{}); err != nil {
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
		// Created first of all, and still injected after every template guideline.
		newFieldsGuideline("f1", "alice", "맛집·카페 전용", testNow, "restaurant", "cafe"),
		newGuideline("g1", "alice", "전역", guideline.ScopeGlobal, testNow.Add(time.Minute)),
		newGuideline("s1", "alice", "p1 전용", guideline.ScopeTemplates, testNow.Add(2*time.Minute), "alice-p1"),
		newGuideline("s2", "alice", "p2 전용", guideline.ScopeTemplates, testNow.Add(3*time.Minute), "alice-p2"),
		newFieldsGuideline("f2", "alice", "여행 전용", testNow.Add(4*time.Minute), "domestic_travel"),
		newGuideline("gb", "bob", "밥의 전역", guideline.ScopeGlobal, testNow),
		newFieldsGuideline("fb", "bob", "밥의 맛집", testNow, "restaurant"),
	} {
		if err := s.Insert(ctx, g, 10, guideline.CandidateApproval{}); err != nil {
			t.Fatal(err)
		}
	}

	for name, tc := range map[string]struct {
		templateID, field string
		want              []string
	}{
		"with p1":     {templateID: "alice-p1", want: []string{"전역", "p1 전용"}},
		"with p2":     {templateID: "alice-p2", want: []string{"전역", "p2 전용"}},
		"no template": {templateID: "", want: []string{"전역"}},
		// A template id that is not the account's reaches no link row.
		"foreign template":   {templateID: "bob-p1", want: []string{"전역"}},
		"p1 and a 분야":        {templateID: "alice-p1", field: "restaurant", want: []string{"전역", "p1 전용", "맛집·카페 전용"}},
		"a 분야 only":          {field: "cafe", want: []string{"전역", "맛집·카페 전용"}},
		"another 분야":         {field: "domestic_travel", want: []string{"전역", "여행 전용"}},
		"a 분야 nothing names": {templateID: "alice-p2", field: "pets", want: []string{"전역", "p2 전용"}},
	} {
		got, err := s.ApplicableTexts(ctx, "alice", tc.templateID, tc.field)
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
	got, err := s.ApplicableTexts(ctx, "alice", "alice-p2", "")
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
	if err := s.Insert(ctx, newGuideline("g1", "alice", "a", guideline.ScopeTemplates, testNow, "alice-p1"), 10, guideline.CandidateApproval{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, newGuideline("g2", "alice", "b", guideline.ScopeTemplates, testNow.Add(time.Minute), "alice-p1"), 10, guideline.CandidateApproval{}); err != nil {
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

	// A fields guideline's 분야 links go with it, and another guideline's stay.
	if err := s.Insert(ctx, newFieldsGuideline("f1", "alice", "c", testNow.Add(2*time.Minute), "restaurant", "cafe"), 10, guideline.CandidateApproval{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, newFieldsGuideline("f2", "alice", "d", testNow.Add(3*time.Minute), "restaurant"), 10, guideline.CandidateApproval{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "alice", "f1"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, handle, "SELECT count(*) FROM guideline_fields WHERE guideline_id = 'f1'"); n != 0 {
		t.Fatalf("deleting f1 left %d of its 분야 links", n)
	}
	if n := count(t, handle, "SELECT count(*) FROM guideline_fields WHERE guideline_id = 'f2'"); n != 1 {
		t.Fatalf("deleting f1 took f2's 분야 links: %d left", n)
	}
}

// presetIs compares a preset, reading a nil and an empty 분야 set as the same none.
func presetIs(t *testing.T, name string, got guideline.Preset, enabled bool, fields ...string) {
	t.Helper()
	if got.Enabled != enabled || len(got.Fields) != len(fields) || (len(fields) > 0 && !reflect.DeepEqual(got.Fields, fields)) {
		t.Fatalf("%s: preset = %+v, want enabled %v with %v", name, got, enabled, fields)
	}
}

// GUIDE-34: nothing is seeded, so an account that never touched the preset has no row, and that
// reads as off with no 분야 rather than as an error.
func TestPresetReadsOffWithNoFieldsWithoutARow(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	got, err := s.Preset(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	presetIs(t, "never touched", got, false)
	// An empty patch is a read: it answers the same and writes nothing.
	got, err = s.UpdatePreset(ctx, "alice", guideline.PresetPatch{}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	presetIs(t, "empty patch", got, false)
	if n := count(t, handle, "SELECT count(*) FROM guideline_presets"); n != 0 {
		t.Fatalf("reading the preset wrote %d rows", n)
	}
}

func TestUpdatePresetIsAPresencePatch(t *testing.T) {
	ctx := context.Background()
	s, handle := newStore(t)
	on, off := true, false
	set := func(ids ...string) *[]string { return &ids }
	update := func(name, userID string, patch guideline.PresetPatch, at time.Time, enabled bool, fields ...string) {
		t.Helper()
		got, err := s.UpdatePreset(ctx, userID, patch, at)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		presetIs(t, name, got, enabled, fields...)
		// The answer is what the transaction committed.
		read, err := s.Preset(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		presetIs(t, name+" read back", read, enabled, fields...)
	}

	update("분야 alone on a first write", "alice", guideline.PresetPatch{Fields: set("restaurant", "cafe")}, testNow, false, "cafe", "restaurant")
	update("enabled alone keeps the 분야", "alice", guideline.PresetPatch{Enabled: &on}, testNow.Add(time.Minute), true, "cafe", "restaurant")
	update("분야 alone keeps enabled", "alice", guideline.PresetPatch{Fields: set("domestic_travel")}, testNow.Add(2*time.Minute), true, "domestic_travel")
	update("both at once", "alice", guideline.PresetPatch{Enabled: &off, Fields: set("pets", "cafe")}, testNow.Add(3*time.Minute), false, "cafe", "pets")
	update("a present empty set clears the 분야", "alice", guideline.PresetPatch{Enabled: &on, Fields: set()}, testNow.Add(4*time.Minute), true)
	update("enabled alone on a first write", "bob", guideline.PresetPatch{Enabled: &on}, testNow, true)

	// An empty patch writes nothing, not even the timestamp.
	var before, after string
	if err := handle.Reader.QueryRow("SELECT updated_at FROM guideline_presets WHERE user_id = 'alice'").Scan(&before); err != nil {
		t.Fatal(err)
	}
	update("an empty patch", "alice", guideline.PresetPatch{}, testNow.Add(time.Hour), true)
	if err := handle.Reader.QueryRow("SELECT updated_at FROM guideline_presets WHERE user_id = 'alice'").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("an empty patch moved updated_at from %s to %s", before, after)
	}
}

// GUIDE-39: the preset lives outside guidelines, so the cap counts guideline rows alone and the
// text uniqueness covers them alone.
func TestThePresetSpendsNeitherTheCapNorTextUniqueness(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	const maxPerAccount = 3
	on := true
	if _, err := s.UpdatePreset(ctx, "alice", guideline.PresetPatch{Enabled: &on, Fields: &[]string{"restaurant", "cafe"}}, testNow); err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"a", "b"} {
		if err := s.Insert(ctx, newGuideline("g"+text, "alice", text, guideline.ScopeGlobal, testNow.Add(time.Duration(i)*time.Minute)), maxPerAccount, guideline.CandidateApproval{}); err != nil {
			t.Fatal(err)
		}
	}
	// At the cap less one with the preset on, one more guideline still fits. The preset stores no
	// text, so none is taken, not even its own name.
	if err := s.Insert(ctx, newGuideline("g-last", "alice", "상위 노출 단어 사용", guideline.ScopeGlobal, testNow.Add(time.Hour)), maxPerAccount, guideline.CandidateApproval{}); err != nil {
		t.Fatalf("the preset spent the cap or a text: %v", err)
	}
	var atCap *guideline.AccountCapError
	if err := s.Insert(ctx, newGuideline("g-over", "alice", "c", guideline.ScopeGlobal, testNow.Add(2*time.Hour)), maxPerAccount, guideline.CandidateApproval{}); !errors.As(err, &atCap) {
		t.Fatalf("past the cap err = %v", err)
	}
}

// --- candidates (change 26) ---

func newCandidate(id, userID, text, postSlug string, at time.Time) guideline.Candidate {
	return guideline.Candidate{
		ID: id, UserID: userID, Text: text, PostSlug: postSlug,
		Status: guideline.CandidateStatusPending, Occurrences: 1, FirstSeenAt: at, LastSeenAt: at,
	}
}

func TestRecordCandidateCountsARepeatInsteadOfDuplicating(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if recorded, err := s.RecordCandidate(ctx, newCandidate("c1", "alice", "광고 같아", "post-1", testNow), 10); err != nil || !recorded {
		t.Fatalf("first recording = %v (err %v)", recorded, err)
	}
	later := testNow.Add(time.Hour)
	if recorded, err := s.RecordCandidate(ctx, newCandidate("c2", "alice", "광고 같아", "post-2", later), 10); err != nil || !recorded {
		t.Fatalf("repeat = %v (err %v)", recorded, err)
	}
	rows, _, err := s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("a repeat created %d rows", len(rows))
	}
	if rows[0].Occurrences != 2 || !rows[0].LastSeenAt.Equal(later) || !rows[0].FirstSeenAt.Equal(testNow) {
		t.Fatalf("counted candidate = %+v", rows[0])
	}
	// The link names where the correction was FIRST seen; a repeat must not move it.
	if rows[0].PostSlug != "post-1" {
		t.Fatalf("post_slug moved to %q on a repeat", rows[0].PostSlug)
	}
	// The account boundary: the same text is a first sighting for another account.
	if recorded, err := s.RecordCandidate(ctx, newCandidate("c3", "bob", "광고 같아", "post-9", testNow), 10); err != nil || !recorded {
		t.Fatalf("bob's first recording = %v (err %v)", recorded, err)
	}
}

func TestRecordCandidateSkipsWhatIsAlreadyKnown(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if err := s.Insert(ctx, newGuideline("g1", "alice", "광고 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{}); err != nil {
		t.Fatal(err)
	}
	if recorded, err := s.RecordCandidate(ctx, newCandidate("c1", "alice", "광고 금지", "post-1", testNow), 10); err != nil || recorded {
		t.Fatalf("a saved guideline was recorded again: %v (err %v)", recorded, err)
	}
	// A dismissed row keeps suppressing the same instruction, and is never revived.
	if _, err := s.RecordCandidate(ctx, newCandidate("c2", "alice", "존댓말로", "post-1", testNow), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCandidateStatus(ctx, "alice", "c2", guideline.CandidateStatusDismissed); err != nil {
		t.Fatal(err)
	}
	if recorded, err := s.RecordCandidate(ctx, newCandidate("c3", "alice", "존댓말로", "post-2", testNow.Add(time.Hour)), 10); err != nil || recorded {
		t.Fatalf("a dismissed instruction was recorded again: %v (err %v)", recorded, err)
	}
	rows, _, err := s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("the dismissed candidate came back: %+v", rows)
	}
}

// The cap lives inside the recording transaction, so it is read from the same snapshot the
// insert commits against — two concurrent recordings cannot both pass a full queue.
func TestRecordCandidateStopsAtThePendingBound(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for _, text := range []string{"a", "b"} {
		if recorded, err := s.RecordCandidate(ctx, newCandidate("c"+text, "alice", text, "post-1", testNow), 2); err != nil || !recorded {
			t.Fatalf("recording %q = %v (err %v)", text, recorded, err)
		}
	}
	if recorded, err := s.RecordCandidate(ctx, newCandidate("cc", "alice", "c", "post-1", testNow), 2); err != nil || recorded {
		t.Fatalf("past the bound = %v (err %v)", recorded, err)
	}
	// Clearing one resumes recording. A repeat of an existing pending row still counts,
	// because it adds no row.
	if err := s.SetCandidateStatus(ctx, "alice", "ca", guideline.CandidateStatusApproved); err != nil {
		t.Fatal(err)
	}
	if recorded, err := s.RecordCandidate(ctx, newCandidate("cc", "alice", "c", "post-1", testNow), 2); err != nil || !recorded {
		t.Fatalf("after clearing one = %v (err %v)", recorded, err)
	}
}

// Review order: most-repeated first, then most recent. Exactly what the index serves.
func TestListPendingCandidatesReturnsReviewOrderAndTheHeldCount(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	for _, text := range []string{"once", "twice", "recent"} {
		if _, err := s.RecordCandidate(ctx, newCandidate("c-"+text, "alice", text, "post-1", testNow), 10); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.RecordCandidate(ctx, newCandidate("dup", "alice", "twice", "post-2", testNow.Add(time.Minute)), 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCandidate(ctx, newCandidate("dup2", "alice", "recent", "post-3", testNow.Add(2*time.Hour)), 10); err != nil {
		t.Fatal(err)
	}
	rows, held, err := s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if held != 3 {
		t.Fatalf("held = %d", held)
	}
	got := []string{rows[0].Text, rows[1].Text, rows[2].Text}
	// "recent" and "twice" both hold two occurrences, so the later last-seen wins.
	want := []string{"recent", "twice", "once"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("review order = %v, want %v", got, want)
		}
	}
}

// A deleted post leaves the text intact. There is no foreign key and no cascade: the row is
// still reviewable, only without its link.
func TestDropCandidatePostSlugKeepsTheText(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, err := s.RecordCandidate(ctx, newCandidate("c1", "alice", "광고 같아", "post-1", testNow), 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCandidate(ctx, newCandidate("c2", "alice", "존댓말로", "post-2", testNow), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.DropCandidatePostSlug(ctx, "alice", "post-1"); err != nil {
		t.Fatal(err)
	}
	rows, _, err := s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	links := map[string]string{}
	for _, row := range rows {
		links[row.Text] = row.PostSlug
	}
	if len(rows) != 2 || links["광고 같아"] != "" || links["존댓말로"] != "post-2" {
		t.Fatalf("after the post delete: %+v", links)
	}
}

// Approval rides the insert's own transaction: a create refused by the account cap must leave
// the candidate pending, and a create that lands must move it out of the review list.
func TestInsertApprovesTheCandidateInTheSameTransaction(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, err := s.RecordCandidate(ctx, newCandidate("c1", "alice", "광고 금지", "post-1", testNow), 10); err != nil {
		t.Fatal(err)
	}
	// Refused by the cap (zero allowed): nothing is approved.
	err := s.Insert(ctx, newGuideline("g1", "alice", "광고 금지", guideline.ScopeGlobal, testNow), 0, guideline.CandidateApproval{Text: "광고 금지"})
	var atCap *guideline.AccountCapError
	if !errors.As(err, &atCap) {
		t.Fatalf("capped insert err = %v", err)
	}
	rows, _, err := s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatal("a refused create moved the candidate out of the review list")
	}
	// By text: what an unedited approval and 지침으로 저장 both rely on.
	if err := s.Insert(ctx, newGuideline("g1", "alice", "광고 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{Text: "광고 금지"}); err != nil {
		t.Fatal(err)
	}
	rows, _, err = s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("the approved candidate stayed pending: %+v", rows)
	}
}

// By id: the path for a candidate the user EDITED first, whose text no longer matches.
func TestInsertApprovesAnEditedCandidateByID(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, err := s.RecordCandidate(ctx, newCandidate("c1", "alice", "여기 너무 광고 같아 특히 마지막", "post-1", testNow), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, newGuideline("g1", "alice", "광고 같은 문장 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{ID: "c1", Text: "광고 같은 문장 금지"}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("the edited candidate stayed pending: %+v", rows)
	}
	// A named id that is not the account's is refused rather than silently ignored.
	err = s.Insert(ctx, newGuideline("g2", "alice", "다른 지침", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{ID: "nope", Text: "다른 지침"})
	if !errors.Is(err, guideline.ErrCandidateNotFound) {
		t.Fatalf("unknown candidate id err = %v", err)
	}
}

// Only a pending row moves. A candidate the user already ruled on is terminal, so a stale tab
// can neither approve one twice nor dismiss one that was already approved.
func TestARuledOnCandidateCannotBeMovedAgain(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, err := s.RecordCandidate(ctx, newCandidate("c1", "alice", "광고 같아", "post-1", testNow), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCandidateStatus(ctx, "alice", "c1", guideline.CandidateStatusDismissed); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCandidateStatus(ctx, "alice", "c1", guideline.CandidateStatusApproved); !errors.Is(err, guideline.ErrCandidateNotFound) {
		t.Fatalf("a dismissed candidate was approved: %v", err)
	}
	// And the create rolls back rather than saving a guideline against a terminal candidate.
	err := s.Insert(ctx, newGuideline("g1", "alice", "광고 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{ID: "c1", Text: "광고 금지"})
	if !errors.Is(err, guideline.ErrCandidateNotFound) {
		t.Fatalf("approving a dismissed candidate err = %v", err)
	}
	if _, err := s.Get(ctx, "alice", "g1"); !errors.Is(err, guideline.ErrNotFound) {
		t.Fatalf("the refused create still saved a guideline: %v", err)
	}
}

// A guideline text EDIT is another way for a text to become a saved guideline, so it approves a
// same-text candidate exactly as a create does — otherwise that candidate could never be
// approved (its create would be a duplicate) and would sit in the review list forever.
func TestUpdateGuidelineTextApprovesASameTextCandidate(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, err := s.RecordCandidate(ctx, newCandidate("c1", "alice", "존댓말로", "post-1", testNow), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.Insert(ctx, newGuideline("g1", "alice", "광고 금지", guideline.ScopeGlobal, testNow), 10, guideline.CandidateApproval{}); err != nil {
		t.Fatal(err)
	}
	renamed := "존댓말로"
	if _, err := s.Update(ctx, "alice", "g1", guideline.Patch{Text: &renamed}, testNow); err != nil {
		t.Fatal(err)
	}
	rows, _, err := s.ListPendingCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("a renamed guideline left its candidate pending: %+v", rows)
	}
}

func TestSetCandidateStatusRefusesAForeignID(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	if _, err := s.RecordCandidate(ctx, newCandidate("c1", "bob", "광고 같아", "post-1", testNow), 10); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCandidateStatus(ctx, "alice", "c1", guideline.CandidateStatusDismissed); !errors.Is(err, guideline.ErrCandidateNotFound) {
		t.Fatalf("foreign id err = %v", err)
	}
}
