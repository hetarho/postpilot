package store_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
)

type writingTestPorts struct {
	plan  experiment.TestPlan
	store interface {
		GetTest(context.Context, string, string) (experiment.WritingTest, error)
	}
	jobs                      map[string]bool
	holds, activate, resolves int
	cancels                   int
	beforeReturn              func(experiment.TestExecutionWork)
	deny                      error
	mutate                    bool
}

func (p *writingTestPorts) ResolveTestVariant(_ context.Context, _ string, _ experiment.TestFactor, ref experiment.TestEntrantRef) (experiment.FrozenTestVariant, error) {
	p.resolves++
	if p.deny != nil {
		return experiment.FrozenTestVariant{}, p.deny
	}
	for _, variant := range p.plan.Snapshot.Variants {
		if variant.Reference == ref {
			return variant, nil
		}
	}
	return experiment.FrozenTestVariant{}, experiment.ErrTestEntrant
}
func (p *writingTestPorts) PrepareWritingTest(context.Context, experiment.TestStart, []experiment.FrozenTestVariant) (experiment.TestPlan, error) {
	plan := p.plan
	if p.mutate {
		plan.Snapshot.Hash = "changed-after-quote"
	}
	return plan, nil
}
func (p *writingTestPorts) PrepareFailedTestCandidates(context.Context, experiment.TestRetryQuoteRequest) (experiment.TestPlan, error) {
	return p.plan, nil
}
func (p *writingTestPorts) PriceWritingTest(_ context.Context, _ string, calls []experiment.TestCall) (experiment.TestCost, error) {
	credits := 0
	for _, call := range calls {
		credits += call.Count * 7
	}
	return experiment.TestCost{Credits: credits}, nil
}
func (p *writingTestPorts) StartWritingTestJob(_ context.Context, work experiment.TestExecutionWork) (string, error) {
	if p.jobs == nil {
		p.jobs = map[string]bool{}
	}
	if !p.jobs[work.Fence.JobID] {
		if !work.Fence.NonMetered {
			p.holds++
		}
		p.jobs[work.Fence.JobID] = true
	}
	if p.beforeReturn != nil {
		p.beforeReturn(work)
	}
	return work.Fence.JobID, nil
}
func (p *writingTestPorts) ActivateWritingTestJob(ctx context.Context, user, jobID string) error { // Activation must see an already committed aggregate binding.
	found, err := p.store.GetTest(ctx, user, p.testID(ctx, user, jobID))
	if err != nil {
		return err
	}
	if found.JobID != jobID {
		return fmt.Errorf("activated before aggregate binding")
	}
	p.activate++
	return nil
}
func (p *writingTestPorts) testID(ctx context.Context, user, jobID string) string {
	list, _, _ := p.store.(interface {
		ListTests(context.Context, string, int, string) ([]experiment.WritingTest, string, error)
	}).ListTests(ctx, user, 100, "")
	for _, test := range list {
		if test.JobID == jobID {
			return test.ID
		}
	}
	return ""
}
func (p *writingTestPorts) CancelWritingTestJob(context.Context, string, string) error {
	p.cancels++
	return nil
}
func newWritingService(t *testing.T) (*experiment.WritingTestService, *writingTestPorts, experiment.TestStart) {
	t.Helper()
	store, _ := testStore(t)
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 4, "start")
	ports := &writingTestPorts{plan: plan, store: store}
	service := experiment.NewWritingTestService(experiment.WritingTestDependencies{Store: store, Variants: ports, Preparation: ports, FailedPreparation: ports, Pricing: ports, Queue: ports}, experiment.WritingTestConfig{})
	return service, ports, request
}
func TestWritingTestServiceRequiresExactQuoteAndDispatchesOnlyAfterBinding(t *testing.T) {
	service, ports, request := newWritingService(t)
	ctx := context.Background()
	if _, err := service.Start(ctx, request); !errors.Is(err, experiment.ErrTestQuoteRequired) {
		t.Fatal(err)
	}
	if ports.holds != 0 {
		t.Fatal("unquoted hold")
	}
	quote, err := service.Estimate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if quote.Credits != 28 || quote.Key == "" || quote.UserID != "" || len(quote.Plan.Calls) != 0 || !quote.ExpiresAt.After(time.Now()) {
		t.Fatalf("public quote leaked preparation %+v", quote)
	}
	request.QuoteKey = quote.Key
	found, err := service.Start(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if ports.holds != 1 || ports.activate != 1 || found.JobID == "" || len(found.CommonSnapshot) != 0 || found.Input.Material != "" {
		t.Fatal("invalid admission")
	}
	for _, candidate := range found.Candidates {
		if candidate.Identity != nil || candidate.Usage != nil || len(candidate.Accounting) != 0 || candidate.Ref.SourceKind != "" {
			t.Fatal("blind identity leak")
		}
	}
	beforeResolves := ports.resolves
	again, err := service.Start(ctx, request)
	if err != nil || again.ID != found.ID || ports.holds != 1 || ports.resolves != beforeResolves {
		t.Fatalf("lost response replay=%v holds=%d", err, ports.holds)
	}
	if !reflect.DeepEqual(found.Candidates, again.Candidates) {
		t.Fatal("retry reshuffled")
	}
	foreign := request
	foreign.UserID = "bob"
	if _, err = service.Start(ctx, foreign); !errors.Is(err, experiment.ErrTestQuoteRequired) {
		t.Fatal(err)
	}
	if ports.holds != 1 {
		t.Fatal("foreign hold")
	}
}
func TestWritingTestServiceRejectsExpiredOrChangedPlanBeforeHold(t *testing.T) {
	for _, mode := range []string{"changed-plan", "rights", "changed-material"} {
		t.Run(mode, func(t *testing.T) {
			service, ports, request := newWritingService(t)
			ctx := context.Background()
			quote, err := service.Estimate(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			request.QuoteKey = quote.Key
			switch mode {
			case "changed-plan":
				ports.mutate = true
			case "rights":
				ports.deny = experiment.ErrTestEntrant
			case "changed-material":
				request.Input.Material = "new private material"
			}
			if _, err = service.Start(ctx, request); err == nil {
				t.Fatal("changed quote admitted")
			}
			if ports.holds != 0 {
				t.Fatal("changed quote held credits")
			}
		})
	}
}

func TestWritingTestAdmissionCancellationBeforeBindReleasesDeferredJob(t *testing.T) {
	service, ports, request := newWritingService(t)
	ctx := context.Background()
	quote, err := service.Estimate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.QuoteKey = quote.Key
	ports.beforeReturn = func(work experiment.TestExecutionWork) {
		_, err := ports.store.(interface {
			CancelTest(context.Context, experiment.TestMutation) (experiment.WritingTest, error)
		}).CancelTest(ctx, experiment.TestMutation{UserID: work.Test.UserID, TestID: work.Test.ID, ExpectedRevision: work.Test.Revision, RequestKey: "cancel-before-bind"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = service.Start(ctx, request); !errors.Is(err, experiment.ErrTestStateInvalid) {
		t.Fatalf("cancelled admission=%v", err)
	}
	if ports.activate != 0 || ports.cancels != 1 {
		t.Fatalf("deferred cleanup activates=%d cancels=%d", ports.activate, ports.cancels)
	}
}
func TestWritingTestDisconnectedStartFinishesDurableHandoff(t *testing.T) {
	service, ports, request := newWritingService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	quote, err := service.Estimate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.QuoteKey = quote.Key
	ports.beforeReturn = func(experiment.TestExecutionWork) { cancel() }
	found, err := service.Start(ctx, request)
	if err != nil || found.JobID == "" || ports.activate != 1 || ports.cancels != 0 {
		t.Fatalf("disconnected durable handoff=%v %+v", err, found)
	}
}
func TestWritingTestReplayOnlyRetryCreatesZeroHoldEpochAndRetainsProof(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	request, plan := writingRequest(experiment.FactorModel, experiment.StageWrite, 2, "start")
	work := beginWriting(t, store, admitWriting(t, store, request, plan))
	for _, id := range work.CandidateIDs {
		if err := store.SaveTestCheckpoint(ctx, work.Fence, id, []byte(`{"answer":"durable complete paid answer"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.FinishTestExecution(ctx, work.Fence, 0, &experiment.Failure{Reason: experiment.FailureReasonInterrupted}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmTestSettlement(ctx, work.Fence, 6); err != nil {
		t.Fatal(err)
	}
	failed, _ := store.GetTest(ctx, "alice", work.Fence.TestID)
	replayPlan := plan
	replayPlan.Calls = nil
	replayPlan.EstimateCredits = 0
	replayPlan.Free = true
	ports := &writingTestPorts{plan: replayPlan, store: store}
	service := experiment.NewWritingTestService(experiment.WritingTestDependencies{Store: store, Variants: ports, Preparation: ports, FailedPreparation: ports, Pricing: ports, Queue: ports}, experiment.WritingTestConfig{})
	retryQuote, err := service.EstimateFailed(ctx, experiment.TestRetryQuoteRequest{UserID: "alice", TestID: failed.ID, ExpectedRevision: failed.Revision, CandidateIDs: work.CandidateIDs})
	if err != nil || retryQuote.Credits != 0 || !retryQuote.Free {
		t.Fatalf("replayquote=%+v %v", retryQuote, err)
	}
	admitted, err := service.Retry(ctx, experiment.TestRetry{TestMutation: experiment.TestMutation{UserID: "alice", TestID: failed.ID, ExpectedRevision: failed.Revision, RequestKey: "retry-replay"}, CandidateIDs: work.CandidateIDs, QuoteKey: retryQuote.Key})
	if err != nil {
		t.Fatal(err)
	}
	if ports.holds != 0 || ports.activate != 1 {
		t.Fatal("replay-only epoch held credits")
	}
	replayWork, err := store.PreparedTestWork(ctx, "alice", admitted.ID)
	if err != nil || !replayWork.Fence.NonMetered || len(replayWork.Plan.Calls) != 0 {
		t.Fatalf("replayproof=%+v %v", replayWork.Fence, err)
	}
	if _, err = store.BeginTestExecution(ctx, replayWork.Fence); err != nil {
		t.Fatal(err)
	}
	ready := completeWriting(t, store, replayWork)
	if ready.Status != experiment.TestReview {
		t.Fatal(ready.Status)
	}
	if err = store.ConfirmTestSettlement(ctx, replayWork.Fence, 0); err != nil {
		t.Fatal(err)
	}
	final, _ := store.GetTest(ctx, "alice", ready.ID)
	if final.ConfirmedCredits != 6 {
		t.Fatal("replay double-debit")
	}
	if err = store.PurgeWritingTestPost(ctx, "alice", "post-a"); err != nil {
		t.Fatal(err)
	}
	fakeFence := replayWork.Fence
	fakeFence.NonMetered = false
	if err = store.ConfirmTestSettlement(ctx, fakeFence, 0); !errors.Is(err, experiment.ErrTestStateInvalid) {
		t.Fatal("non-metered epoch spoofed", err)
	}
}

func TestWritingTestExpiredQuoteCreatesNoJobOrReservation(t *testing.T) {
	_, ports, request := newWritingService(t)
	store := ports.store.(experiment.WritingTestStorage)
	service := experiment.NewWritingTestService(experiment.WritingTestDependencies{Store: store, Variants: ports, Preparation: ports, FailedPreparation: ports, Pricing: ports, Queue: ports}, experiment.WritingTestConfig{QuoteTTL: time.Nanosecond})
	ctx := context.Background()
	quote, err := service.Estimate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.QuoteKey = quote.Key
	if _, err = service.Start(ctx, request); !errors.Is(err, experiment.ErrTestQuoteRequired) {
		t.Fatalf("expired quote=%v", err)
	}
	if ports.holds != 0 || ports.activate != 0 {
		t.Fatal("expired quote created job/hold")
	}
	if _, err = store.TestByRequest(ctx, "alice", request.RequestKey); !errors.Is(err, experiment.ErrTestNotFound) {
		t.Fatal("expired quote created aggregate", err)
	}
}
