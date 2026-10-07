package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
)

type adoptionCatalog struct {
	model llm.ModelInfo
	reads int
}

func (c *adoptionCatalog) Models() []llm.ModelInfo { return []llm.ModelInfo{c.model} }
func (c *adoptionCatalog) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	c.reads++
	return c.model, ref == c.model.Ref
}

type lostAdoptionResponse struct {
	*providerstore.Store
	lose bool
}

func (s *lostAdoptionResponse) CommitTestModelAdoption(ctx context.Context, in provider.TestModelAdoption, at time.Time) (provider.TestModelReceipt, error) {
	receipt, err := s.Store.CommitTestModelAdoption(ctx, in, at)
	if err == nil && s.lose {
		s.lose = false
		return provider.TestModelReceipt{}, errors.New("response lost after commit")
	}
	return receipt, err
}

func TestModelAdoptionReceiptAndSelectionCommitAtomicallyAndRecoverWithoutReplay(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "adoption.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?, 'hash', '2026-10-07T00:00:00Z')`, user); err != nil {
			t.Fatal(err)
		}
	}
	store := providerstore.New(handle.Writer, handle.Reader)
	catalog := &adoptionCatalog{model: llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "p", ModelID: "winner"}, Stages: []string{"write"}, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"}}
	service := provider.NewService(store, catalog, raceCredits{}).WithModelGrades()
	response := &lostAdoptionResponse{Store: store}
	adoptions := provider.NewTestModelAdoptions(service, response)
	in := provider.TestModelAdoption{UserID: "alice", TestID: "test", WinnerID: "winner", RequestKey: "adopt", Fingerprint: "frozen", Stage: provider.StageWrite, Ref: catalog.model.Ref}
	manual := provider.Selection{Stage: provider.StageWrite, Ref: llm.ModelRef{ProviderID: "p", ModelID: "manual"}, UpdatedAt: time.Now()}
	if err := store.UpsertSelection(ctx, "alice", manual); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`CREATE TRIGGER fail_adoption_receipt BEFORE INSERT ON provider_test_publications BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := adoptions.AdoptTestModel(ctx, in); err == nil {
		t.Fatal("receipt failure accepted")
	}
	assertSelection := func(want llm.ModelRef) {
		t.Helper()
		rows, err := store.ListSelections(ctx, "alice")
		if err != nil || len(rows) != 1 || rows[0].Ref != want {
			t.Fatalf("selection=%+v err=%v want=%+v", rows, err, want)
		}
	}
	assertSelection(manual.Ref)
	if _, err := handle.Writer.Exec(`DROP TRIGGER fail_adoption_receipt`); err != nil {
		t.Fatal(err)
	}
	response.lose = true
	if _, err := adoptions.AdoptTestModel(ctx, in); err == nil {
		t.Fatal("lost response was not injected")
	}
	assertSelection(in.Ref)
	if err := store.UpsertSelection(ctx, "alice", manual); err != nil {
		t.Fatal(err)
	}
	// Recovery confirms the original action even after rights/catalog disappear.
	catalog.model.Disabled = true
	reads := catalog.reads
	receipt, err := adoptions.AdoptTestModel(ctx, in)
	if err != nil || receipt.RequestKey != in.RequestKey || receipt.Ref != in.Ref || catalog.reads != reads {
		t.Fatalf("receipt=%+v err=%v live reads=%d/%d", receipt, err, catalog.reads, reads)
	}
	if retained, found, err := adoptions.ReadTestPublicationReceipt(ctx, "alice", "test", "winner"); err != nil || !found || retained != receipt || catalog.reads != reads {
		t.Fatalf("payload-free model receipt=%+v found=%v err=%v", retained, found, err)
	}
	if _, found, err := adoptions.ReadTestPublicationReceipt(ctx, "bob", "test", "winner"); err != nil || found {
		t.Fatalf("foreign model receipt found=%v err=%v", found, err)
	}
	assertSelection(manual.Ref)
	for _, mutate := range []func(*provider.TestModelAdoption){
		func(v *provider.TestModelAdoption) { v.Fingerprint = "changed" },
		func(v *provider.TestModelAdoption) { v.WinnerID = "other" },
		func(v *provider.TestModelAdoption) { v.Ref = manual.Ref },
	} {
		conflict := in
		mutate(&conflict)
		if _, err := adoptions.AdoptTestModel(ctx, conflict); !errors.Is(err, provider.ErrTestPublicationConflict) {
			t.Fatalf("conflict=%+v err=%v", conflict, err)
		}
		assertSelection(manual.Ref)
	}
	foreign := in
	foreign.UserID = "bob"
	if _, err := adoptions.AdoptTestModel(ctx, foreign); !errors.Is(err, provider.ErrModelDisabled) {
		t.Fatalf("foreign action bypassed eligibility: %v", err)
	}
}

func TestModelAdoptionChecksCurrentEligibilityBeforeAnySelectionWrite(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "eligibility.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-10-07T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	store := providerstore.New(handle.Writer, handle.Reader)
	catalog := &adoptionCatalog{model: llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "p", ModelID: "winner"}, Stages: []string{"write"}, Levels: map[string]string{"write": "premium"}}}
	adoptions := provider.NewTestModelAdoptions(provider.NewService(store, catalog, raceCredits{}).WithModelGrades(), store)
	in := provider.TestModelAdoption{UserID: "alice", TestID: "test", WinnerID: "winner", RequestKey: "adopt", Fingerprint: "frozen", Stage: provider.StageWrite, Ref: catalog.model.Ref}
	if _, err := adoptions.AdoptTestModel(ctx, in); !errors.Is(err, provider.ErrModelPlanRequired) {
		t.Fatalf("locked model err=%v", err)
	}
	catalog.model.Stages = []string{"observe"}
	if _, err := adoptions.AdoptTestModel(ctx, in); !errors.Is(err, provider.ErrModelUnsuitable) {
		t.Fatalf("unregistered stage err=%v", err)
	}
	rows, err := store.ListSelections(ctx, "alice")
	if err != nil || len(rows) != 0 {
		t.Fatalf("refused adoption wrote choices: %+v %v", rows, err)
	}
}

func TestConcurrentModelAdoptionAcrossSQLiteHandlesReturnsOneReceipt(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "concurrent.db")
	first, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := db.Migrate(ctx, first.Writer); err != nil {
		t.Fatal(err)
	}
	second, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if _, err := first.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-10-07T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	info := func(id string) llm.ModelInfo {
		return llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "p", ModelID: id}, Stages: []string{"write"}, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"}
	}
	catalog := raceCatalog{automatic: info("winner"), manual: info("manual")}
	stores := []*providerstore.Store{providerstore.New(first.Writer, first.Reader), providerstore.New(second.Writer, second.Reader)}
	services := []*provider.Service{
		provider.NewService(stores[0], catalog, raceCredits{}).WithModelGrades(),
		provider.NewService(stores[1], catalog, raceCredits{}).WithModelGrades(),
	}
	adoptions := []*provider.TestModelAdoptionsService{provider.NewTestModelAdoptions(services[0], stores[0]), provider.NewTestModelAdoptions(services[1], stores[1])}
	in := provider.TestModelAdoption{UserID: "alice", TestID: "test", WinnerID: "winner", RequestKey: "adopt", Fingerprint: "frozen", Stage: provider.StageWrite, Ref: catalog.automatic.Ref}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	start := make(chan struct{})
	for i := range 12 {
		wg.Go(func() {
			<-start
			receipt, err := adoptions[i%2].AdoptTestModel(ctx, in)
			if err == nil && (receipt.Ref != in.Ref || receipt.RequestKey != in.RequestKey) {
				err = errors.New("different action receipt")
			}
			errs <- err
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var receipts int
	if err := first.Reader.QueryRow(`SELECT count(*) FROM provider_test_publications`).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("receipt count=%d err=%v", receipts, err)
	}
	if _, err := services[1].SaveSelection(ctx, "alice", provider.StageWrite, catalog.manual.Ref); err != nil {
		t.Fatal(err)
	}
	if _, err := adoptions[0].AdoptTestModel(ctx, in); err != nil {
		t.Fatal(err)
	}
	rows, err := stores[0].ListSelections(ctx, "alice")
	if err != nil || len(rows) != 1 || rows[0].Ref != catalog.manual.Ref {
		t.Fatalf("replay undid ordinary SaveSelection=%+v %v", rows, err)
	}
}
