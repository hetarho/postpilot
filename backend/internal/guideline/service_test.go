package guideline

import (
	"context"
	"errors"
	"reflect"
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
	askedField    string
	askedAccount  string
	applicableErr error

	// The preset half, kept in memory: the presence rules are the store's, so the fake only
	// has to hold what it was last given. presetReads counts how often it was read, and
	// presetWrites how often written.
	preset       Preset
	presetReads  int
	presetWrites []PresetPatch

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
		g.Fields = patch.Scope.Fields
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

func (f *fakeStore) ApplicableTexts(_ context.Context, userID, templateID, field string) ([]string, error) {
	f.askedAccount, f.askedTemplate, f.askedField = userID, templateID, field
	return f.texts, f.applicableErr
}

func (f *fakeStore) Preset(context.Context, string) (Preset, error) {
	f.presetReads++
	return f.preset, nil
}

func (f *fakeStore) UpdatePreset(_ context.Context, _ string, patch PresetPatch, _ time.Time) (Preset, error) {
	f.presetWrites = append(f.presetWrites, patch)
	if patch.Enabled != nil {
		f.preset.Enabled = *patch.Enabled
	}
	if patch.Fields != nil {
		f.preset.Fields = append([]string(nil), *patch.Fields...)
	}
	return f.preset, nil
}

// fakeFields knows the 분야 it lists.
type fakeFields map[string]bool

func (f fakeFields) Known(id string) bool { return f[id] }

var testFields = fakeFields{"cafe": true, "restaurant": true, "pets": true}

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
	svc := NewService(store, testFields, Limits{TextMaxChars: 10, MaxPerAccount: 3}, 2)
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
	created, err := svc.Create(context.Background(), "alice", "  CCTV 언급 금지  ", ScopePatch{Scope: ScopeGlobal}, "")
	if err != nil {
		t.Fatal(err)
	}
	if created.Text != "CCTV 언급 금지" {
		t.Fatalf("text = %q, want the trimmed form", created.Text)
	}
	if store.insertedCap != 3 {
		t.Fatalf("cap handed to the store = %d, want 3", store.insertedCap)
	}
	if _, err := svc.Create(context.Background(), "alice", "   ", ScopePatch{Scope: ScopeGlobal}, ""); !errors.Is(err, ErrInvalidText) {
		t.Fatalf("blank text err = %v", err)
	}
	var tooLong *TextTooLongError
	_, err = svc.Create(context.Background(), "alice", strings.Repeat("가", 11), ScopePatch{Scope: ScopeGlobal}, "")
	if !errors.As(err, &tooLong) || tooLong.Chars != 11 || tooLong.Max != 10 {
		t.Fatalf("over-limit err = %v", err)
	}
	// The limit counts Unicode scalar values: exactly ten Hangul syllables must fit.
	if _, err := svc.Create(context.Background(), "alice", strings.Repeat("나", 10), ScopePatch{Scope: ScopeGlobal}, ""); err != nil {
		t.Fatalf("exactly-at-limit refused: %v", err)
	}
}

// A2: every contradictory shape of the three kinds is refused rather than repaired, and nothing
// is written.
func TestCreateRefusesContradictoryScopeShapes(t *testing.T) {
	directory := &fakeDirectory{templates: []TemplateRef{{ID: "p1", Name: "리뷰"}}}
	svc, store := newTestService(t, directory)
	for name, scope := range map[string]ScopePatch{
		"global with template ids": {Scope: ScopeGlobal, TemplateIDs: []string{"p1"}},
		"global with 분야":           {Scope: ScopeGlobal, Fields: []string{"cafe"}},
		"templates with no ids":    {Scope: ScopeTemplates},
		"templates with 분야":        {Scope: ScopeTemplates, TemplateIDs: []string{"p1"}, Fields: []string{"cafe"}},
		"fields with no 분야":        {Scope: ScopeFields},
		"fields with template ids": {Scope: ScopeFields, TemplateIDs: []string{"p1"}, Fields: []string{"cafe"}},
		"an unknown kind":          {Scope: Scope("voice")},
		"no kind at all":           {},
	} {
		if _, err := svc.Create(context.Background(), "alice", "a", scope, ""); !errors.Is(err, ErrScopeShape) {
			t.Errorf("%s: err = %v, want the scope shape refusal", name, err)
		}
	}
	if len(store.inserted) != 0 {
		t.Fatalf("a refused shape wrote %d rows", len(store.inserted))
	}
}

