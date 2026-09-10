package modelcatalog_test

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
)

var testNow = time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)

// fakeStore is an in-memory catalog_models + catalog_model_purposes. It records refreshes
// so a test can prove that a failed fetch wrote nothing.
type fakeStore struct {
	rows      map[string]modelcatalog.Model
	refreshes int
	syncs     int
	listErr   error
	syncErr   error
	combos    map[modelcatalog.Combo]modelcatalog.ComboAssignment
}

func newFakeStore(rows ...modelcatalog.Model) *fakeStore {
	s := &fakeStore{
		rows:   map[string]modelcatalog.Model{},
		combos: map[modelcatalog.Combo]modelcatalog.ComboAssignment{},
	}
	for _, row := range rows {
		s.rows[row.ModelID] = row
	}
	return s
}

func (s *fakeStore) List(context.Context) ([]modelcatalog.Model, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]modelcatalog.Model, 0, len(s.rows))
	for _, row := range s.rows {
		out = append(out, row)
	}
	return out, nil
}

func (s *fakeStore) Get(_ context.Context, modelID string) (modelcatalog.Model, error) {
	row, ok := s.rows[modelID]
	if !ok {
		return modelcatalog.Model{}, modelcatalog.ErrNotFound
	}
	return row, nil
}

func (s *fakeStore) Upsert(_ context.Context, m modelcatalog.Model) error {
	// The join table is separate storage: an upsert of the row must not invent or drop
	// registrations, mirroring the real store.
	if existing, ok := s.rows[m.ModelID]; ok {
		m.Purposes = existing.Purposes
	} else {
		m.Purposes = nil
	}
	s.rows[m.ModelID] = m
	return nil
}

func (s *fakeStore) Patch(_ context.Context, modelID string, patch modelcatalog.Patch, updatedAt time.Time) (modelcatalog.Model, error) {
	row, ok := s.rows[modelID]
	if !ok {
		return modelcatalog.Model{}, modelcatalog.ErrNotFound
	}
	// Both halves of the patch live on the registration row, so both refuse a purpose the
	// model does not serve — the real store's UPDATE matching zero rows says the same.
	if patch.Reasoning != nil || patch.Level != nil {
		if !slices.Contains(row.Purposes, patch.Purpose) {
			return modelcatalog.Model{}, modelcatalog.ErrPurposeNotRegistered
		}
	}
	if patch.Reasoning != nil {
		if *patch.Reasoning == llm.ReasoningUnspecified {
			delete(row.Reasoning, patch.Purpose)
		} else {
			if row.Reasoning == nil {
				row.Reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{}
			}
			row.Reasoning[patch.Purpose] = *patch.Reasoning
		}
	}
	if patch.Level != nil {
		if *patch.Level == "" {
			delete(row.Levels, patch.Purpose)
		} else {
			if row.Levels == nil {
				row.Levels = map[modelcatalog.Purpose]modelcatalog.Level{}
			}
			row.Levels[patch.Purpose] = *patch.Level
		}
	}
	row.UpdatedAt = updatedAt
	s.rows[modelID] = row
	return row, nil
}

func (s *fakeStore) RegisterPurpose(ctx context.Context, m modelcatalog.Model, purpose modelcatalog.Purpose) error {
	if err := s.Upsert(ctx, m); err != nil {
		return err
	}
	row := s.rows[m.ModelID]
	if !slices.Contains(row.Purposes, purpose) {
		row.Purposes = append(row.Purposes, purpose)
	}
	s.rows[m.ModelID] = row
	return nil
}

func (s *fakeStore) DeregisterPurpose(_ context.Context, modelID string, purpose modelcatalog.Purpose, at time.Time) error {
	row, ok := s.rows[modelID]
	if !ok {
		return nil
	}
	row.Purposes = slices.DeleteFunc(slices.Clone(row.Purposes), func(p modelcatalog.Purpose) bool { return p == purpose })
	// The effort and the level both live on the registration row (migrations 0021, 0040),
	// so removing the registration removes them — the real store deletes the row these maps
	// stand in for.
	if _, ok := row.Reasoning[purpose]; ok {
		row.Reasoning = maps.Clone(row.Reasoning)
		delete(row.Reasoning, purpose)
	}
	if _, ok := row.Levels[purpose]; ok {
		row.Levels = maps.Clone(row.Levels)
		delete(row.Levels, purpose)
	}
	row.UpdatedAt = at
	s.rows[modelID] = row
	return nil
}

