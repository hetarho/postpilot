package modelcatalog_test

import (
	"context"
	"errors"
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
	listErr   error
}

func newFakeStore(rows ...modelcatalog.Model) *fakeStore {
	s := &fakeStore{rows: map[string]modelcatalog.Model{}}
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
	if patch.Reasoning != nil {
		row.Reasoning = *patch.Reasoning
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
	row.UpdatedAt = at
	s.rows[modelID] = row
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

// A2: the browse list is the live catalog annotated with curation, PLUS the curated models
// the provider has stopped offering — the case that would otherwise silently disappear.
func TestBrowse_MergesLiveCatalogWithCuratedRows(t *testing.T) {
	store := newFakeStore(
		curated("openai/gpt-x", modelcatalog.PurposeWriting),
		curated("retired/model", modelcatalog.PurposeWriting),
	)
	upstream := &fakeUpstream{candidates: []modelcatalog.Candidate{
		candidate("openai/gpt-x", 200), candidate("anthropic/claude-y", 100), candidate("openai/gpt-w", 300),
	}}
	svc := newService(t, store, upstream)

	browse, err := svc.Browse(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) != 4 {
		t.Fatalf("entries = %d, want 3 offered plus 1 retired", len(browse.Entries))
	}
	// Grouped by vendor, newest first inside a vendor.
	want := []string{"anthropic/claude-y", "openai/gpt-w", "openai/gpt-x", "retired/model"}
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
	// The retired one keeps the snapshot it was last seen with, so the row still reads as a
	// model rather than an id.
	retired := byID["retired/model"]
	if !retired.Curated || retired.Listed || retired.Label != "retired/model" {
		t.Errorf("retired entry = %+v", retired)
	}
	if store.refreshes != 1 {
		t.Errorf("refreshes = %d, want one bookkeeping write for the live read", store.refreshes)
	}
}

// A7: an upstream failure degrades to curated rows and says so. It must not be treated as
// evidence that models disappeared.
func TestBrowse_DegradesWithoutWritingBookkeeping(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting))
	svc := newService(t, store, &fakeUpstream{err: errors.New("network down")})

	browse, err := svc.Browse(context.Background(), true)
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

	if _, err := svc.Browse(context.Background(), false); err != nil {
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

	browse, err := svc.Browse(context.Background(), false)
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
	row.Reasoning = llm.ReasoningHigh
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
	if model.Reasoning != llm.ReasoningHigh || model.Listed {
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

// A7/A11: the reasoning override still round-trips through an edit, shared across the
// model's purposes, and the registry view follows at once.
func TestUpdate_AppliesCurationToTheRegistryView(t *testing.T) {
	store := newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting))
	svc := newService(t, store, nil)

	high := llm.ReasoningHigh
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{Reasoning: &high}); err != nil {
		t.Fatal(err)
	}
	if source, _ := svc.Lookup("openai/gpt-x"); source.Reasoning != llm.ReasoningHigh {
		t.Errorf("reasoning = %q, want high", source.Reasoning)
	}
}

func TestUpdate_RefusesUnknownModelAndBadValues(t *testing.T) {
	svc := newService(t, newFakeStore(curated("openai/gpt-x", modelcatalog.PurposeWriting)), nil)

	none := llm.ReasoningNone
	if _, err := svc.Update(context.Background(), "nobody/has-this", modelcatalog.Patch{Reasoning: &none}); !errors.Is(err, modelcatalog.ErrNotFound) {
		t.Errorf("unknown model = %v, want ErrNotFound", err)
	}
	nonsense := llm.ReasoningEffort("enormous")
	if _, err := svc.Update(context.Background(), "openai/gpt-x", modelcatalog.Patch{Reasoning: &nonsense}); !errors.Is(err, modelcatalog.ErrInvalidReasoning) {
		t.Errorf("bad effort = %v, want ErrInvalidReasoning", err)
	}
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
	browse, err := svc.Browse(context.Background(), true)
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
