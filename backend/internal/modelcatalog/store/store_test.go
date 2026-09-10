package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/store"
	"github.com/postpilot/backend/internal/platform/db"
)

var testNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(handle.Writer, handle.Reader)
}

// A8: the migration carries the twelve models providers.yaml used to declare, with the
// facts a saved selection depends on. Without this the cutover would clear every user's
// choice through the vanished-selection rule.
func TestMigration_SeedsTheShippedCatalog(t *testing.T) {
	rows, err := newStore(t).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("seeded rows = %d, want 12", len(rows))
	}
	byID := map[string]modelcatalog.Model{}
	for _, row := range rows {
		byID[row.ModelID] = row
		if !row.Listed {
			t.Errorf("%s: listed=%v, want listed", row.ModelID, row.Listed)
		}
		// Change 20's interview decision: every purpose starts EMPTY. No seeded model is
		// registered anywhere until an operator checks it on a purpose tab.
		if len(row.Purposes) != 0 {
			t.Errorf("%s is seeded with purposes %v, want none", row.ModelID, row.Purposes)
		}
	}

	sonnet, ok := byID["anthropic/claude-sonnet-5"]
	if !ok {
		t.Fatal("claude-sonnet-5 is not seeded")
	}
	// Change 24's A4: migration 0021 CLEARS every override rather than copying it onto the
	// five registrations, so even the one shipped `unset` is gone and the model resolves to
	// the code-owned stage policy. Carrying today's values forward would have propagated a
	// blanket `minimal` — the setting measured breaking observation on 2026-09-03.
	if len(sonnet.Reasoning) != 0 {
		t.Errorf("sonnet reasoning = %v, want every override cleared by the migration", sonnet.Reasoning)
	}
	if sonnet.ProviderSlug != "anthropic" || !sonnet.Vision || !sonnet.StructuredOutput {
		t.Errorf("sonnet = %+v", sonnet)
	}

	free, ok := byID["openrouter/free"]
	if !ok {
		t.Fatal("the free router entry is not seeded")
	}
	if free.StructuredOutput || !free.Vision {
		t.Errorf("router entry = %+v", free)
	}
	// The text-only models must stay text-only: photo-analysis registration gates on this.
	if byID["deepseek/deepseek-v4-flash-0731"].Vision {
		t.Error("a text-only model was seeded with vision")
	}
	// Every model defers to the stage policy after the migration.
	for id, row := range byID {
		if len(row.Reasoning) != 0 {
			t.Errorf("%s carries an unexpected reasoning override %v", id, row.Reasoning)
		}
	}
}

func TestUpsertAndGet_RoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	model := modelcatalog.Model{
		ModelID: "acme/new-1", ProviderSlug: "acme", Label: "New 1",
		Vision: true, StructuredOutput: true, ContextTokens: 4096,
		InputUSDPerMillion: "1.25", OutputUSDPerMillion: "4.25", PricingCheckedAt: "2026-09-01", ImageOutput: true, Listed: true,
		LastSeenAt: testNow, CreatedAt: testNow, UpdatedAt: testNow,
	}
	if err := s.Upsert(ctx, model); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "acme/new-1")
	if err != nil {
		t.Fatal(err)
	}
	// The effort is no longer a catalog_models column: an Upsert carries only the snapshot
	// and availability halves, and a registration carries the curation (change 24).
	if got.Label != "New 1" || len(got.Reasoning) != 0 ||
		got.ContextTokens != 4096 || got.InputUSDPerMillion != "1.25" || !got.ImageOutput || got.VideoOutput || !got.Listed {
		t.Fatalf("round trip = %+v", got)
	}
	if !got.LastSeenAt.Equal(testNow) || !got.CreatedAt.Equal(testNow) {
		t.Errorf("timestamps = %v / %v", got.LastSeenAt, got.CreatedAt)
	}

	// A re-upsert keeps the moment the row entered the catalog.
	later := testNow.Add(48 * time.Hour)
	model.CreatedAt, model.UpdatedAt, model.Label = later, later, "New 1 renamed"
	if err := s.Upsert(ctx, model); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx, "acme/new-1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.CreatedAt.Equal(testNow) {
		t.Errorf("created_at = %v, want the original %v", got.CreatedAt, testNow)
	}
	if got.Label != "New 1 renamed" || !got.UpdatedAt.Equal(later) {
		t.Errorf("upsert did not refresh the row: %+v", got)
	}
}