// SyncPurposes is the document path's one write. The fake applies the writes in order; the
// real store's atomicity is pinned in store_test.
func (s *fakeStore) SyncPurposes(ctx context.Context, writes []modelcatalog.PurposeWrite, at time.Time) error {
	if s.syncErr != nil {
		return s.syncErr
	}
	s.syncs++
	for _, write := range writes {
		if write.Register {
			if err := s.RegisterPurpose(ctx, write.Model, write.Purpose); err != nil {
				return err
			}
			continue
		}
		if err := s.DeregisterPurpose(ctx, write.Model.ModelID, write.Purpose, at); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakeStore) RefreshAvailability(_ context.Context, seen []modelcatalog.Candidate, at time.Time) error {
	s.refreshes++
	offered := map[string]modelcatalog.Candidate{}
	for _, candidate := range seen {
		offered[candidate.ModelID] = candidate
	}
	for id, row := range s.rows {
		candidate, still := offered[id]
		row.Listed = still
		if still {
			row.Label, row.ContextTokens = candidate.Label, candidate.ContextTokens
			row.Vision, row.StructuredOutput = candidate.Vision, candidate.StructuredOutput
			row.ImageOutput, row.VideoOutput = candidate.ImageOutput, candidate.VideoOutput
			row.LastSeenAt = at
		} else if len(row.Purposes) > 0 {
			// MODEL-20, as the real store does it: unlisted loses every registration — and
			// the effort that belongs to each registration (MODEL-7) — but keeps the row.
			row.Purposes, row.Reasoning, row.UpdatedAt = nil, nil, at
		}
		s.rows[id] = row
	}
	return nil
}

// fakeUpstream answers with a fixed catalog, or with a failure.
type fakeUpstream struct {
	candidates []modelcatalog.Candidate
	err        error
	fromCache  bool
	calls      int
}

func (u *fakeUpstream) Fetch(context.Context, bool) (modelcatalog.Snapshot, error) {
	u.calls++
	if u.err != nil {
		return modelcatalog.Snapshot{}, u.err
	}
	return modelcatalog.Snapshot{Candidates: u.candidates, FetchedAt: testNow, FromCache: u.fromCache}, nil
}

func curated(id string, purposes ...modelcatalog.Purpose) modelcatalog.Model {
	return modelcatalog.Model{
		ModelID: id, ProviderSlug: modelcatalog.ProviderSlugOf(id), Label: id,
		Vision: true, Purposes: purposes, Listed: true,
		CreatedAt: testNow, UpdatedAt: testNow,
	}
}

// priced is a curated row a combo can actually be quoted with: registered, still listed,
// and carrying the two prices the estimator multiplies.
func priced(id string, in, out string, purposes ...modelcatalog.Purpose) modelcatalog.Model {
	row := curated(id, purposes...)
	row.InputUSDPerMillion = in
	row.OutputUSDPerMillion = out
	return row
}

func candidate(id string, created int64) modelcatalog.Candidate {
	return modelcatalog.Candidate{
		ModelID: id, ProviderSlug: modelcatalog.ProviderSlugOf(id), Label: id + " label",
		Vision: true, StructuredOutput: true, SourceCreatedAt: created,
	}
}

func newService(t *testing.T, store modelcatalog.Store, upstream modelcatalog.Upstream) *modelcatalog.Service {
	t.Helper()
	svc := modelcatalog.NewService(store)
	if upstream != nil {
		svc.SetUpstream(upstream)
	}
	if err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	return svc
}

// A2: the browse list is the live catalog annotated with curation. A curated model the
// provider has stopped offering is deregistered by the read and not shown (MODEL-20) — the
// operator has nothing left to do with it — while its row stays in the store for a return.
func TestBrowse_MergesLiveCatalogWithCuratedRows(t *testing.T) {
	store := newFakeStore(
		curated("openai/gpt-x", modelcatalog.PurposeWriting),
		curated("retired/model", modelcatalog.PurposeWriting),
	)
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("openai/gpt-x", 200), candidate("anthropic/claude-y", 100), candidate("openai/gpt-w", 300),
	}}
	svc := newService(t, store, upstream)

	browse, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 3 {
		t.Fatalf("entries = %d, want the 3 offered and no retired row", len(browse.Entries))
	}
	// Grouped by vendor, newest first inside a vendor.
	want := []string{"anthropic/claude-y", "openai/gpt-w", "openai/gpt-x"}
	for i, id := range want {
		if browse.Entries[i].ModelID != id {
			t.Errorf("entry %d = %s, want %s", i, browse.Entries[i].ModelID, id)
		}
	}
	byID := map[string]modelcatalog.Entry{}
	for _, entry := range browse.Entries {
		byID[entry.ModelID] = entry
	}
	if got := byID["openai/gpt-x"]; !got.Curated || !got.Listed ||
		!slices.Contains(got.Purposes, modelcatalog.PurposeWriting) {
		t.Errorf("curated offered entry = %+v", got)
	}
	if byID["anthropic/claude-y"].Curated || len(byID["anthropic/claude-y"].Purposes) != 0 {
		t.Errorf("an un-curated candidate looks curated: %+v", byID["anthropic/claude-y"])
	}
	if _, shown := byID["retired/model"]; shown {
		t.Error("a delisted model was offered on the screen")
	}
	// The row survives, unregistered, and the registry no longer serves it.
	retired, ok := store.rows["retired/model"]
	if !ok || retired.Listed || len(retired.Purposes) != 0 {
		t.Errorf("retired row = %+v / %v, want kept, unlisted, unregistered", retired, ok)
	}
	if _, served := svc.Lookup("retired/model"); served {
		t.Error("the registry still serves a delisted model")
	}
	if store.refreshes != 1 {
		t.Errorf("refreshes = %d, want one bookkeeping write for the live read", store.refreshes)
	}
}

// MODEL-20 on the degraded path: a row an earlier successful read unlisted stays hidden
// while the fetch is failing — the stored rows are served, but not that one.
func TestBrowse_HidesADelistedRowWhenTheFetchFails(t *testing.T) {
	delisted := curated("retired/model")
	delisted.Listed = false
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting), delisted)
	svc := newService(t, store, &fakeUpstream{err: errors.New("network down")})

	browse, err := svc.Browse(context.Background(), true, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 1 || browse.Entries[0].ModelID != "openai/gpt-x" {
		t.Fatalf("entries = %+v, want only the listed curated row", browse.Entries)
	}
}

// MODEL-20: a model the source offers again is an unregistered candidate on a row that still
// exists — the operator re-checks a box and sets the effort again, as after any uncheck.
func TestBrowse_AReturningModelIsOfferedAgainUnregistered(t *testing.T) {
	returning := curated("retired/model")
	returning.Listed = false
	store := newFakeStore(returning)
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{candidate("retired/model", 10)}}
	svc := newService(t, store, upstream)

	browse, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 1 {
		t.Fatalf("entries = %+v, want the returning model once", browse.Entries)
	}
	got := browse.Entries[0]
	if !got.Listed || !got.Curated || len(got.Purposes) != 0 {
		t.Errorf("returning entry = %+v, want listed, curated, unregistered", got)
	}
	if got.Reasoning != llm.ReasoningUnspecified {
		t.Errorf("effort = %v, want none: the override went with the registration", got.Reasoning)
	}
}

// A7: an upstream failure degrades to curated rows and says so. It must not be treated as
// evidence that models disappeared.
func TestBrowse_DegradesWithoutWritingBookkeeping(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting))
	svc := newService(t, store, &fakeUpstream{err: errors.New("network down")})

	browse, err := svc.Browse(context.Background(), true, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if browse.FetchError == "" {
		t.Error("a failed fetch was reported as success")
	}
	if len(browse.Entries) != 1 || browse.Entries[0].ModelID != "openai/gpt-x" {
		t.Fatalf("entries = %+v, want the curated row", browse.Entries)
	}
	if !browse.Entries[0].Listed {
		t.Error("an outage unlisted a model")
	}
	if store.refreshes != 0 {
		t.Errorf("refreshes = %d, want none for a failed fetch", store.refreshes)
	}
}

// A cached snapshot is the same models the last successful read already recorded, so it
// buys no new evidence and must not spend a write on every screen load.
func TestBrowse_CachedSnapshotWritesNoBookkeeping(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting))
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{candidate("openai/gpt-x", 1)}, fromCache: true}
	svc := newService(t, store, upstream)

	if _, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}
	if store.refreshes != 0 {
		t.Errorf("refreshes = %d, want none for a cached snapshot", store.refreshes)
	}
}

