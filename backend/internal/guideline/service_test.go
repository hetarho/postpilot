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
	approvals     []CandidateApproval
	rows          map[string]Guideline
	patched       Patch
	texts         []string
	askedTemplate string
	askedAccount  string
	applicableErr error

	// The candidate half. The store owns the whole recording decision, so the fake records
	// what it was asked to record rather than re-deciding it.
	candidates    []Candidate
	candidateCap  int
	recorded      bool
	recordErr     error
	pending       []Candidate
	pendingHeld   int
	statusSet     map[string]CandidateStatus
	statusErr     error
	detachedSlugs []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]Guideline{}, statusSet: map[string]CandidateStatus{}, recorded: true}
}

func (f *fakeStore) Insert(_ context.Context, g Guideline, maxPerAccount int, approval CandidateApproval) error {
	f.insertedCap = maxPerAccount
	if f.insertErr != nil {
		return f.insertErr
	}
	f.approvals = append(f.approvals, approval)
	f.inserted = append(f.inserted, g)
	f.rows[g.ID] = g
	return nil
}

func (f *fakeStore) RecordCandidate(_ context.Context, c Candidate, maxPending int) (bool, error) {
	f.candidateCap = maxPending
	if f.recordErr != nil {
		return false, f.recordErr
	}
	f.candidates = append(f.candidates, c)
	return f.recorded, nil
}

func (f *fakeStore) ListPendingCandidates(_ context.Context, userID string) ([]Candidate, int, error) {
	out := make([]Candidate, 0, len(f.pending))
	for _, c := range f.pending {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	held := f.pendingHeld
	if held == 0 {
		held = len(out)
	}
	return out, held, nil
}

func (f *fakeStore) SetCandidateStatus(_ context.Context, _, id string, status CandidateStatus) error {
	if f.statusErr != nil {
		return f.statusErr
	}
	f.statusSet[id] = status
	return nil
}

func (f *fakeStore) DropCandidatePostSlug(_ context.Context, _, postSlug string) error {
	f.detachedSlugs = append(f.detachedSlugs, postSlug)
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
		g.TemplateIDs = patch.Scope.TemplateIDs
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

func (f *fakeStore) ApplicableTexts(_ context.Context, userID, templateID string) ([]string, error) {
	f.askedAccount, f.askedTemplate = userID, templateID
	return f.texts, f.applicableErr
}

type fakeDirectory struct {
	templates []TemplateRef
	err       error
	calls     int
}

func (f *fakeDirectory) Templates(_ context.Context, _ string) ([]TemplateRef, error) {
	f.calls++
	return f.templates, f.err
}

func newTestService(t *testing.T, directory *fakeDirectory) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	svc := NewService(store, Limits{TextMaxChars: 10, MaxPerAccount: 3}, 2)
	svc.now = func() time.Time { return testNow }
	ids := 0
	svc.newID = func() string { ids++; return "g" + string(rune('0'+ids)) }
	if directory != nil {
		svc.SetTemplateDirectory(directory)
	}
	return svc, store
}

// A1: text is trimmed, non-empty and bounded; the cap reaches the store as the store's own
// business, because only the store can count and insert atomically.
func TestCreateTrimsBoundsAndPassesTheCap(t *testing.T) {
	svc, store := newTestService(t, nil)
	created, err := svc.Create(context.Background(), "alice", "  CCTV 언급 금지  ", ScopeGlobal, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if created.Text != "CCTV 언급 금지" {
		t.Fatalf("text = %q, want the trimmed form", created.Text)
	}
	if store.insertedCap != 3 {
		t.Fatalf("cap handed to the store = %d, want 3", store.insertedCap)
	}
	if _, err := svc.Create(context.Background(), "alice", "   ", ScopeGlobal, nil, ""); !errors.Is(err, ErrInvalidText) {
		t.Fatalf("blank text err = %v", err)
	}
	var tooLong *TextTooLongError
	_, err = svc.Create(context.Background(), "alice", strings.Repeat("가", 11), ScopeGlobal, nil, "")
	if !errors.As(err, &tooLong) || tooLong.Chars != 11 || tooLong.Max != 10 {
		t.Fatalf("over-limit err = %v", err)
	}
	// The limit counts Unicode scalar values: exactly ten Hangul syllables must fit.
	if _, err := svc.Create(context.Background(), "alice", strings.Repeat("나", 10), ScopeGlobal, nil, ""); err != nil {
		t.Fatalf("exactly-at-limit refused: %v", err)
	}
}

// A2: the two contradictory scope shapes are refused rather than repaired.
func TestCreateRefusesContradictoryScopeShapes(t *testing.T) {
	directory := &fakeDirectory{templates: []TemplateRef{{ID: "p1", Name: "리뷰"}}}
	svc, _ := newTestService(t, directory)
	if _, err := svc.Create(context.Background(), "alice", "a", ScopeGlobal, []string{"p1"}, ""); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("global with ids err = %v", err)
	}
	if _, err := svc.Create(context.Background(), "alice", "a", ScopeTemplates, nil, ""); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("templates with no ids err = %v", err)
	}
	if _, err := svc.Create(context.Background(), "alice", "a", Scope("voice"), nil, ""); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("unknown scope err = %v", err)
	}
}

