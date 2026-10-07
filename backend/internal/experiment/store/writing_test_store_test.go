package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
)

func writingRequest(factor experiment.TestFactor, stage experiment.Stage, count int, key string) (experiment.TestStart, experiment.TestPlan) {
	request := experiment.TestStart{UserID: "alice", RequestKey: key, QuoteKey: key + "-quote", Factor: factor, ModelStage: stage, Count: count, Input: experiment.TestInput{SourcePostSlug: "post-a", TargetLanguage: "ko", Material: "Private common material", InputRevision: 4, ContentRevision: 9, Fictional: true}}
	plan := experiment.TestPlan{Snapshot: experiment.TestSnapshot{Common: []byte(`{"private":"frozen common"}`), Hash: "hash", PromptVersion: "full-v1", AssignmentsHash: "assignments"}, EstimateCredits: count * 7}
	for i := 0; i < count; i++ {
		ref := experiment.TestEntrantRef{SourceKind: "setting", SettingKind: string(factor), SettingID: fmt.Sprintf("setting-%d", i), SettingRevision: "1"}
		if factor == experiment.FactorModel {
			ref = experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "p", ModelID: fmt.Sprintf("m-%d", i)}}
		}
		request.Entrants = append(request.Entrants, ref)
		plan.Snapshot.Variants = append(plan.Snapshot.Variants, experiment.FrozenTestVariant{Reference: ref, Content: []byte(fmt.Sprintf(`{"entrant":%d}`, i)), SemanticKey: fmt.Sprintf("semantic-%d", i), Revision: "1", Label: fmt.Sprintf("Frozen label %d", i), Synthetic: factor != experiment.FactorModel})
		plan.Calls = append(plan.Calls, experiment.TestCall{Ref: experiment.ModelRef{ProviderID: "p", ModelID: fmt.Sprintf("m-%d", i)}, Stage: experiment.StageWrite, Count: 1, PromptTokens: 100, CompletionTokens: 200})
	}
	return request, plan
}
func admitWriting(t *testing.T, store *experimentstore.Store, request experiment.TestStart, plan experiment.TestPlan) experiment.WritingTest {
	t.Helper()
	now := time.Now().UTC()
	quote := experiment.TestQuote{Key: request.QuoteKey, UserID: request.UserID, Fingerprint: experiment.WritingTestStartFingerprint(request), Start: request, Plan: plan, Credits: plan.EstimateCredits, CreatedAt: now, ExpiresAt: now.Add(time.Hour), Retention: 30 * 24 * time.Hour}
	if err := store.PutTestQuote(context.Background(), quote); err != nil {
		t.Fatal(err)
	}
	found, fresh, err := store.AdmitTest(context.Background(), request, plan)
	if err != nil || !fresh {
		t.Fatalf("admit fresh=%v: %v", fresh, err)
	}
	return found
}
func beginWriting(t *testing.T, store *experimentstore.Store, found experiment.WritingTest) experiment.TestExecutionWork {
	t.Helper()
	ctx := context.Background()
	work, err := store.PreparedTestWork(ctx, found.UserID, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	if work.Fence.JobID == "" {
		t.Fatal("job identity missing before queue admission")
	}
	if _, err = store.BindTestJob(ctx, work.Fence, work.Fence.JobID); err != nil {
		t.Fatal(err)
	}
	started, err := store.BeginTestExecution(ctx, work.Fence)
	if err != nil {
		t.Fatal(err)
	}
	return started
}
func testWritingOutput(t *testing.T, id string) []byte {
	t.Helper()
	raw, err := experiment.EncodeTestOutput(experiment.TestOutput{ContentLanguage: "ko", Content: experiment.TestOutputContent{Title: "Complete " + id, Summary: "Summary", Tags: []string{"tag"}, Blocks: []experiment.TestOutputBlock{{Type: "TEXT", Content: "A complete generated post."}}}, Storyline: &experiment.TestOutputStoryline{Paragraphs: []experiment.TestOutputParagraph{{Text: "A complete generated post."}}}, Nouns: []string{"post"}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func completeWriting(t *testing.T, store *experimentstore.Store, work experiment.TestExecutionWork) experiment.WritingTest {
	t.Helper()
	ctx := context.Background()
	for _, id := range work.CandidateIDs {
		raw := testWritingOutput(t, id)
		accounting, _ := json.Marshal(experiment.Usage{PromptTokens: 12, CompletionTokens: 23, CostMicrousd: 999, CostSource: experiment.CostReported, LatencyMS: 500})
		if err := store.CompleteTestCandidate(ctx, work.Fence, id, raw, accounting, nil); err != nil {
			t.Fatal(err)
		}
	}
	found, err := store.FinishTestExecution(ctx, work.Fence, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestWritingTestEveryFormatAndFactorRequiresExactlyNMinusOneDecisions(t *testing.T) {
	axes := []struct {
		factor experiment.TestFactor
		stage  experiment.Stage
	}{{experiment.FactorModel, experiment.StageObserve}, {experiment.FactorModel, experiment.StageWrite}, {experiment.FactorVoice, ""}, {experiment.FactorTemplate, ""}, {experiment.FactorGuideline, ""}}
	for _, axis := range axes {
		for _, count := range []int{2, 4, 8, 16} {
			t.Run(fmt.Sprintf("%s-%s-%d", axis.factor, axis.stage, count), func(t *testing.T) {
				store, _ := testStore(t)
				ctx := context.Background()
				request, plan := writingRequest(axis.factor, axis.stage, count, "start")
				admitted := admitWriting(t, store, request, plan)
				replayed, fresh, err := store.AdmitTest(ctx, request, experiment.TestPlan{})
				if err != nil || fresh || !reflect.DeepEqual(admitted.Candidates, replayed.Candidates) {
					t.Fatalf("seeded replay changed: fresh=%v err=%v", fresh, err)
				}
				work := beginWriting(t, store, admitted)
				if _, err = store.BeginTestExecution(ctx, work.Fence); !errors.Is(err, experiment.ErrTestRunning) {
					t.Fatalf("concurrent job claim=%v", err)
				}
				found := completeWriting(t, store, work)
				if found.Status != experiment.TestReview || len(found.Matches) != count/2 {
					t.Fatalf("barrier %+v", found)
				}
				initialMatches := slices.Clone(found.Matches)
				reloaded, err := store.GetTest(ctx, "alice", found.ID)
				if err != nil || !reflect.DeepEqual(initialMatches, reloaded.Matches) {
					t.Fatalf("unstable matches=%v", err)
				}
				decisions := 0
				for found.Status != experiment.TestCompleted {
					var match experiment.TestMatch
					for _, m := range found.Matches {
						if m.WinnerID == "" {
							match = m
							break
						}
					}
					if match.ID == "" {
						t.Fatal("bracket stuck")
					}
					vote := experiment.MatchDecision{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, RequestKey: fmt.Sprintf("vote-%d", decisions), ExpectedRevision: found.Revision}, MatchID: match.ID, WinnerCandidateID: match.LeftID}
					found, err = store.DecideMatch(ctx, vote)
					if err != nil {
						t.Fatal(err)
					}
					again, err := store.DecideMatch(ctx, vote)
					if err != nil || again.Revision != found.Revision || len(again.Matches) != len(found.Matches) {
						t.Fatalf("vote replay %v", err)
					}
					conflict := vote
					conflict.WinnerCandidateID = match.RightID
					if _, err = store.DecideMatch(ctx, conflict); !errors.Is(err, experiment.ErrTestDecisionConflict) {
						t.Fatalf("conflicting replay %v", err)
					}
					decisions++
				}
				if decisions != count-1 || len(found.Matches) != count-1 || found.WinnerID == "" || found.ContentExpiresAt == nil {
					t.Fatalf("champion decisions=%d matches=%d", decisions, len(found.Matches))
				}
				if len(found.Publications) != 0 || found.ConfirmedCredits != 0 || found.Input.Material != request.Input.Material {
					t.Fatal("decision mutated source or metered calls")
				}
				if err = store.ConfirmTestSettlement(ctx, work.Fence, count*3); err != nil {
					t.Fatal(err)
				}
				if err = store.ConfirmTestSettlement(ctx, work.Fence, count*3); err != nil {
					t.Fatal(err)
				}
				settled, _ := store.GetTest(ctx, "alice", found.ID)
				if settled.ConfirmedCredits != count*3 || settled.ReservedCredits != 0 {
					t.Fatalf("settlement %+v", settled)
				}
			})
		}
	}
}
func TestWritingTestOwnerQuoteAndRequestIsolation(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 4, "start")
	found := admitWriting(t, store, request, plan)
	for _, user := range []string{"bob", "unknown"} {
		if _, err := store.GetTest(ctx, user, found.ID); !errors.Is(err, experiment.ErrTestNotFound) {
			t.Fatalf("foreign=%v", err)
		}
		if _, err := store.GetTestQuote(ctx, user, request.QuoteKey); !errors.Is(err, experiment.ErrTestNotFound) {
			t.Fatalf("foreignquote=%v", err)
		}
	}
	changed := request
	changed.Input.Material = "changed"
	if _, _, err := store.AdmitTest(ctx, changed, plan); !errors.Is(err, experiment.ErrTestOperation) {
		t.Fatalf("request replay forgery=%v", err)
	}
	changed = request
	changed.RequestKey = "second"
	if _, _, err := store.AdmitTest(ctx, changed, plan); !errors.Is(err, experiment.ErrTestQuoteRequired) {
		t.Fatalf("quote reuse=%v", err)
	}
	foreign := request
	foreign.UserID = "bob"
	foreign.RequestKey = "foreign"
	if _, _, err := store.AdmitTest(ctx, foreign, plan); !errors.Is(err, experiment.ErrTestQuoteRequired) {
		t.Fatalf("foreignquote=%v", err)
	}
	list, next, err := store.ListTests(ctx, "alice", 1, "")
	if err != nil || len(list) != 1 || next != "" || len(list[0].Candidates) != 4 || list[0].CommonSnapshot != nil {
		t.Fatalf("history=%+v %q %v", list, next, err)
	}
}
func TestWritingTestConcurrentVotesPreserveOneWinnerAndStaleVotes(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
	found := completeWriting(t, store, beginWriting(t, store, admitWriting(t, store, request, plan)))
	match := found.Matches[0]
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for index, id := range []string{match.LeftID, match.RightID} {
		wg.Add(1)
		go func(index int, id string) {
			defer wg.Done()
			_, err := store.DecideMatch(ctx, experiment.MatchDecision{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: fmt.Sprintf("race-%d", index)}, MatchID: match.ID, WinnerCandidateID: id})
			errs <- err
		}(index, id)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, experiment.ErrTestRevisionConflict) {
			t.Fatalf("race failure=%v", err)
		}
	}
	if success != 1 {
		t.Fatalf("winners=%d", success)
	}
	reloaded, _ := store.GetTest(ctx, "alice", found.ID)
	if reloaded.Status != experiment.TestCompleted || len(reloaded.Matches) != 1 || reloaded.WinnerID == "" {
		t.Fatal("invalid race winner")
	}
}
func TestWritingTestFailedOnlyRetryPreservesOriginalSnapshotAndSuccess(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorTemplate, "", 4, "start")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	success := work.CandidateIDs[0]
	original := testWritingOutput(t, success)
	if err := store.CompleteTestCandidate(ctx, work.Fence, success, original, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTestCheckpoint(ctx, work.Fence, "shared", []byte(`{"prepared":true}`)); err != nil {
		t.Fatal(err)
	}
	failed := work.CandidateIDs[1]
	if err := store.SaveTestCheckpoint(ctx, work.Fence, failed, []byte(`{"failed_stage":"write"}`)); err != nil {
		t.Fatal(err)
	}
	found, err := store.FinishTestExecution(ctx, work.Fence, 0, &experiment.Failure{Reason: experiment.FailureReasonUnknown})
	if err != nil || found.Status != experiment.TestPartial || len(found.Matches) != 0 {
		t.Fatalf("failedbarrier %v", err)
	}
	if err = store.ConfirmTestSettlement(ctx, work.Fence, 11); err != nil {
		t.Fatal(err)
	}
	found, _ = store.GetTest(ctx, "alice", found.ID)
	ids := work.CandidateIDs[1:]
	retry := experiment.TestRetry{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "retry"}, CandidateIDs: ids, QuoteKey: "retry-quote"}
	retryPlan := plan
	retryPlan.Calls = slices.Clone(plan.Calls[1:])
	retryPlan.EstimateCredits = 21
	now := time.Now()
	quote := experiment.TestQuote{Key: retry.QuoteKey, UserID: "alice", TestID: found.ID, Fingerprint: experiment.WritingTestRetryFingerprint(retry), Retry: experiment.TestRetryQuoteRequest{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, CandidateIDs: ids}, Plan: retryPlan, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err = store.PutTestQuote(ctx, quote); err != nil {
		t.Fatal(err)
	}
	admitted, fresh, err := store.ReserveTestRetry(ctx, retry, retryPlan)
	if err != nil || !fresh {
		t.Fatalf("retry=%v", err)
	}
	if _, fresh, err = store.ReserveTestRetry(ctx, retry, experiment.TestPlan{}); err != nil || fresh {
		t.Fatalf("retryreplay=%v", err)
	}
	retryWork := beginWriting(t, store, admitted)
	if len(retryWork.CandidateIDs) != 3 || !reflect.DeepEqual(retryWork.Plan.Snapshot, plan.Snapshot) || string(retryWork.SharedCheckpoint) != `{"prepared":true}` || string(retryWork.CandidateCheckpoints[failed]) != `{"failed_stage":"write"}` {
		t.Fatalf("retry inputs changed %+v", retryWork)
	}
	if err = store.CompleteTestCandidate(ctx, work.Fence, failed, testWritingOutput(t, failed), nil, nil); !errors.Is(err, experiment.ErrTestStateInvalid) {
		t.Fatalf("oldcallback=%v", err)
	}
	final := completeWriting(t, store, retryWork)
	if final.Status != experiment.TestReview {
		t.Fatal(final.Status)
	}
	for _, candidate := range final.Candidates {
		if candidate.ID == success && !slices.Equal(candidate.Output, original) {
			t.Fatal("successful output regenerated")
		}
	}
	if err = store.ConfirmTestSettlement(ctx, retryWork.Fence, 6); err != nil {
		t.Fatal(err)
	}
	final, _ = store.GetTest(ctx, "alice", found.ID)
	if final.ConfirmedCredits != 17 {
		t.Fatalf("confirmed=%d", final.ConfirmedCredits)
	}
}
func TestWritingTestCancelPurgeAndRestartFenceLatePayloadButSettleUsage(t *testing.T) {
	for _, mode := range []string{"cancel", "source-purge", "restart"} {
		t.Run(mode, func(t *testing.T) {
			store, handle := testStore(t)
			ctx := context.Background()
			request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
			work := beginWriting(t, store, admitWriting(t, store, request, plan))
			if err := store.SaveTestCheckpoint(ctx, work.Fence, "shared", []byte(`{"private":"checkpoint"}`)); err != nil {
				t.Fatal(err)
			}
			if err := store.CompleteTestCandidate(ctx, work.Fence, work.CandidateIDs[0], testWritingOutput(t, "saved"), nil, nil); err != nil {
				t.Fatal(err)
			}
			found, _ := store.GetTest(ctx, "alice", work.Fence.TestID)
			switch mode {
			case "cancel":
				request := experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "cancel"}
				if _, err := store.CancelTest(ctx, request); err != nil {
					t.Fatal(err)
				}
				if _, err := store.CancelTest(ctx, request); err != nil {
					t.Fatal(err)
				}
			case "source-purge":
				if _, err := handle.Writer.Exec(`DELETE FROM posts WHERE slug='post-a'`); err == nil {
					t.Fatal("unpurged source deleted")
				}
				if err := store.PurgeWritingTestPost(ctx, "alice", "post-a"); err != nil {
					t.Fatal(err)
				}
				if _, err := handle.Writer.Exec(`DELETE FROM posts WHERE slug='post-a'`); err != nil {
					t.Fatal(err)
				}
			case "restart":
				if err := store.RecoverInterruptedTests(ctx); err != nil {
					t.Fatal(err)
				}
				if err := store.RecoverInterruptedTests(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.CompleteTestCandidate(ctx, work.Fence, work.CandidateIDs[1], testWritingOutput(t, "late"), nil, nil); !errors.Is(err, experiment.ErrTestStateInvalid) {
				t.Fatalf("lateoutput=%v", err)
			}
			if err := store.SaveTestCheckpoint(ctx, work.Fence, "shared", []byte(`{"resurrect":"private"}`)); !errors.Is(err, experiment.ErrTestStateInvalid) {
				t.Fatalf("latecheckpoint=%v", err)
			}
			if err := store.ConfirmTestSettlement(ctx, work.Fence, 5); err != nil {
				t.Fatal(err)
			}
			if err := store.ConfirmTestSettlement(ctx, work.Fence, 5); err != nil {
				t.Fatal(err)
			}
			if err := store.ConfirmTestSettlement(ctx, work.Fence, 6); !errors.Is(err, experiment.ErrTestStateInvalid) {
				t.Fatalf("changedsettlement=%v", err)
			}
			final, _ := store.GetTest(ctx, "alice", found.ID)
			if final.ConfirmedCredits != 5 || len(final.Matches) != 0 {
				t.Fatal("badsettlement or automatic loss")
			}
			if mode == "source-purge" {
				if final.PurgeFence == 0 || len(final.CommonSnapshot) != 0 || final.SourcePostSlug != "" || final.Input.Material != "" {
					t.Fatal("privateinput restored")
				}
				for _, c := range final.Candidates {
					if len(c.Output) > 0 || len(c.FrozenVariant) > 0 {
						t.Fatal("privatecandidate restored")
					}
				}
				quote, _ := store.GetTestQuote(ctx, "alice", request.QuoteKey)
				if len(quote.Plan.Snapshot.Common) > 0 {
					t.Fatal("privatequote restored")
				}
			} else if mode == "restart" && final.Status != experiment.TestPartial {
				t.Fatal(final.Status)
			}
		})
	}
}
func TestWritingTestExpiryRetainsBracketIdentityAndLedgerOnly(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	found := completeWriting(t, store, work)
	match := found.Matches[0]
	found, err := store.DecideMatch(ctx, experiment.MatchDecision{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "vote"}, MatchID: match.ID, WinnerCandidateID: match.LeftID})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ConfirmTestSettlement(ctx, work.Fence, 9); err != nil {
		t.Fatal(err)
	}
	if _, err = handle.Writer.Exec(`UPDATE writing_tests SET content_expires_at='2020-01-01T00:00:00.000000000Z' WHERE id=?`, found.ID); err != nil {
		t.Fatal(err)
	}
	count, err := store.PurgeExpiredWritingTests(ctx, time.Now())
	if err != nil || count != 1 {
		t.Fatalf("purge %d %v", count, err)
	}
	count, err = store.PurgeExpiredWritingTests(ctx, time.Now())
	if err != nil || count != 0 {
		t.Fatalf("purge replay %d %v", count, err)
	}
	retained, err := store.GetTest(ctx, "alice", found.ID)
	if err != nil || retained.Status != experiment.TestCompleted || retained.WinnerID != found.WinnerID || retained.ConfirmedCredits != 9 || len(retained.Matches) != 1 || retained.Candidates[0].Identity == nil {
		t.Fatal("metadata lost")
	}
	for _, candidate := range retained.Candidates {
		if len(candidate.Output) > 0 || len(candidate.FrozenVariant) > 0 {
			t.Fatal("retained privatepayload")
		}
	}
}

func TestWritingTestCrashAfterAllOutputsOpensStableBracketWithoutProviderReplay(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 4, "start")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	for _, id := range work.CandidateIDs {
		if err := store.CompleteTestCandidate(ctx, work.Fence, id, testWritingOutput(t, id), nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecoverInterruptedTests(ctx); err != nil {
		t.Fatal(err)
	}
	found, err := store.GetTest(ctx, "alice", work.Fence.TestID)
	if err != nil || found.Status != experiment.TestReview || len(found.Matches) != 2 {
		t.Fatalf("recovery barrier=%v %+v", err, found)
	}
	if err = store.RecoverInterruptedTests(ctx); err != nil {
		t.Fatal(err)
	}
	again, _ := store.GetTest(ctx, "alice", found.ID)
	if !reflect.DeepEqual(found.Matches, again.Matches) || found.Revision != again.Revision {
		t.Fatal("recovery reshuffled/regenerated")
	}
	if err = store.ConfirmTestSettlement(ctx, work.Fence, 7); err != nil {
		t.Fatal(err)
	}
}

func TestWritingTestRetryCannotReplaceAnyFrozenVariantOrAssignmentMetadata(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorVoice, "", 2, "start")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	found, err := store.FinishTestExecution(ctx, work.Fence, 0, &experiment.Failure{Reason: experiment.FailureReasonUnknown})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"variant", "reference", "semantic", "assignments", "order"} {
		t.Run(change, func(t *testing.T) {
			modified := plan
			modified.Snapshot.Variants = slices.Clone(plan.Snapshot.Variants)
			switch change {
			case "variant":
				modified.Snapshot.Variants[0].Content = []byte(`{"replacement":true}`)
			case "reference":
				modified.Snapshot.Variants[0].Reference.SettingRevision = "2"
			case "semantic":
				modified.Snapshot.Variants[0].SemanticKey = "new"
			case "assignments":
				modified.Snapshot.AssignmentsHash = "new"
			case "order":
				modified.Snapshot.Variants[0], modified.Snapshot.Variants[1] = modified.Snapshot.Variants[1], modified.Snapshot.Variants[0]
			}
			retry := experiment.TestRetry{TestMutation: experiment.TestMutation{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, RequestKey: "retry-" + change}, CandidateIDs: work.CandidateIDs, QuoteKey: "quote-" + change}
			now := time.Now()
			quote := experiment.TestQuote{Key: retry.QuoteKey, UserID: "alice", TestID: found.ID, Fingerprint: experiment.WritingTestRetryFingerprint(retry), Retry: experiment.TestRetryQuoteRequest{UserID: "alice", TestID: found.ID, ExpectedRevision: found.Revision, CandidateIDs: work.CandidateIDs}, Plan: modified, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
			if err := store.PutTestQuote(ctx, quote); err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.ReserveTestRetry(ctx, retry, modified); !errors.Is(err, experiment.ErrTestMaterialInvalid) {
				t.Fatalf("replacement accepted %v", err)
			}
			again, _ := store.GetTest(ctx, "alice", found.ID)
			if again.Revision != found.Revision || again.Status != experiment.TestFailed {
				t.Fatal("failed retry changed aggregate")
			}
		})
	}
}

func TestWritingTestTerminalQueueReconciliationClosesEveryAdmissionStage(t *testing.T) {
	for _, stage := range []string{"prepared", "queued", "running"} {
		for _, outcome := range []experiment.TestExecutionOutcome{experiment.TestExecutionFailed, experiment.TestExecutionCancelled} {
			t.Run(stage+"-"+string(outcome), func(t *testing.T) {
				store, _ := testStore(t)
				ctx := context.Background()
				request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
				found := admitWriting(t, store, request, plan)
				work, err := store.PreparedTestWork(ctx, "alice", found.ID)
				if err != nil {
					t.Fatal(err)
				}
				if stage != "prepared" {
					if _, err = store.BindTestJob(ctx, work.Fence, work.Fence.JobID); err != nil {
						t.Fatal(err)
					}
				}
				if stage == "running" {
					if _, err = store.BeginTestExecution(ctx, work.Fence); err != nil {
						t.Fatal(err)
					}
					if err = store.CompleteTestCandidate(ctx, work.Fence, work.CandidateIDs[0], testWritingOutput(t, "success"), nil, nil); err != nil {
						t.Fatal(err)
					}
				}
				failure := &experiment.Failure{Reason: experiment.FailureReasonInterrupted}
				if err = store.EndTestExecution(ctx, work.Fence, outcome, failure); err != nil {
					t.Fatal(err)
				}
				if err = store.EndTestExecution(ctx, work.Fence, outcome, failure); err != nil {
					t.Fatal(err)
				}
				if err = store.CompleteTestCandidate(ctx, work.Fence, work.CandidateIDs[1], testWritingOutput(t, "late"), nil, nil); !errors.Is(err, experiment.ErrTestStateInvalid) {
					t.Fatalf("unfenced terminal=%v", err)
				}
				pending, err := store.PendingTestSettlements(ctx)
				if err != nil || len(pending) != 1 || pending[0] != work.Fence {
					t.Fatalf("pending receipts=%+v %v", pending, err)
				}
				if err = store.ConfirmTestSettlement(ctx, work.Fence, 3); err != nil {
					t.Fatal(err)
				}
				pending, err = store.PendingTestSettlements(ctx)
				if err != nil || len(pending) != 0 {
					t.Fatalf("settlement recovery repeated=%v", err)
				}
				final, _ := store.GetTest(ctx, "alice", found.ID)
				if outcome == experiment.TestExecutionCancelled {
					if final.Status != experiment.TestCancelled {
						t.Fatal(final.Status)
					}
				} else if stage == "running" {
					if final.Status != experiment.TestPartial {
						t.Fatal(final.Status)
					}
				} else if final.Status != experiment.TestFailed {
					t.Fatal(final.Status)
				}
				if final.ConfirmedCredits != 3 || len(final.Matches) != 0 {
					t.Fatal("badterminal receipt")
				}
				if stage == "running" {
					succeeded := 0
					for _, candidate := range final.Candidates {
						if candidate.Status == string(experiment.TestCandidateSucceeded) && len(candidate.Output) > 0 {
							succeeded++
						}
					}
					if succeeded != 1 {
						t.Fatal("terminal discarded success")
					}
				}
			})
		}
	}
}
func TestWritingTestConcurrentAdmissionAllocatesOneTestAndOneSeedSet(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 4, "start")
	now := time.Now()
	quote := experiment.TestQuote{Key: request.QuoteKey, UserID: "alice", Fingerprint: experiment.WritingTestStartFingerprint(request), Start: request, Plan: plan, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.PutTestQuote(ctx, quote); err != nil {
		t.Fatal(err)
	}
	type result struct {
		test  experiment.WritingTest
		fresh bool
		err   error
	}
	responses := make(chan result, 5)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			test, fresh, err := store.AdmitTest(ctx, request, plan)
			responses <- result{test, fresh, err}
		}()
	}
	wg.Wait()
	close(responses)
	var first experiment.WritingTest
	freshCount := 0
	for response := range responses {
		if response.err != nil {
			t.Fatal(response.err)
		}
		if response.fresh {
			freshCount++
		}
		if first.ID == "" {
			first = response.test
		} else if first.ID != response.test.ID || !reflect.DeepEqual(first.Candidates, response.test.Candidates) {
			t.Fatal("duplicate seed/test")
		}
	}
	if freshCount != 1 {
		t.Fatalf("fresh tests=%d", freshCount)
	}
	pending, err := store.PendingTestSettlements(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatal("duplicate execution identities", err)
	}
}
func TestWritingTestCancelAndCandidateCompletionRaceCannotRestoreAfterPurge(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	output := testWritingOutput(t, "late")
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		errs <- store.CompleteTestCandidate(ctx, work.Fence, work.CandidateIDs[0], output, nil, nil)
	}()
	go func() { defer wg.Done(); errs <- store.PurgeWritingTestPost(ctx, "alice", "post-a") }()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, experiment.ErrTestStateInvalid) {
			t.Fatal(err)
		}
	}
	found, err := store.GetTest(ctx, "alice", work.Fence.TestID)
	if err != nil {
		t.Fatal(err)
	}
	if found.PurgeFence == 0 || found.Status != experiment.TestCancelled {
		t.Fatal("purge lost race")
	}
	for _, candidate := range found.Candidates {
		if len(candidate.Output) > 0 || len(candidate.FrozenVariant) > 0 {
			t.Fatal("purge resurrected private result")
		}
	}
	if err = store.EndTestExecution(ctx, work.Fence, experiment.TestExecutionFailed, &experiment.Failure{Reason: experiment.FailureReasonUnknown}); err != nil {
		t.Fatal(err)
	}
	if err = store.ConfirmTestSettlement(ctx, work.Fence, 2); err != nil {
		t.Fatal(err)
	}
}