// Registering snapshots the live entry and the registry's view follows immediately — no
// restart, no second call. The registered purposes project onto the matching stages.
func TestSetPurpose_SnapshotsAndAppliesWithoutRestart(t *testing.T) {
	store := newFakeStore()
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{candidate("openai/gpt-x", 10)}}
	svc := newService(t, store, upstream)

	if len(svc.Models()) != 0 {
		t.Fatal("the registry saw a model before anything was curated")
	}
	model, err := svc.SetPurpose(context.Background(), "openai/gpt-x", modelcatalog.PurposePhotoAnalysis, true)
	if err != nil {
		t.Fatal(err)
	}
	if model.Label != "openai/gpt-x label" || !model.Vision || !model.StructuredOutput {
		t.Fatalf("registered model = %+v", model)
	}
	if _, err := svc.SetPurpose(context.Background(), "openai/gpt-x", modelcatalog.PurposeWriting, true); err != nil {
		t.Fatal(err)
	}
	source, ok := svc.Lookup("openai/gpt-x")
	if !ok {
		t.Fatal("the registry does not see the registered model")
	}
	if !slices.Equal(source.Stages, []string{"observe", "write"}) {
		t.Errorf("stages = %v, want the purposes projected onto stages", source.Stages)
	}
	if len(svc.Models()) != 1 {
		t.Errorf("registry models = %d, want 1", len(svc.Models()))
	}
}

// A1: purposes are independent — deregistering one leaves the others standing, and only a
// model with zero registrations disappears from the registry's view.
func TestSetPurpose_PurposesAreIndependent(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting))
	svc := newService(t, store, nil)

	model, err := svc.SetPurpose(context.Background(), "openai/gpt-x", modelcatalog.PurposePhotoAnalysis, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(model.Purposes, []modelcatalog.Purpose{modelcatalog.PurposeWriting}) {
		t.Fatalf("purposes = %v, want writing to survive", model.Purposes)
	}
	source, ok := svc.Lookup("openai/gpt-x")
	if !ok || !slices.Equal(source.Stages, []string{"write"}) {
		t.Fatalf("stages = %v, want just write", source.Stages)
	}

	if _, err := svc.SetPurpose(context.Background(), "openai/gpt-x", modelcatalog.PurposeWriting, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.Lookup("openai/gpt-x"); ok {
		t.Error("a model with zero registrations is still selectable")
	}
	if len(svc.Models()) != 0 {
		t.Error("a model with zero registrations is still in the catalog")
	}
}

// A4: a generation purpose registers and persists, but projects onto no stage, so the
// registry serves the model to no picker.
func TestSetPurpose_GenerationPurposesFeedNoStage(t *testing.T) {
	store := newFakeStore()
	imageModel := candidate("openai/image-gen", 5)
	imageModel.ImageOutput = true
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{imageModel}})

	model, err := svc.SetPurpose(context.Background(), "openai/image-gen", modelcatalog.PurposeImageGeneration, true)
	if err != nil {
		t.Fatal(err)
	}
	if !model.ImageOutput || !slices.Equal(model.Purposes, []modelcatalog.Purpose{modelcatalog.PurposeImageGeneration}) {
		t.Fatalf("registered model = %+v", model)
	}
	source, ok := svc.Lookup("openai/image-gen")
	if !ok || len(source.Stages) != 0 {
		t.Fatalf("stages = %v, want none for a generation-only registration", source.Stages)
	}
}

// A2: the capability gate is enforced server-side. A model without vision cannot join
// photo-analysis; without the matching output modality it cannot join a generation purpose.
func TestSetPurpose_EnforcesTheCapabilityGate(t *testing.T) {
	textOnly := candidate("openai/text-only", 5)
	textOnly.Vision = false
	svc := newService(t, newFakeStore(), &fakeUpstream{candidates: []modelcatalog.Candidate{textOnly}})

	for _, purpose := range []modelcatalog.Purpose{
		modelcatalog.PurposePhotoAnalysis,
		modelcatalog.PurposeImageGeneration,
		modelcatalog.PurposeVideoGeneration,
	} {
		if _, err := svc.SetPurpose(context.Background(), "openai/text-only", purpose, true); !errors.Is(err, modelcatalog.ErrPurposeIneligible) {
			t.Errorf("%s = %v, want ErrPurposeIneligible", purpose, err)
		}
	}
	// The gate refuses the registration entirely — nothing is written.
	if _, ok := svc.Lookup("openai/text-only"); ok {
		t.Error("a refused registration reached the registry")
	}
	if _, err := svc.SetPurpose(context.Background(), "openai/text-only", modelcatalog.PurposeWriting, true); err != nil {
		t.Errorf("writing (ungated) = %v", err)
	}
}

// A refresh that removes the capability a registration was gated on stops the stage at
// once, while the registration row survives for the operator to see and uncheck — the
// flag-never-auto-retire contract, extended to capability drift.
func TestBrowse_CapabilityDriftStopsTheStageButKeepsTheRegistration(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting))
	sightless := candidate("openai/gpt-x", 10)
	sightless.Vision = false
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{sightless}})

	browse, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 1 || len(browse.Entries[0].Purposes) != 2 {
		t.Fatalf("entries = %+v, want the registration kept for the operator", browse.Entries)
	}
	source, ok := svc.Lookup("openai/gpt-x")
	if !ok {
		t.Fatal("the still-writing-registered model left the registry entirely")
	}
	if !slices.Equal(source.Stages, []string{"write"}) {
		t.Errorf("stages = %v, want observe gone once vision is gone", source.Stages)
	}
}