// A2: an unknown or foreign template id is not-found and nothing is applied; duplicates in one
// request collapse to one link.
func TestCreateValidatesScopedTemplatesAndCollapsesDuplicates(t *testing.T) {
	directory := &fakeDirectory{templates: []TemplateRef{{ID: "p1", Name: "리뷰"}, {ID: "p2", Name: "후기"}}}
	svc, store := newTestService(t, directory)

	if _, err := svc.Create(context.Background(), "alice", "a", ScopeTemplates, []string{"p1", "nope"}, ""); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("unknown template err = %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatal("a refused scope still wrote a row")
	}
	created, err := svc.Create(context.Background(), "alice", "a", ScopeTemplates, []string{"p2", "p1", "p2"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := created.TemplateIDs; len(got) != 2 || got[0] != "p2" || got[1] != "p1" {
		t.Fatalf("template ids = %v, want the request order with the duplicate collapsed", got)
	}
	// The projection is by name, not by request order, so chips read predictably.
	if len(created.Templates) != 2 || created.Templates[0].Name != "리뷰" || created.Templates[1].Name != "후기" {
		t.Fatalf("projected templates = %+v", created.Templates)
	}
}

// A3: presence is the edit unit. A text-only patch must carry no scope at all, so nothing can
// overwrite a scope saved from elsewhere.
func TestUpdateCarriesOnlyWhatTheRequestSent(t *testing.T) {
	directory := &fakeDirectory{templates: []TemplateRef{{ID: "p1", Name: "리뷰"}}}
	svc, store := newTestService(t, directory)
	store.rows["g1"] = Guideline{ID: "g1", UserID: "alice", Text: "old", Scope: ScopeTemplates, TemplateIDs: []string{"p1"}}

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
	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{Scope: &ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"gone"}}}); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("foreign template in a scope patch err = %v", err)
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

// A5: a post with a template asks for that template; a post without one asks for no template, and
// the two must not be spelled the same way.
func TestForPromptDistinguishesNoTemplateFromATemplate(t *testing.T) {
	svc, store := newTestService(t, nil)
	store.texts = []string{"전역 1", "템플릿 1"}

	texts, err := svc.ForPrompt(context.Background(), "alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if store.askedTemplate != "" || store.askedAccount != "alice" {
		t.Fatalf("no-template resolution asked for %q / %q", store.askedAccount, store.askedTemplate)
	}
	if len(texts) != 2 {
		t.Fatalf("texts = %v", texts)
	}
	id := "  p1  "
	if _, err := svc.ForPrompt(context.Background(), "alice", &id); err != nil {
		t.Fatal(err)
	}
	if store.askedTemplate != "p1" {
		t.Fatalf("template asked = %q, want the trimmed id", store.askedTemplate)
	}
}