// GUIDE-5: a fields scope names at least one listed 분야, collapsed to one link each in first-seen
// order; an unlisted or blank one is not-found and nothing is written.
func TestCreateWithFieldsCollapsesAndValidates(t *testing.T) {
	svc, store := newTestService(t, nil)
	created, err := svc.Create(context.Background(), "alice", "a", ScopePatch{Scope: ScopeFields, Fields: []string{" pets ", "cafe", "pets"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if created.Scope != ScopeFields || !reflect.DeepEqual(created.Fields, []string{"pets", "cafe"}) || created.TemplateIDs != nil {
		t.Fatalf("created = %+v", created)
	}
	for name, fields := range map[string][]string{"an unlisted 분야": {"cafe", "moon"}, "a blank 분야": {"cafe", "  "}} {
		if _, err := svc.Create(context.Background(), "alice", "b", ScopePatch{Scope: ScopeFields, Fields: fields}, ""); !errors.Is(err, ErrFieldNotFound) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if len(store.inserted) != 1 {
		t.Fatalf("a refused 분야 wrote a row: %d inserted", len(store.inserted))
	}
}

// A rescope between templates and fields is ONE normalized patch: the kind with its own set and
// the other kind's set nil, so the store's replacement leaves no link of the kind it left.
func TestRescopeBetweenTemplatesAndFieldsIsOneNormalizedPatch(t *testing.T) {
	directory := &fakeDirectory{templates: []TemplateRef{{ID: "p1", Name: "리뷰"}}}
	svc, store := newTestService(t, directory)
	store.rows["g1"] = Guideline{ID: "g1", UserID: "alice", Text: "a", Scope: ScopeTemplates, TemplateIDs: []string{"p1"}}

	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{Scope: &ScopePatch{Scope: ScopeFields, Fields: []string{"cafe", "cafe"}}}); err != nil {
		t.Fatal(err)
	}
	if want := (ScopePatch{Scope: ScopeFields, Fields: []string{"cafe"}}); !reflect.DeepEqual(*store.patched.Scope, want) {
		t.Fatalf("to fields: patch = %+v, want %+v", *store.patched.Scope, want)
	}
	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{Scope: &ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"p1"}}}); err != nil {
		t.Fatal(err)
	}
	if want := (ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"p1"}}); !reflect.DeepEqual(*store.patched.Scope, want) {
		t.Fatalf("to templates: patch = %+v, want %+v", *store.patched.Scope, want)
	}
	// A text-only edit leaves the 분야 set alone: no scope reaches the store at all.
	text := "b"
	if _, err := svc.Update(context.Background(), "alice", "g1", Patch{Text: &text}); err != nil || store.patched.Scope != nil {
		t.Fatalf("a text edit carried %+v (%v)", store.patched.Scope, err)
	}
}