// A model nobody offers any more can still be re-registered from its stored row: an
// operator undoing a deregistration must not have to wait for the provider to be reachable.
func TestSetPurpose_ReRegistersFromTheStoredRowWhenUpstreamIsDown(t *testing.T) {
	row := curated("retired/model")
	row.Purposes = []modelcatalog.Purpose{modelcatalog.PurposeWriting}
	row.Reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{modelcatalog.PurposeWriting: llm.ReasoningHigh}
	row.Listed = false
	store := newFakeStore(row)
	svc := newService(t, store, &fakeUpstream{err: errors.New("network down")})

	model, err := svc.SetPurpose(context.Background(), "retired/model", modelcatalog.PurposeWriting, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(model.Purposes, modelcatalog.PurposeWriting) {
		t.Fatalf("re-registered model = %+v", model)
	}
	// Curation the operator made earlier survives; so does the fact that it is unoffered.
	if model.Reasoning[modelcatalog.PurposeWriting] != llm.ReasoningHigh || model.Listed {
		t.Errorf("re-registration overwrote stored state: %+v", model)
	}
	source, ok := svc.Lookup("retired/model")
	if !ok || !source.Delisted {
		t.Errorf("the registry does not see it as delisted: %+v", source)
	}
}

func TestSetPurpose_RefusesUnknownModelsAndPurposes(t *testing.T) {
	svc := newService(t, newFakeStore(), &fakeUpstream{candidates: []modelcatalog.Candidate{candidate("openai/gpt-x", 1)}})

	if _, err := svc.SetPurpose(context.Background(), "nobody/has-this", modelcatalog.PurposeWriting, true); !errors.Is(err, modelcatalog.ErrNotFound) {
		t.Errorf("unknown model = %v, want ErrNotFound", err)
	}
	if _, err := svc.SetPurpose(context.Background(), "nobody/has-this", modelcatalog.PurposeWriting, false); !errors.Is(err, modelcatalog.ErrNotFound) {
		t.Errorf("deregister without a row = %v, want ErrNotFound", err)
	}
	if _, err := svc.SetPurpose(context.Background(), "openai/gpt-x", modelcatalog.Purpose("cooking"), true); !errors.Is(err, modelcatalog.ErrUnknownPurpose) {
		t.Errorf("unknown purpose = %v, want ErrUnknownPurpose", err)
	}
}

// A7/A11: the override round-trips through an edit for the purpose it was made on, and the
// registry view follows at once — projected onto that purpose's STAGE.
func TestUpdate_AppliesCurationToTheRegistryView(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting))
	svc := newService(t, store, nil)

	high := llm.ReasoningHigh
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: &high,
	}); err != nil {
		t.Fatal(err)
	}
	source, _ := svc.Lookup("openai/gpt-x")
	if source.Reasoning[llm.StageNameWrite] != llm.ReasoningHigh {
		t.Errorf("write reasoning = %q, want high", source.Reasoning[llm.StageNameWrite])
	}
}

// A1/A2 — the 2026-09-03 regression, asserted directly: two purposes of one model hold two
// independent efforts, and editing one leaves the other exactly as it was.
func TestUpdate_KeepsEveryOtherPurposeUnchanged(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x",
		modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting, modelcatalog.PurposeStyleAnalysis))
	svc := newService(t, store, nil)

	minimal, high := llm.ReasoningMinimal, llm.ReasoningHigh
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposePhotoAnalysis, Reasoning: &high,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: &minimal,
	}); err != nil {
		t.Fatal(err)
	}
	source, _ := svc.Lookup("openai/gpt-x")
	if got := source.Reasoning[llm.StageNameObserve]; got != llm.ReasoningHigh {
		t.Errorf("observe reasoning = %q, want high — the writing edit reached it", got)
	}
	if got := source.Reasoning[llm.StageNameWrite]; got != llm.ReasoningMinimal {
		t.Errorf("write reasoning = %q, want minimal", got)
	}
	// A3/A5: an untouched purpose sends NO key — it falls through to the stage policy.
	if got, ok := source.Reasoning[llm.StageNameAnalyze]; ok {
		t.Errorf("analyze reasoning = %q, want no override at all", got)
	}
}

func TestUpdate_RefusesUnknownModelAndBadValues(t *testing.T) {
	svc := newService(t, newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting)), nil)

	none := llm.ReasoningNone
	if _, err := svc.Update(context.Background(), "nobody/has-this", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: &none,
	}); !errors.Is(err, modelcatalog.ErrNotFound) {
		t.Errorf("unknown model = %v, want ErrNotFound", err)
	}
	nonsense := llm.ReasoningEffort("enormous")
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: &nonsense,
	}); !errors.Is(err, modelcatalog.ErrInvalidReasoning) {
		t.Errorf("bad effort = %v, want ErrInvalidReasoning", err)
	}
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{Reasoning: &none}); !errors.Is(err, modelcatalog.ErrUnknownPurpose) {
		t.Errorf("absent purpose = %v, want ErrUnknownPurpose", err)
	}
	// The control appears only once registered, and the server holds the same rule.
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposePhotoAnalysis, Reasoning: &none,
	}); !errors.Is(err, modelcatalog.ErrPurposeNotRegistered) {
		t.Errorf("unregistered purpose = %v, want ErrPurposeNotRegistered", err)
	}
}

// A11: the operator sees, per purpose, that a model is spending its budget on reasoning —
// without reading the database and without waiting for a user's job to fail. The evidence
// comes from the ledger through this context's own port; the catalog never reads usage_events.
func TestBrowse_AttachesTheReasoningSpendForTheListedPurpose(t *testing.T) {
	store := newFakeStore(
		curated("openai/reasoner", modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting),
		curated("openai/quiet", modelcatalog.PurposeWriting),
		curated("openai/unmeasured", modelcatalog.PurposeWriting),
	)
	// The three are offered live: a successful read that lacked them would deregister and
	// hide them (MODEL-20), and this test is about the spend column, not that.
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("openai/reasoner", 3), candidate("openai/quiet", 2), candidate("openai/unmeasured", 1),
	}})
	svc.SetReasoningSpend(fakeSpend{rows: map[string][]modelcatalog.SpendRow{
		llm.StageNameWrite: {
			{Model: "openai/reasoner", Calls: 3, ReasoningTokens: 24_000, CompletionTokens: 24_300, ReasoningTruncations: 2},
			{Model: "openai/quiet", Calls: 5, ReasoningTokens: 120, CompletionTokens: 6_000},
			// A model with no recorded call renders nothing rather than a zero that would
			// read as a measurement.
			{Model: "openai/unmeasured", Calls: 0},
		},
		llm.StageNameObserve: {
			{Model: "openai/reasoner", Calls: 2, ReasoningTokens: 40, CompletionTokens: 1_300},
		},
	}})

	writing, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]modelcatalog.Entry{}
	for _, entry := range writing.Entries {
		byID[entry.ModelID] = entry
	}
	if spend := byID["openai/reasoner"].ReasoningSpend; spend == nil || spend.ReasoningShare() < 0.9 {
		t.Fatalf("reasoner writing spend = %+v, want a reasoning-heavy share", spend)
	} else if spend.ReasoningTruncations != 2 {
		t.Fatalf("reasoner truncations = %d, want 2", spend.ReasoningTruncations)
	}
	if spend := byID["openai/quiet"].ReasoningSpend; spend == nil || spend.ReasoningShare() > 0.1 {
		t.Fatalf("quiet writing spend = %+v", spend)
	}
	if spend := byID["openai/unmeasured"].ReasoningSpend; spend != nil {
		t.Fatalf("a model with no recorded call carried %+v", spend)
	}

	// The SAME model on another tab reports that tab's stage — the evidence matches the tab.
	observing, err := svc.Browse(context.Background(), false, modelcatalog.PurposePhotoAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range observing.Entries {
		if entry.ModelID != "openai/reasoner" {
			continue
		}
		if entry.ReasoningSpend == nil || entry.ReasoningSpend.ReasoningShare() > 0.1 {
			t.Fatalf("reasoner observe spend = %+v, want the observation stage's own numbers", entry.ReasoningSpend)
		}
	}
}

