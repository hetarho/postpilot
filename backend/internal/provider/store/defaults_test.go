package store_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
)

type raceCatalog struct{ automatic, manual llm.ModelInfo }

func (c raceCatalog) Models() []llm.ModelInfo { return []llm.ModelInfo{c.automatic, c.manual} }
func (c raceCatalog) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	for _, model := range c.Models() {
		if model.Ref == ref {
			return model, true
		}
	}
	return llm.ModelInfo{}, false
}

type raceCredits struct{}

func (raceCredits) Tier(context.Context, string) (plan.Plan, error) { return plan.Free, nil }
func (raceCredits) ForCalls([]provider.PlannedCall) int             { panic("unexpected model work") }
func (raceCredits) Balance(context.Context, string) (int, bool, error) {
	panic("unexpected credit work")
}

func TestDefaultInitializationAndManualSelectionRaceAcrossSQLiteHandles(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "defaults.db")
	first, err := db.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := db.Migrate(ctx, first.Writer); err != nil {
		t.Fatal(err)
	}
	second, err := db.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	store := providerstore.New(first.Writer, first.Reader)
	info := func(id string) llm.ModelInfo {
		return llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "p", ModelID: id}, Vision: true,
			Stages: []string{"observe", "write", "analyze"}, Levels: map[string]string{"observe": "free", "write": "free", "analyze": "free"},
			InputUSDPerMillion: "0", OutputUSDPerMillion: "0"}
	}
	catalog := raceCatalog{automatic: info("automatic"), manual: info("manual")}
	services := []*provider.Service{
		provider.NewService(store, catalog, raceCredits{}).WithModelGrades(),
		provider.NewService(providerstore.New(second.Writer, second.Reader), catalog, raceCredits{}).WithModelGrades(),
	}
	for iteration := range 24 {
		userID := fmt.Sprintf("owner-%d", iteration)
		if _, err := first.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?, 'hash', '2026-10-07T00:00:00Z')`, userID); err != nil {
			t.Fatal(err)
		}
		pair := []provider.Selection{
			{Stage: provider.StageWrite, Slot: provider.SlotCandidateA, Ref: catalog.automatic.Ref, UpdatedAt: time.Unix(1, 0)},
			{Stage: provider.StageWrite, Slot: provider.SlotCandidateB, Ref: catalog.manual.Ref, UpdatedAt: time.Unix(1, 0)},
		}
		if err := store.SaveSelections(ctx, userID, pair); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 3)
		for _, service := range services {
			go func() {
				<-start
				_, err := service.InitializeDefaultSelections(ctx, userID)
				results <- err
			}()
		}
		go func() {
			<-start
			_, err := services[iteration%2].SaveSelection(ctx, userID, provider.StageWrite, catalog.manual.Ref)
			results <- err
		}()
		close(start)
		for range 3 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		rows, err := store.ListSelectionSlots(ctx, userID)
		if err != nil || len(rows) != 5 {
			t.Fatalf("race result: %+v %v", rows, err)
		}
		for _, selection := range rows {
			if selection.Stage == provider.StageWrite && selection.Slot == provider.SlotActive && selection.Ref != catalog.manual.Ref {
				t.Fatalf("initializer overwrote the manual selection: %+v", rows)
			}
		}
		if _, err := services[0].InitializeDefaultSelections(ctx, userID); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"generation_jobs", "usage_admissions", "usage_events", "credit_hold_lots"} {
		var count int
		if err := first.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("initializer created %s: count=%d error=%v", table, count, err)
		}
	}
}

func TestInsertDefaultsRollsBackAndCannotChangeComparisonSlots(t *testing.T) {
	ctx := context.Background()
	store, handle := openRecommendationStore(t)
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice', 'hash', '2026-10-07T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	rows := []provider.Selection{
		{Stage: provider.StageWrite, Slot: provider.SlotActive, Ref: llm.ModelRef{ProviderID: "p", ModelID: "a"}, UpdatedAt: time.Unix(1, 0)},
		{Stage: provider.StageObserve, Slot: provider.SlotCandidateA, Ref: llm.ModelRef{ProviderID: "p", ModelID: "b"}, UpdatedAt: time.Unix(1, 0)},
	}
	if err := store.InsertDefaultSelections(ctx, "alice", rows); err == nil {
		t.Fatal("default writer accepted a comparison slot")
	}
	saved, err := store.ListSelectionSlots(ctx, "alice")
	if err != nil || len(saved) != 0 {
		t.Fatalf("failed defaults leaked a partial transaction: %+v %v", saved, err)
	}
}