func TestGet_UnknownModel(t *testing.T) {
	_, err := newStore(t).Get(context.Background(), "nobody/has-this")
	if !errors.Is(err, modelcatalog.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// A patch writes only what it names, so two operators editing different properties of one
// model do not overwrite each other — and purpose registrations are separate storage a
// patch can never touch.
func TestPatch_WritesOnlyWhatItNames(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := "anthropic/claude-sonnet-5"

	row, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterPurpose(ctx, row, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}

	// The effort lands on the REGISTRATION, so a patch names the purpose it applies to.
	high := llm.ReasoningHigh
	updated, err := s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Reasoning: &high}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Reasoning[modelcatalog.PurposeWriting] != llm.ReasoningHigh {
		t.Errorf("reasoning = %v, want high on writing", updated.Reasoning)
	}
	if len(updated.Purposes) != 1 || updated.Purposes[0] != modelcatalog.PurposeWriting {
		t.Errorf("purposes = %v, a patch must not touch registrations", updated.Purposes)
	}
	// An empty effort is a real value: it clears the override back to the stage policy.
	cleared := llm.ReasoningUnspecified
	updated, err = s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Reasoning: &cleared}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Reasoning) != 0 {
		t.Errorf("reasoning = %v, want cleared", updated.Reasoning)
	}

	if _, err := s.Patch(ctx, "nobody/has-this", modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Reasoning: &cleared}, testNow); !errors.Is(err, modelcatalog.ErrNotFound) {
		t.Fatalf("patching an unknown model = %v, want ErrNotFound", err)
	}
	// An effort on a purpose the model does not serve has nowhere to be read from, so the
	// UPDATE matching no join row IS the refusal.
	if _, err := s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeStyleAnalysis, Reasoning: &high}, testNow); !errors.Is(err, modelcatalog.ErrPurposeNotRegistered) {
		t.Fatalf("patching an unregistered purpose = %v, want ErrPurposeNotRegistered", err)
	}
}

// A1/A4: registrations round-trip per purpose, are idempotent, come back in display order,
// and removing one leaves the row and the other purposes standing.
func TestPurposes_RoundTripIndependently(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := "anthropic/claude-sonnet-5"

	row, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, purpose := range []modelcatalog.Purpose{
		modelcatalog.PurposeWriting, modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting,
	} {
		if err := s.RegisterPurpose(ctx, row, purpose); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := []modelcatalog.Purpose{modelcatalog.PurposePhotoAnalysis, modelcatalog.PurposeWriting}
	if len(got.Purposes) != 2 || got.Purposes[0] != want[0] || got.Purposes[1] != want[1] {
		t.Fatalf("purposes = %v, want %v in display order", got.Purposes, want)
	}

	stamp := testNow.Add(24 * time.Hour)
	if err := s.DeregisterPurpose(ctx, id, modelcatalog.PurposePhotoAnalysis, stamp); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Purposes) != 1 || got.Purposes[0] != modelcatalog.PurposeWriting {
		t.Fatalf("purposes after remove = %v", got.Purposes)
	}
	// A deregistration is a curation edit: the row's updated_at moves with it.
	if !got.UpdatedAt.Equal(stamp) {
		t.Errorf("updated_at = %v, want the deregistration stamp %v", got.UpdatedAt, stamp)
	}

	// List assembles the same registrations across all rows.
	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ModelID == id {
			if len(row.Purposes) != 1 || row.Purposes[0] != modelcatalog.PurposeWriting {
				t.Fatalf("listed purposes = %v", row.Purposes)
			}
		} else if len(row.Purposes) != 0 {
			t.Errorf("%s has purposes %v, want none", row.ModelID, row.Purposes)
		}
	}
}