// The signal is evidence beside a control, so a ledger that cannot answer must never be what
// stops an operator from curating.
func TestBrowse_SurvivesAFailingSpendRead(t *testing.T) {
	svc := newService(t, newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting)),
		&fakeUpstream{candidates: []modelcatalog.Candidate{candidate("openai/gpt-x", 1)}})
	svc.SetReasoningSpend(fakeSpend{err: errors.New("ledger unavailable")})

	browse, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatalf("a failing spend read broke the whole screen: %v", err)
	}
	for _, entry := range browse.Entries {
		if entry.ReasoningSpend != nil {
			t.Errorf("%s carried a spend signal from a failed read", entry.ModelID)
		}
	}
}

type fakeSpend struct {
	rows map[string][]modelcatalog.SpendRow
	err  error
}

func (f fakeSpend) ReasoningSpendByModel(_ context.Context, stage string) ([]modelcatalog.SpendRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rows[stage], nil
}

// A4: the registry's view is memory, so the user-facing catalog never touches the network —
// an upstream that is not even configured changes nothing for users.
func TestModelSource_ServesWithoutAnUpstream(t *testing.T) {
	svc := newService(t, newFakeStore(
		curated("openai/gpt-x", modelcatalog.PurposeWriting),
		curated("openai/gpt-off"),
	), nil)

	models := svc.Models()
	if len(models) != 1 || models[0].ModelID != "openai/gpt-x" {
		t.Fatalf("models = %+v, want only the registered one", models)
	}
	browse, err := svc.Browse(context.Background(), true, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if browse.FetchError == "" {
		t.Error("browsing without an upstream reported no problem")
	}
	if len(browse.Entries) != 2 {
		t.Errorf("entries = %d, want both curated rows", len(browse.Entries))
	}
}

func TestProviderSlugOf(t *testing.T) {
	for id, want := range map[string]string{
		"openai/gpt-5.6-sol": "openai",
		"z-ai/glm-5.3":       "z-ai",
		"openrouter/free":    "openrouter",
		"bare":               "bare",
	} {
		if got := modelcatalog.ProviderSlugOf(id); got != want {
			t.Errorf("ProviderSlugOf(%q) = %q, want %q", id, got, want)
		}
	}
}

// --- reasoning capability (change 27) ---

func reasoningCapable(efforts []string, mandatory bool) modelcatalog.ReasoningCapability {
	return modelcatalog.ReasoningCapability{
		Reasons: true, Efforts: efforts, DefaultEffort: "high",
		Mandatory: mandatory, NativeEffort: true,
	}
}

// A6: the override is bounded by what the model publishes. The enum check stays the first
// gate, and a model with no published list keeps accepting all eight values.
func TestUpdate_RefusesAnEffortTheModelDoesNotAccept(t *testing.T) {
	bounded := curated("deepseek/deepseek-v4-pro-0813", modelcatalog.PurposeWriting)
	bounded.ReasoningCapability = reasoningCapable([]string{"max", "high", "low"}, false)
	mandatory := curated("google/gemini-3.8-flash", modelcatalog.PurposeWriting)
	mandatory.ReasoningCapability = reasoningCapable([]string{"high", "medium", "low"}, true)
	// No published list: absence is unknown, not "supports nothing".
	listless := curated("vendor/no-list", modelcatalog.PurposeWriting)
	listless.ReasoningCapability = modelcatalog.ReasoningCapability{Reasons: true}
	svc := newService(t, newFakeStore(bounded, mandatory, listless), nil)

	set := func(modelID string, effort llm.ReasoningEffort) error {
		_, err := svc.Update(context.Background(), modelID, modelcatalog.Patch{
			Purpose: modelcatalog.PurposeWriting, Reasoning: &effort,
		})
		return err
	}
	// A6, against a real model: `medium` is a valid effort and not one this model takes.
	if err := set("deepseek/deepseek-v4-pro-0813", llm.ReasoningEffort("medium")); !errors.Is(err, modelcatalog.ErrInvalidReasoning) {
		t.Errorf("medium on a max/high/low model = %v, want ErrInvalidReasoning", err)
	}
	// The same model is not mandatory yet lists no `none` — the shape that let the app's own
	// "turn it off" control send a value the model does not take.
	if err := set("deepseek/deepseek-v4-pro-0813", llm.ReasoningNone); !errors.Is(err, modelcatalog.ErrInvalidReasoning) {
		t.Errorf("none on a model that does not list it = %v, want ErrInvalidReasoning", err)
	}
	if err := set("deepseek/deepseek-v4-pro-0813", llm.ReasoningEffort("high")); err != nil {
		t.Errorf("a published value was refused: %v", err)
	}
	// Mandatory: `none` is refused even though the list is otherwise satisfied.
	if err := set("google/gemini-3.8-flash", llm.ReasoningNone); !errors.Is(err, modelcatalog.ErrInvalidReasoning) {
		t.Errorf("none on a mandatory model = %v, want ErrInvalidReasoning", err)
	}
	if err := set("google/gemini-3.8-flash", llm.ReasoningEffort("medium")); err != nil {
		t.Errorf("a published value was refused on the mandatory model: %v", err)
	}
	// A6 second half: every value stays acceptable where the source publishes no list.
	for _, effort := range []llm.ReasoningEffort{llm.ReasoningNone, "minimal", "low", "medium", "high", "xhigh", "max"} {
		if err := set("vendor/no-list", effort); err != nil {
			t.Errorf("%q was refused on a model with no published list: %v", effort, err)
		}
	}
	// Clearing the override is always allowed: it is not a claim about the model.
	if err := set("vendor/no-list", llm.ReasoningUnspecified); err != nil {
		t.Errorf("clearing the override was refused: %v", err)
	}
}

// A7: an override that is no longer in its model's list survives, is still projected onto the
// stage keys, and is reported as drifted.
func TestBrowse_ReportsDriftWithoutChangingWhatIsSent(t *testing.T) {
	row := curated("deepseek/deepseek-v4-pro-0813", modelcatalog.PurposeWriting)
	row.Reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{
		modelcatalog.PurposeWriting: llm.ReasoningEffort("medium"),
	}
	store := newFakeStore(row)
	live := candidate("deepseek/deepseek-v4-pro-0813", 1)
	live.ReasoningCapability = reasoningCapable([]string{"max", "high", "low"}, false)
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{live}})

	browse, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 1 {
		t.Fatalf("entries = %d", len(browse.Entries))
	}
	entry := browse.Entries[0]
	if !entry.ReasoningDrifted {
		t.Fatal("an override outside the live list is not reported as drifted")
	}
	// Kept, not rewritten: the source's list changing is not a mandate to change a decision.
	if entry.Reasoning != "medium" {
		t.Fatalf("the drifted override was rewritten to %q", entry.Reasoning)
	}
	// And it is still what the registry projects onto the stage, so the call still sends it.
	info, ok := svc.Lookup("deepseek/deepseek-v4-pro-0813")
	if !ok {
		t.Fatal("the drifted model left the registry")
	}
	if info.Reasoning[llm.StageNameWrite] != "medium" {
		t.Fatalf("stage reasoning = %+v, want the stored override still projected", info.Reasoning)
	}
}

