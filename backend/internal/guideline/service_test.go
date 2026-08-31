package guideline

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// fakeStore records what the service asked for. The domain rules under test are validation,
// dedup shape, scope shape and ordering hand-off — none of which need real SQL.
type fakeStore struct {
	inserted      []Guideline
	insertedCap   int
	insertErr     error
	rows          map[string]Guideline
	patched       Patch
	texts         []string
	askedPurpose  string
	askedAccount  string
	applicableErr error
}

func newFakeStore() *fakeStore { return &fakeStore{rows: map[string]Guideline{}} }

func (f *fakeStore) Insert(_ context.Context, g Guideline, maxPerAccount int) error {
	f.insertedCap = maxPerAccount
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, g)
	f.rows[g.ID] = g
	return nil
}

func (f *fakeStore) List(_ context.Context, userID string) ([]Guideline, error) {
	out := make([]Guideline, 0, len(f.rows))
	for _, g := range f.rows {
		if g.UserID == userID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (f *fakeStore) Get(_ context.Context, userID, id string) (Guideline, error) {
	g, ok := f.rows[id]
	if !ok || g.UserID != userID {
		return Guideline{}, ErrNotFound
	}
	return g, nil
}

func (f *fakeStore) Update(_ context.Context, userID, id string, patch Patch, _ time.Time) (Guideline, error) {
	f.patched = patch
	g, ok := f.rows[id]
	if !ok || g.UserID != userID {
		return Guideline{}, ErrNotFound
	}
	if patch.Text != nil {
		g.Text = *patch.Text
	}
	if patch.Scope != nil {
		g.Scope = patch.Scope.Scope
		g.PurposeIDs = patch.Scope.PurposeIDs
	}
	f.rows[id] = g
	return g, nil
}

func (f *fakeStore) Delete(_ context.Context, userID, id string) error {
	g, ok := f.rows[id]
	if !ok || g.UserID != userID {
		return ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func (f *fakeStore) ApplicableTexts(_ context.Context, userID, purposeID string) ([]string, error) {
	f.askedAccount, f.askedPurpose = userID, purposeID
	return f.texts, f.applicableErr
}

type fakeDirectory struct {
	purposes []PurposeRef
	err      error
	calls    int
}

func (f *fakeDirectory) Purposes(_ context.Context, _ string) ([]PurposeRef, error) {
	f.calls++
	return f.purposes, f.err
}

func newTestService(t *testing.T, directory *fakeDirectory) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	svc := NewService(store, Limits{TextMaxChars: 10, MaxPerAccount: 3})
	svc.now = func() time.Time { return testNow }
	ids := 0
	svc.newID = func() string { ids++; return "g" + string(rune('0'+ids)) }
	if directory != nil {
		svc.SetPurposeDirectory(directory)
	}
	return svc, store
}

// A1: text is trimmed, non-empty and bounded; the cap reaches the store as the store's own
// business, because only the store can count and insert atomically.
func TestCreateTrimsBoundsAndPassesTheCap(t *testing.T) {
	svc, store := newTestService(t, nil)
	created, err := svc.Create(context.Background(), "alice", "  CCTV 언급 금지  ", ScopeGlobal, nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.Text != "CCTV 언급 금지" {
		t.Fatalf("text = %q, want the trimmed form", created.Text)
	}
	if store.insertedCap != 3 {
		t.Fatalf("cap handed to the store = %d, want 3", store.insertedCap)
	}
	if _, err := svc.Create(context.Background(), "alice", "   ", ScopeGlobal, nil); !errors.Is(err, ErrInvalidText) {
		t.Fatalf("blank text err = %v", err)
	}
	var tooLong *TextTooLongError
	_, err = svc.Create(context.Background(), "alice", strings.Repeat("가", 11), ScopeGlobal, nil)
	if !errors.As(err, &tooLong) || tooLong.Chars != 11 || tooLong.Max != 10 {
		t.Fatalf("over-limit err = %v", err)
	}
	// The limit counts Unicode scalar values: exactly ten Hangul syllables must fit.
	if _, err := svc.Create(context.Background(), "alice", strings.Repeat("나", 10), ScopeGlobal, nil); err != nil {
		t.Fatalf("exactly-at-limit refused: %v", err)
	}
}

// A2: the two contradictory scope shapes are refused rather than repaired.
func TestCreateRefusesContradictoryScopeShapes(t *testing.T) {
	directory := &fakeDirectory{purposes: []PurposeRef{{ID: "p1", Name: "리뷰"}}}
	svc, _ := newTestService(t, directory)
	if _, err := svc.Create(context.Background(), "alice", "a", ScopeGlobal, []string{"p1"}); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("global with ids err = %v", err)
	}
	if _, err := svc.Create(context.Background(), "alice", "a", ScopePurposes, nil); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("purposes with no ids err = %v", err)
	}
	if _, err := svc.Create(context.Background(), "alice", "a", Scope("voice"), nil); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("unknown scope err = %v", err)
	}
}

// A2: an unknown or foreign purpose id is not-found and nothing is applied; duplicates in one
// request collapse to one link.
func TestCreateValidatesScopedPurposesAndCollapsesDuplicates(t *testing.T) {
	directory := &fakeDirectory{purposes: []PurposeRef{{ID: "p1", Name: "리뷰"}, {ID: "p2", Name: "후기"}}}
	svc, store := newTestService(t, directory)

	if _, err := svc.Create(context.Background(), "alice", "a", ScopePurposes, []string{"p1", "nope"}); !errors.Is(err, ErrPurposeNotFound) {
		t.Fatalf("unknown purpose err = %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatal("a refused scope still wrote a row")
	}
	created, err := svc.Create(context.Background(), "alice", "a", ScopePurposes, []string{"p2", "p1", "p2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := created.PurposeIDs; len(got) != 2 || got[0] != "p2" || got[1] != "p1" {
		t.Fatalf("purpose ids = %v, want the request order with the duplicate collapsed", got)
	}
	// The projection is by name, not by request order, so chips read predictably.
	if len(created.Purposes) != 2 || created.Purposes[0].Name != "리뷰" || created.Purposes[1].Name != "후기" {
		t.Fatalf("projected purposes = %+v", created.Purposes)
	}
}

// A3: presence is the edit unit. A text-only patch must carry no scope at all, so nothing can
// overwrite a scope saved from elsewhere.
func TestUpdateCarriesOnlyWhatTheRequestSent(t *testing.T) {
	directory := &fakeDirectory{purposes: []PurposeRef{{ID: "p1", Name: "리뷰"}}}
	svc, store := newTestService(t, directory)
	store.rows["g1"] = Guideline{ID: "g1", UserID: "alice", Text: "old", Scope: ScopePurposes, PurposeIDs: []string{"p1"}}

	text := "  new  "
	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{Text: &text}); err != nil {
		t.Fatal(err)
	}
	if store.patched.Scope != nil {
		t.Fatal("a text-only edit named the scope")
	}
	if *store.patched.Text != "new" {
		t.Fatalf("text patch = %q, want trimmed", *store.patched.Text)
	}

	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{Scope: &ScopePatch{Scope: ScopeGlobal}}); err != nil {
		t.Fatal(err)
	}
	if store.patched.Text != nil {
		t.Fatal("a scope-only edit named the text")
	}
	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{Scope: &ScopePatch{Scope: ScopePurposes, PurposeIDs: []string{"gone"}}}); !errors.Is(err, ErrPurposeNotFound) {
		t.Fatalf("foreign purpose in a scope patch err = %v", err)
	}
}