// A6: a successful read refreshes the snapshot of what it saw and unlists the rest. It
// never disables anything — that stays an operator decision.
func TestRefreshAvailability_MarksSeenAndUnlistsTheRest(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	at := testNow.Add(72 * time.Hour)

	seen := []modelcatalog.Candidate{{
		ModelID: "anthropic/claude-sonnet-5", ProviderSlug: "anthropic", Label: "Claude Sonnet 5.1",
		Vision: true, StructuredOutput: true, ImageOutput: true, ContextTokens: 2_000_000,
		InputUSDPerMillion: "3", OutputUSDPerMillion: "15",
	}, {
		// Not curated: a candidate with no row must not create one.
		ModelID: "acme/unknown", ProviderSlug: "acme", Label: "Unknown",
	}}
	if err := s.RefreshAvailability(ctx, seen, at); err != nil {
		t.Fatal(err)
	}

	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("rows = %d, want the seeded 12 with nothing added", len(rows))
	}
	for _, row := range rows {
		switch row.ModelID {
		case "anthropic/claude-sonnet-5":
			if !row.Listed || !row.LastSeenAt.Equal(at) {
				t.Errorf("seen model: listed=%v last_seen=%v", row.Listed, row.LastSeenAt)
			}
			if row.Label != "Claude Sonnet 5.1" || row.ContextTokens != 2_000_000 || row.InputUSDPerMillion != "3" {
				t.Errorf("snapshot was not refreshed: %+v", row)
			}
			if row.PricingCheckedAt != at.UTC().Format(time.DateOnly) {
				t.Errorf("pricing_checked_at = %q, want the fetch date", row.PricingCheckedAt)
			}
			// The curation the operator owns is untouched by a refresh — and after the
			// migration there is none to change, which is A4.
			if len(row.Reasoning) != 0 {
				t.Errorf("a refresh changed curation: %+v", row)
			}
			if !row.ImageOutput {
				t.Errorf("output modalities were not refreshed: %+v", row)
			}
		default:
			if row.Listed {
				t.Errorf("%s was not unlisted", row.ModelID)
			}
		}
	}
}

// MODEL-20: a registered model the read did not see loses every registration in the same
// transaction — and with it the effort override, which belongs to the registration
// (MODEL-7), exactly as an operator's uncheck does. The row survives, `listed = 0`, with a
// curation stamp, so the model is offered again if the source lists it again.
func TestRefreshAvailability_DeregistersWhatItDidNotSee(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	gone := modelcatalog.Model{
		ModelID: "acme/gone", ProviderSlug: "acme", Label: "Gone", Listed: true,
		CreatedAt: testNow, UpdatedAt: testNow,
	}
	for _, purpose := range []modelcatalog.Purpose{modelcatalog.PurposeWriting, modelcatalog.PurposeStyleAnalysis} {
		if err := s.RegisterPurpose(ctx, gone, purpose); err != nil {
			t.Fatal(err)
		}
	}
	effort := llm.ReasoningLow
	if _, err := s.Patch(ctx, gone.ModelID, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Reasoning: &effort}, testNow); err != nil {
		t.Fatal(err)
	}
	// A registered row the read DID see, and an unregistered unseen row: neither is stamped.
	kept := modelcatalog.Model{
		ModelID: "acme/kept", ProviderSlug: "acme", Label: "Kept", Listed: true,
		CreatedAt: testNow, UpdatedAt: testNow,
	}
	if err := s.RegisterPurpose(ctx, kept, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}

	at := testNow.Add(48 * time.Hour)
	seen := []modelcatalog.Candidate{{ModelID: "acme/kept", ProviderSlug: "acme", Label: "Kept"}}
	if err := s.RefreshAvailability(ctx, seen, at); err != nil {
		t.Fatal(err)
	}

	row, err := s.Get(ctx, gone.ModelID)
	if err != nil {
		t.Fatalf("the delisted row was removed: %v", err)
	}
	if row.Listed || len(row.Purposes) != 0 {
		t.Errorf("delisted row: listed=%v purposes=%v, want unlisted with none", row.Listed, row.Purposes)
	}
	if !row.UpdatedAt.Equal(at) {
		t.Errorf("updated_at = %v, want the refresh time: losing a registration is a curation change", row.UpdatedAt)
	}
	if len(row.Reasoning) != 0 {
		t.Errorf("effort override = %v, want none: it belongs to the registration that was removed", row.Reasoning)
	}

	still, err := s.Get(ctx, kept.ModelID)
	if err != nil {
		t.Fatal(err)
	}
	if !still.Listed || len(still.Purposes) != 1 || !still.UpdatedAt.Equal(testNow) {
		t.Errorf("seen row was touched: listed=%v purposes=%v updated=%v", still.Listed, still.Purposes, still.UpdatedAt)
	}
	// The seeded rows had no registrations to lose, so the sweep must not have stamped them.
	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ModelID != gone.ModelID && r.ModelID != kept.ModelID && r.UpdatedAt.Equal(at) {
			t.Errorf("%s was stamped without losing a registration", r.ModelID)
		}
	}
}

