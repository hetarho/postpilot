package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	experimentapp "github.com/postpilot/backend/internal/experiment/app"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
)

func capturedSharedCheckpoint(t *testing.T) []byte {
	t.Helper()
	cp := generation.WritingTestCheckpoint{SnapshotHash: "hash", Index: -1, RequestInspections: []llm.RequestInspection{{Version: 1, Status: llm.InspectionCaptured, Stage: "observe", Mode: "post-observation", PromptVersion: "v1", SchemaVersion: "v1", CallID: "photo1", Output: llm.OutputContractInspection{Name: "observation", Version: "v1"}, Fragments: []llm.RequestFragment{{ID: "private-observer", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "private-observation", Text: "private issued observer prompt"}}}}}
	raw, err := json.Marshal(struct {
		Version    int                              `json:"version"`
		Checkpoint generation.WritingTestCheckpoint `json:"checkpoint"`
	}{1, cp})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func completeEvidenceTest(t *testing.T, store *experimentstore.Store) (experiment.WritingTest, experiment.TestExecutionWork) {
	t.Helper()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "evidence")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	if err := store.SaveTestCheckpoint(context.Background(), work.Fence, "shared", capturedSharedCheckpoint(t)); err != nil {
		t.Fatal(err)
	}
	ready := completeWriting(t, store, work)
	reader := experimentapp.NewWritingTestInspection(store)
	blind, err := reader.ReadTestRequestInspection(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, "write", llm.InspectionCaptured)
	if err != nil || blind.Status != llm.InspectionUnavailable || len(blind.Fragments) != 0 {
		t.Fatalf("active blind private evidence: %v %#v", err, blind)
	}
	match := ready.Matches[0]
	finished, err := store.DecideMatch(context.Background(), experiment.MatchDecision{TestMutation: experiment.TestMutation{UserID: "alice", TestID: ready.ID, RequestKey: "vote", ExpectedRevision: ready.Revision}, MatchID: match.ID, WinnerCandidateID: match.LeftID})
	if err != nil {
		t.Fatal(err)
	}
	if finished.ContentExpiresAt == nil || finished.ContentExpiresAt.Sub(finished.UpdatedAt) != 30*24*time.Hour {
		t.Fatal("private evidence retention changed")
	}
	return finished, work
}

func TestPrivateWritingEvidenceRetainsMatchingCallsAndPurgesTogether(t *testing.T) {
	for _, scope := range []string{"expiry", "source", "account"} {
		t.Run(scope, func(t *testing.T) {
			store, handle := testStore(t)
			found, work := completeEvidenceTest(t, store)
			reader := experimentapp.NewWritingTestInspection(store)
			capture, err := reader.ReadTestRequestInspection(context.Background(), "alice", found.ID, found.WinnerID, "write", llm.InspectionCaptured)
			if err != nil || capture.Status != llm.InspectionCaptured || capture.CallID != found.WinnerID {
				t.Fatalf("matching capture lost: %v %#v", err, capture)
			}
			observe, err := reader.ReadTestRequestInspection(context.Background(), "alice", found.ID, found.WinnerID, "observe", llm.InspectionCaptured)
			if err != nil || observe.Status != llm.InspectionCaptured || observe.CallID != "photo1" {
				t.Fatalf("shared capture lost: %v %#v", err, observe)
			}
			switch scope {
			case "expiry":
				if _, err := handle.Writer.Exec("UPDATE writing_tests SET content_expires_at=? WHERE id=?", time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano), found.ID); err != nil {
					t.Fatal(err)
				}
				absent, err := reader.ReadTestRequestInspection(context.Background(), "alice", found.ID, found.WinnerID, "write", llm.InspectionCaptured)
				if err != nil || absent.Status != llm.InspectionUnavailable {
					t.Fatalf("unswept expired evidence disclosed: %v %#v", err, absent)
				}
				if count, err := store.PurgeExpiredWritingTests(context.Background(), time.Now()); err != nil || count != 1 {
					t.Fatalf("expiry sweep: %d %v", count, err)
				}
			case "source":
				if err := store.PurgeWritingTestPost(context.Background(), "alice", "post-a"); err != nil {
					t.Fatal(err)
				}
			case "account":
				if _, err := handle.Writer.Exec("DELETE FROM users WHERE id=?", "alice"); err != nil {
					t.Fatal(err)
				}
			}
			retained, err := store.ReadTestInspectionWork(context.Background(), "alice", found.ID)
			if scope == "account" {
				if !errors.Is(err, experiment.ErrTestNotFound) {
					t.Fatalf("account private evidence survived: %v", err)
				}
			} else {
				if err != nil || retained.Test.PurgeFence == 0 || len(retained.SharedCheckpoint) > 0 || len(retained.Test.CommonSnapshot) > 0 || retained.Test.Input.Material != "" {
					t.Fatalf("private input/checkpoints survived: %v %#v", err, retained)
				}
				for _, candidate := range retained.Test.Candidates {
					if len(candidate.Output) > 0 || len(candidate.FrozenVariant) > 0 {
						t.Fatal("origin/request/output survived purge")
					}
				}
			}
			if err := store.SaveTestCheckpoint(context.Background(), work.Fence, "shared", capturedSharedCheckpoint(t)); err == nil {
				t.Fatal("late callback restored capture")
			}
			if err := store.CompleteTestCandidate(context.Background(), work.Fence, found.WinnerID, testWritingOutput(t, found.WinnerID), nil, nil); err == nil {
				t.Fatal("late callback restored origin/output")
			}
		})
	}
}