// A value the model still publishes is not drift, and neither is an absent list or an
// unset override — drift is a positive answer, never a default.
func TestBrowse_ReportsNoDriftWithoutAPositiveMismatch(t *testing.T) {
	inList := curated("a/in-list", modelcatalog.PurposeWriting)
	inList.Reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{modelcatalog.PurposeWriting: "high"}
	noList := curated("b/no-list", modelcatalog.PurposeWriting)
	noList.Reasoning = map[modelcatalog.Purpose]llm.ReasoningEffort{modelcatalog.PurposeWriting: "medium"}
	noOverride := curated("c/no-override", modelcatalog.PurposeWriting)

	withList := candidate("a/in-list", 3)
	withList.ReasoningCapability = reasoningCapable([]string{"max", "high", "low"}, false)
	without := candidate("b/no-list", 2)
	without.ReasoningCapability = modelcatalog.ReasoningCapability{Reasons: true}
	bare := candidate("c/no-override", 1)
	bare.ReasoningCapability = reasoningCapable([]string{"max", "high", "low"}, false)

	svc := newService(t, newFakeStore(inList, noList, noOverride), &fakeUpstream{
		candidates: []modelcatalog.Candidate{withList, without, bare},
	})
	browse, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range browse.Entries {
		if entry.ReasoningDrifted {
			t.Errorf("%s reported drift: override=%q list=%v", entry.ModelID, entry.Reasoning, entry.Efforts)
		}
	}
}

// A1: a register refreshes the capability from the live entry, exactly as it refreshes the
// label and the pricing.
func TestSetPurpose_SnapshotsTheReasoningCapability(t *testing.T) {
	store := newFakeStore()
	live := candidate("deepseek/deepseek-v4-pro-0813", 1)
	live.ReasoningCapability = reasoningCapable([]string{"max", "high", "low"}, false)
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{live}})

	row, err := svc.SetPurpose(context.Background(), "deepseek/deepseek-v4-pro-0813", modelcatalog.PurposeWriting, true)
	if err != nil {
		t.Fatal(err)
	}
	if !row.Reasons || row.DefaultEffort != "high" || len(row.Efforts) != 3 || !row.NativeEffort {
		t.Fatalf("the register did not snapshot the capability: %+v", row.ReasoningCapability)
	}
}

// MODEL-46/47: the two capability facts execution needs cross the catalog boundary; the
// remaining descriptive fields stay catalog-only.
func TestModelSource_CarriesExecutionReasoningCapability(t *testing.T) {
	plain := curated("openai/gpt-x", modelcatalog.PurposeWriting)
	rich := curated("openai/gpt-x-rich", modelcatalog.PurposeWriting)
	rich.ReasoningCapability = reasoningCapable([]string{"max", "high", "low"}, true)
	svc := newService(t, newFakeStore(plain, rich), nil)

	a, okA := svc.Lookup("openai/gpt-x")
	b, okB := svc.Lookup("openai/gpt-x-rich")
	if !okA || !okB {
		t.Fatal("a curated model left the registry")
	}
	if a.ReasoningNativeEffort || len(a.ReasoningEfforts) != 0 {
		t.Fatalf("plain capability leaked values: %+v", a)
	}
	if !b.ReasoningNativeEffort || !reflect.DeepEqual(b.ReasoningEfforts, []llm.ReasoningEffort{llm.ReasoningMax, llm.ReasoningHigh, llm.ReasoningLow}) {
		t.Fatalf("execution capability was not carried: %+v", b)
	}
}