func TestWritingTestTerminalCancellationAfterOutputBarrierStillExplicitlyAbandons(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	ready := completeWriting(t, store, work)
	if err := store.EndTestExecution(ctx, work.Fence, experiment.TestExecutionCancelled, &experiment.Failure{Reason: "GENERATION_CANCELLED"}); err != nil {
		t.Fatal(err)
	}
	abandoned, _ := store.GetTest(ctx, "alice", ready.ID)
	if abandoned.Status != experiment.TestCancelled || abandoned.WinnerID != "" || len(abandoned.Matches) != 1 || abandoned.Matches[0].WinnerID != "" {
		t.Fatal("cancellation awardedchampion")
	}
	for _, candidate := range abandoned.Candidates {
		if len(candidate.Output) == 0 || candidate.Status != string(experiment.TestCandidateSucceeded) {
			t.Fatal("cancellationdiscardedpaidoutput")
		}
	}
	revision := abandoned.Revision
	if err := store.EndTestExecution(ctx, work.Fence, experiment.TestExecutionCancelled, nil); err != nil {
		t.Fatal(err)
	}
	again, _ := store.GetTest(ctx, "alice", ready.ID)
	if again.Revision != revision {
		t.Fatal("terminalcancelreplaymutated")
	}
}

func TestWritingTestFailedAndPartialExpiryRetainsBlindMetadataWithoutAbandonment(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			store, handle := testStore(t)
			ctx := context.Background()
			request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
			work := beginWriting(t, store, admitWriting(t, store, request, plan))
			if partial {
				if err := store.CompleteTestCandidate(ctx, work.Fence, work.CandidateIDs[0], testWritingOutput(t, "saved"), nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			failed, err := store.FinishTestExecution(ctx, work.Fence, 0, &experiment.Failure{Reason: experiment.FailureReasonInterrupted})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = handle.Writer.Exec(`UPDATE writing_tests SET content_expires_at='2020-01-01T00:00:00.000000000Z' WHERE id=?`, failed.ID); err != nil {
				t.Fatal(err)
			}
			n, err := store.PurgeExpiredWritingTests(ctx, time.Now())
			if err != nil || n != 1 {
				t.Fatalf("purgefailed=%d %v", n, err)
			}
			retained, _ := store.GetTest(ctx, "alice", failed.ID)
			if retained.Status != failed.Status || retained.PurgeFence == 0 || len(retained.CommonSnapshot) != 0 {
				t.Fatal("expiry inventedabandonment or retainedprivateinput")
			}
			wire := experiment.ProjectWritingTest(retained)
			for _, candidate := range wire.Candidates {
				if candidate.Identity != nil || candidate.Usage != nil || len(candidate.Output) != 0 {
					t.Fatal("expiry revealedblindidentity or retainedprivateoutput")
				}
			}
			if err = store.ConfirmTestSettlement(ctx, work.Fence, 4); err != nil {
				t.Fatal(err)
			}
		})
	}
}