func TestPrivateWritingCaptureCompletionAndSourcePurgeRaceCannotRestorePayload(t *testing.T) {
	store, _ := testStore(t)
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "racing-evidence")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	checkpoint, output := capturedSharedCheckpoint(t), testWritingOutput(t, work.CandidateIDs[0])
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for _, action := range []func() error{func() error { return store.SaveTestCheckpoint(context.Background(), work.Fence, "shared", checkpoint) }, func() error {
		return store.CompleteTestCandidate(context.Background(), work.Fence, work.CandidateIDs[0], output, nil, nil)
	}, func() error { return store.PurgeWritingTestPost(context.Background(), "alice", "post-a") }} {
		wg.Add(1)
		go func(action func() error) { defer wg.Done(); errs <- action() }(action)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, experiment.ErrTestStateInvalid) {
			t.Fatal(err)
		}
	}
	retained, err := store.ReadTestInspectionWork(context.Background(), "alice", work.Fence.TestID)
	if err != nil || retained.Test.PurgeFence == 0 || len(retained.SharedCheckpoint) > 0 {
		t.Fatalf("capture purge lost: %v %#v", err, retained)
	}
	for _, candidate := range retained.Test.Candidates {
		if len(candidate.Output) != 0 {
			t.Fatal("output restored after capture purge")
		}
	}
}

type noOrdinaryPublicationJobs struct{}

func (noOrdinaryPublicationJobs) HasOrdinaryWrite(context.Context, *sql.Tx, string, string) (bool, error) {
	return false, nil
}

type blockingPayloadGuard struct {
	delegate         *experimentapp.TestPayloadGuard
	entered, release chan struct{}
}

func (g *blockingPayloadGuard) TestPayloadAvailable(ctx context.Context, tx *sql.Tx, user, test, winner string, fence uint64) (bool, error) {
	close(g.entered)
	<-g.release
	return g.delegate.TestPayloadAvailable(ctx, tx, user, test, winner, fence)
}

