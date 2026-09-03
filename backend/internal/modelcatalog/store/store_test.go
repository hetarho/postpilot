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
	// A11: the one shipped override. Losing it would replace the model's provider-controlled
	// adaptive thinking with the write stage's `low`.
	if sonnet.Reasoning != llm.ReasoningUnset {
		t.Errorf("sonnet reasoning = %q, want unset", sonnet.Reasoning)
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
	// Every other model defers to the stage policy.
	for id, row := range byID {
		if id != "anthropic/claude-sonnet-5" && row.Reasoning != llm.ReasoningUnspecified {
			t.Errorf("%s carries an unexpected reasoning override %q", id, row.Reasoning)
		}
	}
}

func TestUpsertAndGet_RoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	model := modelcatalog.Model{
		ModelID: "acme/new-1", ProviderSlug: "acme", Label: "New 1",
		Vision: true, StructuredOutput: true, ContextTokens: 4096,
		InputUSDPerMillion: "1.25", OutputUSDPerMillion: "4.25", PricingCheckedAt: "2026-09-01", Reasoning: llm.ReasoningHigh, ImageOutput: true, Listed: true,
		LastSeenAt: testNow, CreatedAt: testNow, UpdatedAt: testNow,
	}
	if err := s.Upsert(ctx, model); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "acme/new-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Label != "New 1" || got.Reasoning != llm.ReasoningHigh ||
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

	// An empty effort is a real value: it clears the override back to the stage policy.
	cleared := llm.ReasoningUnspecified
	updated, err := s.Patch(ctx, id, modelcatalog.Patch{Reasoning: &cleared}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Reasoning != llm.ReasoningUnspecified {
		t.Errorf("reasoning = %q, want cleared", updated.Reasoning)
	}
	if len(updated.Purposes) != 1 || updated.Purposes[0] != modelcatalog.PurposeWriting {
		t.Errorf("purposes = %v, a patch must not touch registrations", updated.Purposes)
	}

	if _, err := s.Patch(ctx, "nobody/has-this", modelcatalog.Patch{Reasoning: &cleared}, testNow); !errors.Is(err, modelcatalog.ErrNotFound) {
		t.Fatalf("patching an unknown model = %v, want ErrNotFound", err)
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
			// The curation the operator owns is untouched by a refresh.
			if row.Reasoning != llm.ReasoningUnset {
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
