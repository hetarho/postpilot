package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/postpilot/backend/internal/experiment"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
)

func publicationChampion(t *testing.T, store *experimentstore.Store, factor experiment.TestFactor) experiment.WritingTest {
	t.Helper()
	stage := experiment.Stage("")
	if factor == experiment.FactorModel {
		stage = experiment.StageWrite
	}
	request, plan := writingRequest(factor, stage, 2, "publication")
	found := admitWriting(t, store, request, plan)
	found = completeWriting(t, store, beginWriting(t, store, found))
	var err error
	found, err = store.DecideMatch(context.Background(), experiment.MatchDecision{TestMutation: experiment.TestMutation{UserID: found.UserID, TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "human-decision"}, MatchID: found.Matches[0].ID, WinnerCandidateID: found.Matches[0].LeftID})
	if err != nil {
		t.Fatal(err)
	}
	return found
}
func TestPublicationAdmissionIsConcurrentActionIdempotentWithImmutableExplicitChoices(t *testing.T) {
	store, handle := testStore(t)
	found := publicationChampion(t, store, experiment.FactorGuideline)
	request := experiment.WinnerPublication{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "publish"}, WinnerID: found.WinnerID, Action: "save_setting", Name: "Named winner", Scope: "fields", ScopeIDs: []string{"food", "travel"}}
	const n = 8
	results := make(chan experiment.TestPublication, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Go(func() { p, e := store.BeginPublication(context.Background(), request); results <- p; errs <- e })
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first experiment.TestPublication
	for result := range results {
		if first.ID == "" {
			first = result
		}
		if result != first {
			t.Fatalf("different publication=%+v/%+v", first, result)
		}
	}
	var count int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM writing_test_publications").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
	retry := request
	retry.RequestKey = "lost-original-key"
	retry.ExpectedRevision = 0
	retry.ScopeIDs = []string{"travel", "food"}
	if p, err := store.BeginPublication(context.Background(), retry); err != nil || p != first {
		t.Fatalf("lost-key recovery=%+v err=%v", p, err)
	}
	for _, change := range []func(*experiment.WinnerPublication){func(p *experiment.WinnerPublication) { p.Name = "Different" }, func(p *experiment.WinnerPublication) { p.Scope = "global"; p.ScopeIDs = nil }, func(p *experiment.WinnerPublication) { p.MakeDefault = true }, func(p *experiment.WinnerPublication) { p.WinnerID = "another" }, func(p *experiment.WinnerPublication) { p.Action = "use_setting" }} {
		bad := request
		change(&bad)
		if _, err := store.BeginPublication(context.Background(), bad); !errors.Is(err, experiment.ErrTestPublicationConflict) {
			t.Fatalf("changed choice accepted=%+v err=%v", bad, err)
		}
	}
	foreign := request
	foreign.UserID = "bob"
	if _, err := store.BeginPublication(context.Background(), foreign); !errors.Is(err, experiment.ErrTestNotFound) {
		t.Fatalf("foreign publication=%v", err)
	}
}
func TestPublicationConfirmationIsAtomicOwnerBoundAndRecoversAfterPayloadPurge(t *testing.T) {
	store, handle := testStore(t)
	found := publicationChampion(t, store, experiment.FactorTemplate)
	request := experiment.WinnerPublication{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "publish"}, WinnerID: found.WinnerID, Action: "save_setting", Name: "Named winner"}
	pending, err := store.BeginPublication(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	receipt := experiment.PublicationReceipt{UserID: "alice", TestID: found.ID, WinnerID: found.WinnerID, Action: "save_setting", RequestKey: "publish", TargetID: "durable-owned-setting"}
	for _, change := range []func(*experiment.PublicationReceipt){func(p *experiment.PublicationReceipt) { p.UserID = "bob" }, func(p *experiment.PublicationReceipt) { p.TestID = "other" }, func(p *experiment.PublicationReceipt) { p.WinnerID = "other" }, func(p *experiment.PublicationReceipt) { p.Action = "use_setting" }, func(p *experiment.PublicationReceipt) { p.RequestKey = "another" }, func(p *experiment.PublicationReceipt) { p.TargetID = "" }} {
		bad := receipt
		change(&bad)
		if _, err := store.ConfirmPublication(context.Background(), pending, bad); !errors.Is(err, experiment.ErrTestPublicationConflict) {
			t.Fatalf("forged receipt=%+v err=%v", bad, err)
		}
	}
	if _, err := handle.Writer.Exec("CREATE TRIGGER fail_writing_publication BEFORE UPDATE OF status ON writing_test_publications WHEN NEW.status='confirmed' BEGIN SELECT RAISE(ABORT,'receipt unavailable'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmPublication(context.Background(), pending, receipt); err == nil {
		t.Fatal("confirmation failure missing")
	}
	unchanged, _ := store.GetTest(context.Background(), "alice", found.ID)
	if len(unchanged.Publications) != 1 || unchanged.Publications[0].Status != "pending" {
		t.Fatalf("partial confirm=%+v", unchanged.Publications)
	}
	if _, err := handle.Writer.Exec("DROP TRIGGER fail_writing_publication"); err != nil {
		t.Fatal(err)
	}
	if err := store.PurgeWritingTestPost(context.Background(), "alice", "post-a"); err != nil {
		t.Fatal(err)
	}
	retry := request
	retry.RequestKey = "lost-key"
	retry.ExpectedRevision = 0
	recovered, err := store.BeginPublication(context.Background(), retry)
	if err != nil || recovered.ID != pending.ID || recovered.RequestKey != "publish" {
		t.Fatalf("purged pending=%+v err=%v", recovered, err)
	}
	confirmed, err := store.ConfirmPublication(context.Background(), recovered, receipt)
	if err != nil || confirmed.Status != "confirmed" || confirmed.TargetID != receipt.TargetID {
		t.Fatalf("confirmed=%+v err=%v", confirmed, err)
	}
	if replay, err := store.ConfirmPublication(context.Background(), pending, receipt); err != nil || replay != confirmed {
		t.Fatalf("confirmation replay=%+v err=%v", replay, err)
	}
	purged, _ := store.GetTest(context.Background(), "alice", found.ID)
	if purged.PurgeFence == 0 || len(purged.CommonSnapshot) != 0 {
		t.Fatal("confirmation restored private payload")
	}
	for _, c := range purged.Candidates {
		if len(c.Output) > 0 || len(c.FrozenVariant) > 0 {
			t.Fatal("confirmation restored candidate payload")
		}
	}
}
func TestPublicationNewAdmissionRejectsStaleAndPurgedTargetsWithoutIntents(t *testing.T) {
	for _, scenario := range []string{"stale", "purged", "uncompleted", "wrong factor"} {
		t.Run(scenario, func(t *testing.T) {
			store, handle := testStore(t)
			found := publicationChampion(t, store, experiment.FactorModel)
			request := experiment.WinnerPublication{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "publish"}, WinnerID: found.WinnerID, Action: "adopt_model"}
			switch scenario {
			case "stale":
				request.ExpectedRevision--
			case "purged":
				if err := store.PurgeWritingTestPost(context.Background(), "alice", "post-a"); err != nil {
					t.Fatal(err)
				}
				found, _ = store.GetTest(context.Background(), "alice", found.ID)
				request.ExpectedRevision = found.Revision
			case "uncompleted":
				if _, err := handle.Writer.Exec("UPDATE writing_tests SET status='review' WHERE id=?", found.ID); err != nil {
					t.Fatal(err)
				}
			case "wrong factor":
				request.Action = "save_setting"
			}
			if _, err := store.BeginPublication(context.Background(), request); err == nil {
				t.Fatal("invalid admitted")
			}
			var count int
			_ = handle.Reader.QueryRow("SELECT count(*) FROM writing_test_publications").Scan(&count)
			if count != 0 {
				t.Fatal("refused admission recorded target intent")
			}
		})
	}
}