// A1/A2: the six capability fields survive a round trip, and the source's descending effort
// order survives the comma-joined storage form.
func TestReasoningCapability_RoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	capability := modelcatalog.ReasoningCapability{
		Reasons: true, Efforts: []string{"max", "high", "low"}, DefaultEffort: "high",
		Mandatory: false, NativeEffort: true, MaxTokens: false,
	}
	err := s.Upsert(ctx, modelcatalog.Model{
		ModelID: "deepseek/deepseek-v4-pro-0813", ProviderSlug: "deepseek", Label: "V4 Pro",
		ReasoningCapability: capability, Listed: true, CreatedAt: testNow, UpdatedAt: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := s.Get(ctx, "deepseek/deepseek-v4-pro-0813")
	if err != nil {
		t.Fatal(err)
	}
	if !row.Reasons || row.Mandatory || !row.NativeEffort || row.MaxTokens {
		t.Fatalf("flags = %+v", row.ReasoningCapability)
	}
	if row.DefaultEffort != "high" {
		t.Fatalf("default effort = %q", row.DefaultEffort)
	}
	want := []string{"max", "high", "low"}
	for i := range want {
		if i >= len(row.Efforts) || row.Efforts[i] != want[i] {
			t.Fatalf("efforts = %v, want %v in that order", row.Efforts, want)
		}
	}

	// An empty list reads back as nil, not as one blank value: the storage form is a joined
	// string, and "" must stay UNKNOWN rather than becoming a one-element list.
	err = s.Upsert(ctx, modelcatalog.Model{
		ModelID: "vendor/unknown-list", ProviderSlug: "vendor", Label: "Unknown",
		Listed: true, CreatedAt: testNow, UpdatedAt: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	blank, err := s.Get(ctx, "vendor/unknown-list")
	if err != nil {
		t.Fatal(err)
	}
	if len(blank.Efforts) != 0 || blank.Reasons {
		t.Fatalf("unwritten capability = %+v", blank.ReasoningCapability)
	}
	if blank.Known() {
		t.Fatal("a row nothing was written to claims a known capability")
	}

	// The column's whole claim is that it holds the source's values VERBATIM, so the storage
	// form has to survive a value containing the delimiter a naive join would have used.
	err = s.Upsert(ctx, modelcatalog.Model{
		ModelID: "vendor/awkward", ProviderSlug: "vendor", Label: "Awkward",
		ReasoningCapability: modelcatalog.ReasoningCapability{
			Reasons: true, Efforts: []string{"very,high", "low"},
		},
		Listed: true, CreatedAt: testNow, UpdatedAt: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	awkward, err := s.Get(ctx, "vendor/awkward")
	if err != nil {
		t.Fatal(err)
	}
	if len(awkward.Efforts) != 2 || awkward.Efforts[0] != "very,high" || awkward.Efforts[1] != "low" {
		t.Fatalf("efforts = %v, want the source's two values intact", awkward.Efforts)
	}
}

// A1: a successful refresh writes the capability exactly as it writes the label and pricing,
// and A7: it does not touch the operator's override.
func TestRefreshAvailability_RefreshesTheReasoningCapability(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	const modelID = "anthropic/claude-sonnet-5"
	if err := s.RegisterPurpose(ctx, modelcatalog.Model{
		ModelID: modelID, ProviderSlug: "anthropic", Label: "Claude", Listed: true,
		CreatedAt: testNow, UpdatedAt: testNow,
	}, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}
	// An override the incoming list will NOT contain: it must survive the refresh unchanged.
	effort := llm.ReasoningEffort("medium")
	if _, err := s.Patch(ctx, modelID, modelcatalog.Patch{
		Purpose: modelcatalog.PurposeWriting, Reasoning: &effort,
	}, testNow); err != nil {
		t.Fatal(err)
	}

	at := testNow.Add(24 * time.Hour)
	err := s.RefreshAvailability(ctx, []modelcatalog.Candidate{{
		ModelID: modelID, ProviderSlug: "anthropic", Label: "Claude Sonnet 5.1",
		ReasoningCapability: modelcatalog.ReasoningCapability{
			Reasons: true, Efforts: []string{"max", "high", "low"}, DefaultEffort: "high",
			NativeEffort: true,
		},
	}}, at)
	if err != nil {
		t.Fatal(err)
	}
	row, err := s.Get(ctx, modelID)
	if err != nil {
		t.Fatal(err)
	}
	if !row.Reasons || row.DefaultEffort != "high" || len(row.Efforts) != 3 || !row.NativeEffort {
		t.Fatalf("the refresh did not write the capability: %+v", row.ReasoningCapability)
	}
	if row.Reasoning[modelcatalog.PurposeWriting] != "medium" {
		t.Fatalf("the refresh rewrote the operator's override: %+v", row.Reasoning)
	}
	if !row.DriftedFrom(row.Reasoning[modelcatalog.PurposeWriting]) {
		t.Fatal("an override outside the new list is not reported as drifted")
	}
}

// MODEL-53: a document is applied whole or not at all. The failure is forced with the
// purpose CHECK constraint the schema already carries, so the rollback is the database's
// own — the point is that the registration written before it does not survive.
func TestSyncPurposes_AppliesEverythingOrNothing(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	row, err := s.Get(ctx, "anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	err = s.SyncPurposes(ctx, []modelcatalog.PurposeWrite{
		{Model: row, Purpose: modelcatalog.PurposeWriting, Register: true},
		{Model: row, Purpose: modelcatalog.Purpose("audio-analysis"), Register: true},
	}, testNow)
	if err == nil {
		t.Fatal("want the invalid write to fail the sync")
	}

	after, err := s.Get(ctx, "anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Purposes) != 0 {
		t.Errorf("purposes = %v, want none — the first write must have rolled back with the second", after.Purposes)
	}
}

func TestSyncPurposes_RegistersAndDeregistersInOnePass(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	sonnet, err := s.Get(ctx, "anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterPurpose(ctx, sonnet, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}
	// An effort on the registration, to prove the deregistration takes it along.
	effort := llm.ReasoningLow
	if _, err := s.Patch(ctx, sonnet.ModelID, modelcatalog.Patch{
		Reasoning: &effort, Purpose: modelcatalog.PurposeWriting,
	}, testNow); err != nil {
		t.Fatal(err)
	}
	grok, err := s.Get(ctx, "x-ai/grok-4.6")
	if err != nil {
		t.Skipf("seed does not carry x-ai/grok-4.6: %v", err)
	}

	err = s.SyncPurposes(ctx, []modelcatalog.PurposeWrite{
		{Model: grok, Purpose: modelcatalog.PurposeWriting, Register: true},
		{Model: modelcatalog.Model{ModelID: sonnet.ModelID}, Purpose: modelcatalog.PurposeWriting},
	}, testNow)
	if err != nil {
		t.Fatal(err)
	}

	after, err := s.Get(ctx, sonnet.ModelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Purposes) != 0 {
		t.Errorf("purposes = %v, want the registration gone", after.Purposes)
	}
	if got, ok := after.Reasoning[modelcatalog.PurposeWriting]; ok {
		t.Errorf("effort = %q, want it removed with the registration row", got)
	}
	registered, err := s.Get(ctx, grok.ModelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(registered.Purposes) != 1 || registered.Purposes[0] != modelcatalog.PurposeWriting {
		t.Errorf("purposes = %v, want writing", registered.Purposes)
	}
}

// T092/MODEL-57: the level rides the registration exactly as the effort does — set per
// purpose, cleared by an empty value, refused for a purpose the model does not serve.
func TestPatch_LevelRidesTheRegistration(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := "anthropic/claude-sonnet-5"

	row, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, purpose := range []modelcatalog.Purpose{modelcatalog.PurposeWriting, modelcatalog.PurposePhotoAnalysis} {
		if err := s.RegisterPurpose(ctx, row, purpose); err != nil {
			t.Fatal(err)
		}
	}
	// A fresh registration starts unset — nothing is derived from price or capability.
	if fresh, err := s.Get(ctx, id); err != nil {
		t.Fatal(err)
	} else if len(fresh.Levels) != 0 {
		t.Fatalf("levels on a fresh registration = %v, want none", fresh.Levels)
	}

	top := modelcatalog.LevelTop
	updated, err := s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Level: &top}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Levels[modelcatalog.PurposeWriting] != modelcatalog.LevelTop {
		t.Errorf("levels = %v, want top on writing", updated.Levels)
	}
	// Per REGISTRATION: the same model is a different bargain for another task.
	value := modelcatalog.LevelValue
	if _, err := s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposePhotoAnalysis, Level: &value}, testNow); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Levels[modelcatalog.PurposeWriting] != modelcatalog.LevelTop ||
		got.Levels[modelcatalog.PurposePhotoAnalysis] != modelcatalog.LevelValue {
		t.Fatalf("levels = %v, want top on writing and value on photo-analysis", got.Levels)
	}

	// A patch that names only the effort leaves the level exactly where it was.
	high := llm.ReasoningHigh
	updated, err = s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Reasoning: &high}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Levels[modelcatalog.PurposeWriting] != modelcatalog.LevelTop {
		t.Errorf("levels after an effort-only patch = %v, want top kept", updated.Levels)
	}

	// An empty level is a real request: it clears back to unset.
	cleared := modelcatalog.Level("")
	updated, err = s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Level: &cleared}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updated.Levels[modelcatalog.PurposeWriting]; ok {
		t.Errorf("levels = %v, want writing cleared", updated.Levels)
	}
	if updated.Levels[modelcatalog.PurposePhotoAnalysis] != modelcatalog.LevelValue {
		t.Errorf("clearing one purpose changed another: %v", updated.Levels)
	}

	if _, err := s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeStyleAnalysis, Level: &top}, testNow); !errors.Is(err, modelcatalog.ErrPurposeNotRegistered) {
		t.Fatalf("level on an unregistered purpose = %v, want ErrPurposeNotRegistered", err)
	}
}