func TestPrivateChampionPublicationSerializesPurgeAndRecoversReceiptAfterDeletion(t *testing.T) {
	store, handle := testStore(t)
	finished, _ := completeEvidenceTest(t, store)
	posts := poststore.New(handle.Writer, handle.Reader)
	current, err := posts.GetPost(context.Background(), "post-a")
	if err != nil {
		t.Fatal(err)
	}
	var winner experiment.TestOutput
	for _, candidate := range finished.Candidates {
		if candidate.ID == finished.WinnerID {
			winner, err = experiment.DecodeTestOutput(candidate.Output)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	content := post.PostContent{Title: winner.Content.Title, Summary: winner.Content.Summary, Tags: winner.Content.Tags, Blocks: []post.Block{{Type: post.BlockText, Content: winner.Content.Blocks[0].Content}}}
	story := &post.Storyline{Paragraphs: []post.StorylineParagraph{{Text: winner.Storyline.Paragraphs[0].Text}}, Origins: winner.Storyline.Origins}
	in := post.TestOutputPublication{UserID: "alice", TestID: finished.ID, WinnerID: finished.WinnerID, RequestKey: "apply", PostSlug: "post-a", AssignmentsHash: post.TestAssignmentsHash(current), InputRevision: current.InputRevision, ContentRevision: current.ContentRevision, Content: content, Baseline: content, ContentLanguage: post.LanguageKorean, Origins: winner.Origins, Storyline: story, PrivatePayloadFence: &finished.PurgeFence}
	delegate := experimentapp.NewTestPayloadGuard(func(tx *sql.Tx) experimentapp.TestPayloadAvailability { return experimentstore.NewTx(tx) })
	guard := &blockingPayloadGuard{delegate: delegate, entered: make(chan struct{}), release: make(chan struct{})}
	publication := poststore.NewFencedTestResultStore(handle.Writer, handle.Reader, noOrdinaryPublicationJobs{}, guard)
	type publicationResult struct {
		receipt post.TestOutputReceipt
		err     error
	}
	results := make(chan publicationResult, 1)
	go func() {
		receipt, err := publication.ApplyTestResult(context.Background(), in, time.Now())
		results <- publicationResult{receipt, err}
	}()
	<-guard.entered
	purged := make(chan error, 1)
	go func() {
		purged <- store.PurgeWritingTestPost(context.Background(), "alice", "post-a")
	}()
	close(guard.release)
	result := <-results
	if result.err != nil {
		t.Fatalf("first publication: %v", result.err)
	}
	if err := <-purged; err != nil {
		t.Fatal(err)
	}
	retained, err := store.GetTest(context.Background(), "alice", finished.ID)
	if err != nil || retained.PurgeFence == 0 || len(retained.CommonSnapshot) != 0 {
		t.Fatalf("source purge failed: %v %#v", err, retained)
	}
	// The receipt precedes payload/target guards on retry. No second guarded call
	// reaches the blocking guard or restores result evidence after deletion.
	published, err := posts.GetPost(context.Background(), "post-a")
	if err != nil {
		t.Fatal(err)
	}
	manual := post.PostContent{Title: "Later manual", Blocks: []post.Block{{Type: post.BlockText, Content: "Later manual content"}}}
	if ok, err := posts.SaveContent(context.Background(), "post-a", "alice", manual, published.ContentRevision, time.Now()); err != nil || !ok {
		t.Fatalf("real manual edit: %v %v", ok, err)
	}
	again, err := publication.ApplyTestResult(context.Background(), in, time.Now())
	if err != nil || !reflect.DeepEqual(again, result.receipt) {
		t.Fatalf("lost receipt failed after purge: %v %#v", err, again)
	}
	after, err := posts.GetPost(context.Background(), "post-a")
	if err != nil || after.Content == nil || !reflect.DeepEqual(*after.Content, manual) {
		t.Fatalf("receipt replay undid later manual edit: %v %#v", err, after)
	}
	if ok, err := posts.DeletePost(context.Background(), "post-a", "alice"); err != nil || !ok {
		t.Fatalf("delete source: %v %v", ok, err)
	}
	again, err = publication.ApplyTestResult(context.Background(), in, time.Now())
	if err != nil || !reflect.DeepEqual(again, result.receipt) {
		t.Fatalf("deleted target receipt replay: %v %#v", err, again)
	}
	if _, err := posts.GetPost(context.Background(), "post-a"); !errors.Is(err, post.ErrNotFound) {
		t.Fatal("publication resurrected deleted source")
	}
	for _, candidate := range retained.Candidates {
		if len(candidate.Output) != 0 {
			t.Fatal("receipt replay restored private winner evidence")
		}
	}
	in.RequestKey, in.WinnerID = "other-operation", "uncommitted-candidate"
	fresh := poststore.NewFencedTestResultStore(handle.Writer, handle.Reader, noOrdinaryPublicationJobs{}, delegate)
	if _, err := fresh.ApplyTestResult(context.Background(), in, time.Now()); !errors.Is(err, post.ErrTestPublicationConflict) {
		t.Fatalf("new private publication after purge accepted: %v", err)
	}
}

func TestPrivateChampionPublicationRefusesPurgeOrExpiryAfterWinningPayloadRead(t *testing.T) {
	for _, action := range []string{"purge", "expiry"} {
		t.Run(action, func(t *testing.T) {
			store, handle := testStore(t)
			finished, _ := completeEvidenceTest(t, store)
			posts := poststore.New(handle.Writer, handle.Reader)
			current, err := posts.GetPost(context.Background(), "post-a")
			if err != nil {
				t.Fatal(err)
			}
			// The caller already holds the legitimate winning private payload.
			var winner experiment.TestOutput
			for _, candidate := range finished.Candidates {
				if candidate.ID == finished.WinnerID {
					winner, err = experiment.DecodeTestOutput(candidate.Output)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			content := post.PostContent{Title: winner.Content.Title, Summary: winner.Content.Summary, Tags: winner.Content.Tags, Blocks: []post.Block{{Type: post.BlockText, Content: winner.Content.Blocks[0].Content}}}
			in := post.TestOutputPublication{UserID: "alice", TestID: finished.ID, WinnerID: finished.WinnerID, RequestKey: "apply", PostSlug: "post-a", AssignmentsHash: post.TestAssignmentsHash(current), InputRevision: current.InputRevision, ContentRevision: current.ContentRevision, Content: content, Baseline: content, ContentLanguage: post.LanguageKorean, Origins: winner.Origins, PrivatePayloadFence: &finished.PurgeFence}
			if action == "purge" {
				if err := store.PurgeWritingTestPost(context.Background(), "alice", "post-a"); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := handle.Writer.Exec("UPDATE writing_tests SET content_expires_at=? WHERE id=?", time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano), finished.ID); err != nil {
					t.Fatal(err)
				}
			}
			guard := experimentapp.NewTestPayloadGuard(func(tx *sql.Tx) experimentapp.TestPayloadAvailability { return experimentstore.NewTx(tx) })
			publication := poststore.NewFencedTestResultStore(handle.Writer, handle.Reader, noOrdinaryPublicationJobs{}, guard)
			if _, err := publication.ApplyTestResult(context.Background(), in, time.Now()); !errors.Is(err, post.ErrTestPublicationConflict) {
				t.Fatalf("%s after winning read accepted: %v", action, err)
			}
			after, err := posts.GetPost(context.Background(), "post-a")
			if err != nil || !reflect.DeepEqual(after, current) {
				t.Fatalf("failed fence changed content/plan/baseline/origins: %v", err)
			}
			if _, exists, err := publication.ReadTestPublicationReceipt(context.Background(), "alice", finished.ID, finished.WinnerID); err != nil || exists {
				t.Fatalf("failed publication recorded receipt: %v %v", exists, err)
			}
		})
	}
}