// A8 server-side, and the ambiguity the six fields cannot resolve alone: a CONFIRMED
// non-reasoning model accepts no effort, while a row that merely predates this data — the
// shape migration 0024 leaves every existing row in — still accepts everything.
func TestUpdate_RefusesAnEffortOnAConfirmedNonReasoningModel(t *testing.T) {
	confirmed := curated("vendor/no-reasoning", modelcatalog.PurposeWriting)
	unknown := curated("vendor/never-refreshed", modelcatalog.PurposeWriting)
	store := newFakeStore(confirmed, unknown)
	// Only the confirmed one is offered upstream, and its live entry says it does not reason
	// while still declaring something (mandatory: false is not enough — the flags below are).
	live := candidate("vendor/no-reasoning", 1)
	live.ReasoningCapability = modelcatalog.ReasoningCapability{NativeEffort: true}
	svc := newService(t, store, &fakeUpstream{candidates: []modelcatalog.Candidate{live}})

	medium := llm.ReasoningEffort("medium")
	if _, err := svc.Update(context.Background(), "vendor/no-reasoning", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: &medium,
	}); !errors.Is(err, modelcatalog.ErrInvalidReasoning) {
		t.Errorf("an effort on a confirmed non-reasoning model = %v, want ErrInvalidReasoning", err)
	}
	// Clearing it is still allowed: that is not a claim about the model.
	if _, err := svc.Update(context.Background(), "vendor/no-reasoning", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: ptr(llm.ReasoningUnspecified),
	}); err != nil {
		t.Errorf("clearing the override on a non-reasoning model was refused: %v", err)
	}
	// The never-refreshed row says nothing at all, so nothing may be inferred from it.
	if _, err := svc.Update(context.Background(), "vendor/never-refreshed", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: &medium,
	}); err != nil {
		t.Errorf("a row with no capability data refused an effort: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

// Drift is "this value would be refused today", not "this value left a list" — which is what
// catches a model that became mandatory, and one that stopped reasoning, with no list to
// compare against at all.
func TestDriftedFrom_CatchesEveryRefusalNotJustAMissingListEntry(t *testing.T) {
	for name, tc := range map[string]struct {
		capability modelcatalog.ReasoningCapability
		effort     llm.ReasoningEffort
		want       bool
	}{
		"left the published list": {
			reasoningCapable([]string{"max", "high", "low"}, false), "medium", true,
		},
		"none on a model that became mandatory, with no list": {
			modelcatalog.ReasoningCapability{Reasons: true, Mandatory: true}, llm.ReasoningNone, true,
		},
		"an effort on a model that stopped reasoning": {
			modelcatalog.ReasoningCapability{NativeEffort: true}, "high", true,
		},
		"still in the list":    {reasoningCapable([]string{"max", "high", "low"}, false), "high", false},
		"no list published":    {modelcatalog.ReasoningCapability{Reasons: true}, "medium", false},
		"capability unknown":   {modelcatalog.ReasoningCapability{}, "medium", false},
		"no override at all":   {reasoningCapable([]string{"max"}, false), llm.ReasoningUnspecified, false},
		"unset is not a value": {reasoningCapable([]string{"max"}, false), llm.ReasoningUnset, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.capability.DriftedFrom(tc.effort); got != tc.want {
				t.Fatalf("DriftedFrom(%q) = %v, want %v", tc.effort, got, tc.want)
			}
		})
	}
}

// The capability read from a row that never had one must not be mistaken for an answer.
func TestReasoningCapability_KnownOnlyWhenSomethingWasRead(t *testing.T) {
	if (modelcatalog.ReasoningCapability{}).Known() {
		t.Fatal("an empty capability claims to be known")
	}
	// Any one field carrying something is a read that looked, including a bare native-effort
	// signal on a model with no reasoning object.
	for name, capability := range map[string]modelcatalog.ReasoningCapability{
		"reasons":       {Reasons: true},
		"a list":        {Efforts: []string{"high"}},
		"a default":     {DefaultEffort: "high"},
		"mandatory":     {Mandatory: true},
		"native effort": {NativeEffort: true},
		"max tokens":    {MaxTokens: true},
	} {
		if !capability.Known() {
			t.Errorf("%s: Known() = false", name)
		}
	}
}

// A failed fetch serves stored rows, and they must NOT claim their capability is known:
// the admin has to keep offering the full vocabulary rather than hide a control whose
// stored value is still being sent.
func TestBrowse_ReportsAStoredCapabilityAsUnknown(t *testing.T) {
	row := curated("vendor/never-refreshed", modelcatalog.PurposeWriting)
	svc := newService(t, newFakeStore(row), &fakeUpstream{err: errors.New("network down")})

	browse, err := svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 1 || browse.Entries[0].ReasoningKnown {
		t.Fatalf("a stored entry claimed a known capability: %+v", browse.Entries)
	}
	// And a live read is known by construction, including a `reasons: false` that means it.
	live := candidate("vendor/never-refreshed", 1)
	live.ReasoningCapability = modelcatalog.ReasoningCapability{NativeEffort: true}
	svc = newService(t, newFakeStore(row), &fakeUpstream{candidates: []modelcatalog.Candidate{live}})
	browse, err = svc.Browse(context.Background(), false, modelcatalog.PurposeWriting)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 1 || !browse.Entries[0].ReasoningKnown {
		t.Fatalf("a live entry did not report a known capability: %+v", browse.Entries)
	}
}

func (f *fakeStore) ListCombos(context.Context) ([]modelcatalog.ComboAssignment, error) {
	out := make([]modelcatalog.ComboAssignment, 0, len(f.combos))
	for _, combo := range modelcatalog.Combos() {
		if assignment, ok := f.combos[combo]; ok {
			out = append(out, assignment)
		}
	}
	return out, nil
}

func (f *fakeStore) AssignCombo(_ context.Context, a modelcatalog.ComboAssignment, _ time.Time) error {
	f.combos[a.Combo] = a
	return nil
}

// QUOTA-39: a combo prices what a real run of it would cost, so both models must be
// registered to the stage they serve. Assigning a writer as the observer is the mistake this
// refusal exists for.
func TestAssignCombo_RequiresBothModelsRegisteredForTheirStage(t *testing.T) {
	store := newFakeStore(
		priced("vendor/eyes", "0.30", "2.50", modelcatalog.PurposePhotoAnalysis),
		priced("vendor/pen", "1.00", "10.00", modelcatalog.PurposeWriting),
	)
	svc := newService(t, store, nil)
	ctx := context.Background()

	if err := svc.AssignCombo(ctx, modelcatalog.ComboQuality, "vendor/eyes", "vendor/pen"); err != nil {
		t.Fatalf("AssignCombo: %v", err)
	}
	if err := svc.AssignCombo(ctx, "premium", "vendor/eyes", "vendor/pen"); !errors.Is(err, modelcatalog.ErrUnknownCombo) {
		t.Errorf("unknown combo = %v, want ErrUnknownCombo", err)
	}
	// The writer cannot observe and the observer cannot write.
	if err := svc.AssignCombo(ctx, modelcatalog.ComboValue, "vendor/pen", "vendor/pen"); !errors.Is(err, modelcatalog.ErrComboModelUnusable) {
		t.Errorf("writer as observer = %v, want ErrComboModelUnusable", err)
	}
	if err := svc.AssignCombo(ctx, modelcatalog.ComboValue, "vendor/eyes", "vendor/eyes"); !errors.Is(err, modelcatalog.ErrComboModelUnusable) {
		t.Errorf("observer as writer = %v, want ErrComboModelUnusable", err)
	}
	if err := svc.AssignCombo(ctx, modelcatalog.ComboValue, "vendor/ghost", "vendor/pen"); !errors.Is(err, modelcatalog.ErrComboModelUnusable) {
		t.Errorf("uncurated model = %v, want ErrComboModelUnusable", err)
	}

	// Combos reports all four so the operator sees which tier is still empty.
	all, err := svc.Combos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 || all[0].Combo != modelcatalog.ComboQuality || all[0].ObserveModelID != "vendor/eyes" {
		t.Fatalf("combos = %+v", all)
	}
	if all[1].ObserveModelID != "" {
		t.Errorf("balanced = %+v, want an empty assignment", all[1])
	}
}

// A combo whose model lost its registration, left the catalog or has no published price is
// dropped from what a comparison screen is given rather than priced anyway.
func TestComboRates_OmitsWhatCannotBeQuoted(t *testing.T) {
	eyes := priced("vendor/eyes", "0.30", "2.50", modelcatalog.PurposePhotoAnalysis)
	pen := priced("vendor/pen", "1.00", "10.00", modelcatalog.PurposeWriting)
	freePen := priced("vendor/free-pen", "", "", modelcatalog.PurposeWriting)
	goneEyes := priced("vendor/gone", "0.30", "2.50", modelcatalog.PurposePhotoAnalysis)
	goneEyes.Listed = false
	store := newFakeStore(eyes, pen, freePen, goneEyes)
	svc := newService(t, store, nil)
	ctx := context.Background()

	for _, assignment := range []struct {
		combo          modelcatalog.Combo
		observe, write string
	}{
		{modelcatalog.ComboQuality, "vendor/eyes", "vendor/pen"},
		{modelcatalog.ComboBalanced, "vendor/eyes", "vendor/free-pen"},
		{modelcatalog.ComboValue, "vendor/gone", "vendor/pen"},
	} {
		if err := svc.AssignCombo(ctx, assignment.combo, assignment.observe, assignment.write); err != nil {
			t.Fatalf("assign %s: %v", assignment.combo, err)
		}
	}

	priced, err := svc.ComboRates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(priced) != 1 || priced[0].Combo != modelcatalog.ComboQuality {
		t.Fatalf("priced combos = %+v, want only quality", priced)
	}
	// The same figures plan_test pins, arriving through a real assignment.
	rates := priced[0].Rates
	if rates.PerPhoto != 723 || rates.PerVideo != 1100 || rates.Per1000Chars != 3600 || rates.PerPostBase != 3800 {
		t.Errorf("rates = %+v", rates)
	}
	if priced[0].ObserveLabel != "vendor/eyes" || priced[0].WriteLabel != "vendor/pen" {
		t.Errorf("labels = %q / %q", priced[0].ObserveLabel, priced[0].WriteLabel)
	}
}

// T092/MODEL-57: a level is set per registration and reaches the registry view keyed by
// STAGE, the same projection the effort gets. Two purposes of one model hold two
// independent levels — the whole reason the column is on the registration.
func TestUpdate_LevelIsPerRegistrationAndReachesTheStageView(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x",
		modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting, modelcatalog.PurposeStyleAnalysis))
	svc := newService(t, store, nil)
	ctx := context.Background()

	value, top := modelcatalog.LevelValue, modelcatalog.LevelTop
	if _, err := svc.Update(ctx, "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposePhotoAnalysis, Level: &value,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Level: &top,
	}); err != nil {
		t.Fatal(err)
	}
	source, ok := svc.Lookup("openai/gpt-x")
	if !ok {
		t.Fatal("model missing from the registry view")
	}
	if got := source.Levels[llm.StageNameObserve]; got != string(modelcatalog.LevelValue) {
		t.Errorf("observe level = %q, want value", got)
	}
	if got := source.Levels[llm.StageNameWrite]; got != string(modelcatalog.LevelTop) {
		t.Errorf("write level = %q, want top", got)
	}
	// An untouched purpose carries no level at all rather than an empty one.
	if got, ok := source.Levels[llm.StageNameAnalyze]; ok {
		t.Errorf("analyze level = %q, want absent", got)
	}

	// Clearing is a real edit and removes the key rather than blanking it.
	cleared := modelcatalog.Level("")
	if _, err := svc.Update(ctx, "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Level: &cleared,
	}); err != nil {
		t.Fatal(err)
	}
	source, _ = svc.Lookup("openai/gpt-x")
	if got, ok := source.Levels[llm.StageNameWrite]; ok {
		t.Errorf("write level after clearing = %q, want absent", got)
	}
	if source.Levels[llm.StageNameObserve] != string(modelcatalog.LevelValue) {
		t.Errorf("clearing writing reached observe: %v", source.Levels)
	}
}