// GUIDE-38, GUIDE-39: the preset is a presence patch. Every 분야 is proved before anything is
// written, an empty set is legal, and an empty patch writes nothing.
func TestUpdatePresetIsAPresencePatch(t *testing.T) {
	svc, store := newTestService(t, nil)
	ctx := context.Background()
	store.preset = Preset{Enabled: true, Fields: []string{"cafe"}}

	if got, err := svc.UpdatePreset(ctx, "alice", PresetPatch{}); err != nil || !reflect.DeepEqual(got, store.preset) || len(store.presetWrites) != 0 {
		t.Fatalf("an empty patch = %+v (%v), %d writes", got, err, len(store.presetWrites))
	}
	off := false
	if _, err := svc.UpdatePreset(ctx, "alice", PresetPatch{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if write := store.presetWrites[0]; write.Fields != nil || *write.Enabled {
		t.Fatalf("the switch alone wrote %+v", write)
	}
	fields := []string{"pets", " cafe ", "pets"}
	if _, err := svc.UpdatePreset(ctx, "alice", PresetPatch{Fields: &fields}); err != nil {
		t.Fatal(err)
	}
	if write := store.presetWrites[1]; write.Enabled != nil || !reflect.DeepEqual(*write.Fields, []string{"pets", "cafe"}) {
		t.Fatalf("the 분야 alone wrote %+v", write)
	}
	for name, bad := range map[string][]string{"an unlisted 분야": {"cafe", "moon"}, "a blank 분야": {" "}} {
		on := true
		if _, err := svc.UpdatePreset(ctx, "alice", PresetPatch{Enabled: &on, Fields: &bad}); !errors.Is(err, ErrFieldNotFound) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if len(store.presetWrites) != 2 {
		t.Fatalf("a refused preset wrote: %d writes", len(store.presetWrites))
	}
	empty := []string{}
	if got, err := svc.UpdatePreset(ctx, "alice", PresetPatch{Fields: &empty}); err != nil || len(got.Fields) != 0 {
		t.Fatalf("the empty set = %+v (%v)", got, err)
	}
}

// GUIDE-17, GUIDE-29, GEN-57: the owner texts come back as the store resolved them, and the
// preset's line comes back apart — set only for a generation of a post whose 분야 the preset is
// switched on for. Whether a write keeps it is generation's decision (GUIDE-40).
func TestForPromptReturnsThePresetLineApart(t *testing.T) {
	cafe, pets := "cafe", "pets"
	for name, tc := range map[string]struct {
		preset      Preset
		field       *string
		forRevision bool
		wantPreset  string
		reads       int
	}{
		"the preset off":       {Preset{Fields: []string{"cafe"}}, &cafe, false, "", 1},
		"on with no 분야":        {Preset{Enabled: true}, &cafe, false, "", 1},
		"on for another 분야":    {Preset{Enabled: true, Fields: []string{"pets"}}, &cafe, false, "", 1},
		"on for the post's 분야": {Preset{Enabled: true, Fields: []string{"pets", "cafe"}}, &cafe, false, PresetText, 1},
		"a revision":           {Preset{Enabled: true, Fields: []string{"cafe"}}, &cafe, true, "", 0},
		"a post with no 분야":    {Preset{Enabled: true, Fields: []string{"cafe"}}, nil, false, "", 0},
		"a blank 분야 is none":   {Preset{Enabled: true, Fields: []string{"cafe"}}, new(string), false, "", 0},
	} {
		t.Run(name, func(t *testing.T) {
			svc, store := newTestService(t, nil)
			store.texts = []string{"전역", "분야"}
			store.preset = tc.preset
			texts, err := svc.ForPrompt(context.Background(), "alice", nil, tc.field, tc.forRevision)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(texts.Owner, []string{"전역", "분야"}) || texts.Preset != tc.wantPreset {
				t.Fatalf("texts = %+v, want owner [전역 분야] and preset %q", texts, tc.wantPreset)
			}
			if store.presetReads != tc.reads {
				t.Fatalf("the preset was read %d times, want %d", store.presetReads, tc.reads)
			}
		})
	}
	// The 분야 asked of the store is the post's, trimmed.
	svc, store := newTestService(t, nil)
	spaced := "  pets  "
	if _, err := svc.ForPrompt(context.Background(), "alice", nil, &spaced, false); err != nil || store.askedField != pets {
		t.Fatalf("asked the store for %q (%v)", store.askedField, err)
	}
}

// GUIDE-39: the preset is not a guideline row, so it spends no cap and takes no text: an account
// at the cap with the preset on is refused at exactly the same count, and an owner guideline
// holding the preset's own text is an ordinary guideline.
func TestThePresetSpendsNoCapAndNoTextUniqueness(t *testing.T) {
	store := newFakeStore()
	store.preset = Preset{Enabled: true, Fields: []string{"cafe"}}
	svc := NewService(store, testFields, Limits{TextMaxChars: 300, MaxPerAccount: 3}, 2)
	created, err := svc.Create(context.Background(), "alice", PresetText, ScopePatch{Scope: ScopeFields, Fields: []string{"cafe"}}, "")
	if err != nil || created.Text != PresetText {
		t.Fatalf("an owner line equal to the preset: %+v (%v)", created, err)
	}
	if store.insertedCap != 3 {
		t.Fatalf("the cap reached the store as %d with the preset on, want 3", store.insertedCap)
	}
	if store.presetReads != 0 || len(store.presetWrites) != 0 {
		t.Fatalf("a create touched the preset: %d reads, %d writes", store.presetReads, len(store.presetWrites))
	}
}

// A2: an unknown or foreign template id is not-found and nothing is applied; duplicates in one
// request collapse to one link.
func TestCreateValidatesScopedTemplatesAndCollapsesDuplicates(t *testing.T) {
	directory := &fakeDirectory{templates: []TemplateRef{{ID: "p1", Name: "리뷰"}, {ID: "p2", Name: "후기"}}}
	svc, store := newTestService(t, directory)

	if _, err := svc.Create(context.Background(), "alice", "a", ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"p1", "nope"}}, ""); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("unknown template err = %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatal("a refused scope still wrote a row")
	}
	created, err := svc.Create(context.Background(), "alice", "a", ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"p2", "p1", "p2"}}, "")
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

	texts, err := svc.ForPrompt(context.Background(), "alice", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if store.askedTemplate != "" || store.askedAccount != "alice" {
		t.Fatalf("no-template resolution asked for %q / %q", store.askedAccount, store.askedTemplate)
	}
	// A post with no 분야 asks for no 분야 group.
	if store.askedField != "" {
		t.Fatalf("resolution asked for the 분야 %q", store.askedField)
	}
	if len(texts.Owner) != 2 || texts.Preset != "" {
		t.Fatalf("texts = %+v", texts)
	}
	id := "  p1  "
	if _, err := svc.ForPrompt(context.Background(), "alice", &id, nil, false); err != nil {
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
	if _, err := svc.Create(context.Background(), "alice", "a", ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"p1"}}, ""); err == nil {
		t.Fatal("a scoped create was accepted with no template directory")
	}
}