// A1: an unknown id and a foreign id must be indistinguishable.
func TestForeignIdsReadAsUnknown(t *testing.T) {
	svc, store := newTestService(t, nil)
	store.rows["g1"] = Guideline{ID: "g1", UserID: "bob", Text: "bob's", Scope: ScopeGlobal}

	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign update err = %v", err)
	}
	if err := svc.Delete(context.Background(), "alice", "g1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign delete err = %v", err)
	}
	if err := svc.Delete(context.Background(), "alice", "   "); !errors.Is(err, ErrNotFound) {
		t.Fatalf("blank id err = %v", err)
	}
}

// A5: a post with a purpose asks for that purpose; a post without one asks for no purpose, and
// the two must not be spelled the same way.
func TestForPromptDistinguishesNoPurposeFromAPurpose(t *testing.T) {
	svc, store := newTestService(t, nil)
	store.texts = []string{"전역 1", "용도 1"}

	texts, err := svc.ForPrompt(context.Background(), "alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if store.askedPurpose != "" || store.askedAccount != "alice" {
		t.Fatalf("no-purpose resolution asked for %q / %q", store.askedAccount, store.askedPurpose)
	}
	if len(texts) != 2 {
		t.Fatalf("texts = %v", texts)
	}
	id := "  p1  "
	if _, err := svc.ForPrompt(context.Background(), "alice", &id); err != nil {
		t.Fatal(err)
	}
	if store.askedPurpose != "p1" {
		t.Fatalf("purpose asked = %q, want the trimmed id", store.askedPurpose)
	}
}

// A2: names come from the directory. A scoped id the directory no longer knows is dropped
// rather than shown as a blank chip — that is the orphaned-scope state.
func TestProjectionDropsUnknownPurposeIdsAndReadsTheDirectoryOnce(t *testing.T) {
	directory := &fakeDirectory{purposes: []PurposeRef{{ID: "p1", Name: "리뷰"}}}
	svc, store := newTestService(t, directory)
	store.rows["g1"] = Guideline{ID: "g1", UserID: "alice", Scope: ScopePurposes, PurposeIDs: []string{"p1", "deleted"}}
	store.rows["g2"] = Guideline{ID: "g2", UserID: "alice", Scope: ScopePurposes, PurposeIDs: []string{"p1"}}

	listed, err := svc.List(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if directory.calls != 1 {
		t.Fatalf("directory read %d times for one list", directory.calls)
	}
	for _, g := range listed {
		for _, ref := range g.Purposes {
			if ref.ID != "p1" {
				t.Fatalf("%s projected an unknown purpose %+v", g.ID, ref)
			}
		}
	}
}

// A global-only account never needs the directory, so listing must not require it to be wired.
func TestListSkipsTheDirectoryWhenNothingIsScoped(t *testing.T) {
	svc, store := newTestService(t, nil)
	store.rows["g1"] = Guideline{ID: "g1", UserID: "alice", Scope: ScopeGlobal}
	if _, err := svc.List(context.Background(), "alice"); err != nil {
		t.Fatal(err)
	}
}

// A scope write with no directory wired must fail closed rather than save an unvalidated set.
func TestScopedWriteWithoutADirectoryFails(t *testing.T) {
	svc, _ := newTestService(t, nil)
	if _, err := svc.Create(context.Background(), "alice", "a", ScopePurposes, []string{"p1"}); err == nil {
		t.Fatal("a scoped create was accepted with no purpose directory")
	}
}

func TestNewServiceRejectsNonPositiveLimits(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a zero limit was accepted")
		}
	}()
	NewService(newFakeStore(), Limits{TextMaxChars: 0, MaxPerAccount: 1})
}