// T092/MODEL-58: the level has the enum gate and the registration gate, and nothing else —
// there is no model-side rule to check it against the way an effort has one.
func TestUpdate_RefusesAnUnknownLevel(t *testing.T) {
	svc := newService(t, newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting)), nil)
	ctx := context.Background()

	nonsense := modelcatalog.Level("legendary")
	if _, err := svc.Update(ctx, "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Level: &nonsense,
	}); !errors.Is(err, modelcatalog.ErrInvalidLevel) {
		t.Errorf("bad level = %v, want ErrInvalidLevel", err)
	}
	top := modelcatalog.LevelTop
	if _, err := svc.Update(ctx, "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposePhotoAnalysis, Level: &top,
	}); !errors.Is(err, modelcatalog.ErrPurposeNotRegistered) {
		t.Errorf("level on an unregistered purpose = %v, want ErrPurposeNotRegistered", err)
	}
}

// T092/MODEL-57: the browse list is read one tab at a time, so an entry reports the level
// of the purpose being listed — not another tab's, and not a merge of both.
func TestBrowse_ReportsTheListedPurposesLevel(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x",
		modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting))
	svc := newService(t, store, nil)
	ctx := context.Background()

	value, top := modelcatalog.LevelValue, modelcatalog.LevelTop
	if _, err := svc.Update(ctx, "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposePhotoAnalysis, Level: &value,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, "openai/gpt-x", modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Level: &top,
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		purpose modelcatalog.Purpose
		want    modelcatalog.Level
	}{
		{modelcatalog.PurposePhotoAnalysis, modelcatalog.LevelValue},
		{modelcatalog.PurposeWriting, modelcatalog.LevelTop},
		// Registered but never levelled: the tab shows the gap rather than a neighbour's grade.
		{modelcatalog.PurposeStyleAnalysis, ""},
	} {
		browse, err := svc.Browse(ctx, false, tc.purpose)
		if err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, entry := range browse.Entries {
			if entry.ModelID != "openai/gpt-x" {
				continue
			}
			found = true
			if entry.Level != tc.want {
				t.Errorf("%s tab level = %q, want %q", tc.purpose, entry.Level, tc.want)
			}
		}
		if !found {
			t.Fatalf("%s tab did not list the model", tc.purpose)
		}
	}
}