// T092/MODEL-20: the level is registration-bound, so every path that drops a registration
// drops it too — an uncheck, and the delisting a refresh performs. Re-registering starts
// unset rather than resurrecting a stale grade.
func TestLevel_GoesWithTheRegistration(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := "anthropic/claude-sonnet-5"

	row, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterPurpose(ctx, row, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}
	premium := modelcatalog.LevelPremium
	if _, err := s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Level: &premium}, testNow); err != nil {
		t.Fatal(err)
	}
	// Re-registering an already-registered purpose keeps it: INSERT OR IGNORE only
	// refreshes the row snapshot.
	if err := s.RegisterPurpose(ctx, row, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(ctx, id); err != nil {
		t.Fatal(err)
	} else if got.Levels[modelcatalog.PurposeWriting] != modelcatalog.LevelPremium {
		t.Fatalf("levels after a re-register = %v, want premium kept", got.Levels)
	}

	if err := s.DeregisterPurpose(ctx, id, modelcatalog.PurposeWriting, testNow); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterPurpose(ctx, row, modelcatalog.PurposeWriting); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Levels) != 0 {
		t.Fatalf("levels after uncheck and re-register = %v, want unset", got.Levels)
	}

	// The delisting a successful refresh performs is the same deregistration (MODEL-20).
	if _, err := s.Patch(ctx, id, modelcatalog.Patch{Purpose: modelcatalog.PurposeWriting, Level: &premium}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAvailability(ctx, nil, testNow); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Purposes) != 0 || len(got.Levels) != 0 {
		t.Fatalf("after delisting purposes = %v levels = %v, want both gone", got.Purposes, got.Levels)
	}
}