// A2: names come from the directory. A scoped id the directory no longer knows is dropped
// rather than shown as a blank chip — that is the orphaned-scope state.
func TestProjectionDropsUnknownTemplateIdsAndReadsTheDirectoryOnce(t *testing.T) {
	directory := &fakeDirectory{templates: []TemplateRef{{ID: "p1", Name: "리뷰"}}}
	svc, store := newTestService(t, directory)
	store.rows["g1"] = Guideline{ID: "g1", UserID: "alice", Scope: ScopeTemplates, TemplateIDs: []string{"p1", "deleted"}}
	store.rows["g2"] = Guideline{ID: "g2", UserID: "alice", Scope: ScopeTemplates, TemplateIDs: []string{"p1"}}

	listed, err := svc.List(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if directory.calls != 1 {
		t.Fatalf("directory read %d times for one list", directory.calls)
	}
	for _, g := range listed {
		for _, ref := range g.Templates {
			if ref.ID != "p1" {
				t.Fatalf("%s projected an unknown template %+v", g.ID, ref)
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
	if _, err := svc.Create(context.Background(), "alice", "a", ScopeTemplates, []string{"p1"}, ""); err == nil {
		t.Fatal("a scoped create was accepted with no template directory")
	}
}

func TestNewServiceRejectsNonPositiveLimits(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a zero limit was accepted")
		}
	}()
	NewService(newFakeStore(), Limits{TextMaxChars: 0, MaxPerAccount: 1}, 2)
}

// --- candidates (change 26) ---

func TestRecordCandidateStoresTheInstructionVerbatimAtTheRevisionBound(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	long := strings.Repeat("가", 400)
	if err := svc.RecordCandidate(context.Background(), "alice", "post-1", "  "+long+"  "); err != nil {
		t.Fatal(err)
	}
	if len(store.candidates) != 1 {
		t.Fatalf("recorded %d candidates", len(store.candidates))
	}
	got := store.candidates[0]
	// 400 characters is past the guideline bound this service was built with (10) and
	// inside the candidate bound: the split is what keeps a long correction from being lost.
	if got.Text != long {
		t.Fatal("the candidate text is not the trimmed instruction verbatim")
	}
	if got.PostSlug != "post-1" || got.Status != CandidateStatusPending || got.Occurrences != 1 {
		t.Fatalf("candidate = %+v", got)
	}
	if !got.FirstSeenAt.Equal(got.LastSeenAt) {
		t.Fatal("a first sighting must carry the same first- and last-seen stamp")
	}
	if store.candidateCap != 2 {
		t.Fatalf("the pending bound handed to the store = %d, want the configured 2", store.candidateCap)
	}
}

func TestRecordCandidateRefusesABlankInstruction(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	if err := svc.RecordCandidate(context.Background(), "alice", "post-1", "   "); !errors.Is(err, ErrCandidateTextInvalid) {
		t.Fatalf("blank err = %v", err)
	}
	if len(store.candidates) != 0 {
		t.Fatal("a blank instruction reached the store")
	}
}

// A skip is an ordinary outcome — the text is already known, or the queue is full — and the
// revision that triggered it must not see an error.
func TestRecordCandidateTreatsASkipAsSuccess(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	store.recorded = false
	if err := svc.RecordCandidate(context.Background(), "alice", "post-1", "광고 같아"); err != nil {
		t.Fatalf("a skip surfaced as an error: %v", err)
	}
}

func TestListCandidatesReportsAFullQueue(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	store.pending = []Candidate{
		{ID: "c1", UserID: "alice", Text: "a", Status: CandidateStatusPending, Occurrences: 3},
	}
	candidates, full, err := svc.ListCandidates(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || full {
		t.Fatalf("candidates=%d full=%v", len(candidates), full)
	}
	// The configured bound is 2, so two pending rows is a full queue.
	store.pendingHeld = 2
	if _, full, err = svc.ListCandidates(context.Background(), "alice"); err != nil || !full {
		t.Fatalf("queue_full at the bound = %v (err %v)", full, err)
	}
}

func TestDismissCandidateMarksRatherThanDeletes(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	if err := svc.DismissCandidate(context.Background(), "alice", "c1"); err != nil {
		t.Fatal(err)
	}
	if store.statusSet["c1"] != CandidateStatusDismissed {
		t.Fatalf("status = %q", store.statusSet["c1"])
	}
	if err := svc.DismissCandidate(context.Background(), "alice", "  "); !errors.Is(err, ErrCandidateNotFound) {
		t.Fatalf("blank id err = %v", err)
	}
}

// The create always carries the saved text as an approval, and carries the id only when the
// user edited the candidate first. Both halves are what keep a saved instruction from
// reappearing as a pending candidate.
func TestCreateCarriesTheCandidateApproval(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	if _, err := svc.Create(context.Background(), "alice", "  광고 금지  ", ScopeGlobal, nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := store.approvals[0]; got.Text != "광고 금지" || got.ID != "" {
		t.Fatalf("approval = %+v", got)
	}
	if _, err := svc.Create(context.Background(), "alice", "짧게", ScopeGlobal, nil, "  c9  "); err != nil {
		t.Fatal(err)
	}
	if got := store.approvals[1]; got.ID != "c9" || got.Text != "짧게" {
		t.Fatalf("edited approval = %+v", got)
	}
}

// A refused create must approve nothing: the candidate has to stay pending so the user can
// shorten it and try again (change 26's bound split).
func TestCreateRefusedByTheTextBoundApprovesNothing(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	_, err := svc.Create(context.Background(), "alice", strings.Repeat("가", 11), ScopeGlobal, nil, "c1")
	var tooLong *TextTooLongError
	if !errors.As(err, &tooLong) {
		t.Fatalf("over-bound create err = %v", err)
	}
	if len(store.approvals) != 0 {
		t.Fatal("a refused create still carried an approval to the store")
	}
}

func TestDetachCandidatePostDropsOnlyTheLink(t *testing.T) {
	svc, store := newTestService(t, &fakeDirectory{})
	if err := svc.DetachCandidatePost(context.Background(), "alice", "post-1"); err != nil {
		t.Fatal(err)
	}
	if len(store.detachedSlugs) != 1 || store.detachedSlugs[0] != "post-1" {
		t.Fatalf("detached = %v", store.detachedSlugs)
	}
	if err := svc.DetachCandidatePost(context.Background(), "alice", "  "); err != nil {
		t.Fatal(err)
	}
	if len(store.detachedSlugs) != 1 {
		t.Fatal("a blank slug reached the store")
	}
}

func TestNewServiceRefusesANonPositivePendingBound(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a non-positive pending bound was accepted")
		}
	}()
	NewService(newFakeStore(), Limits{TextMaxChars: 10, MaxPerAccount: 1}, 0)
}