func TestNewServiceRejectsNonPositiveLimits(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a zero limit was accepted")
		}
	}()
	NewService(newFakeStore(), testFields, Limits{TextMaxChars: 0, MaxPerAccount: 1}, 2)
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
	if _, err := svc.Create(context.Background(), "alice", "  광고 금지  ", ScopePatch{Scope: ScopeGlobal}, ""); err != nil {
		t.Fatal(err)
	}
	if got := store.approvals[0]; got.Text != "광고 금지" || got.ID != "" {
		t.Fatalf("approval = %+v", got)
	}
	if _, err := svc.Create(context.Background(), "alice", "짧게", ScopePatch{Scope: ScopeGlobal}, "  c9  "); err != nil {
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
	_, err := svc.Create(context.Background(), "alice", strings.Repeat("가", 11), ScopePatch{Scope: ScopeGlobal}, "c1")
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
	NewService(newFakeStore(), testFields, Limits{TextMaxChars: 10, MaxPerAccount: 1}, 0)
}

// ARCH-40: every fields scope and every preset write needs the directory, so a service without
// one is a wiring error, not a mode.
func TestNewServiceRequiresAFieldDirectory(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != "guideline: a field directory is required" {
			t.Fatalf("recovered %v", recovered)
		}
	}()
	NewService(newFakeStore(), nil, Limits{TextMaxChars: 10, MaxPerAccount: 1}, 1)
}
