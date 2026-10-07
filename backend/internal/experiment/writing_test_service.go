package experiment

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"
)

// WritingTestService coordinates server-owned preparation and exact durable admission.
// It has no publication port: declaring a champion cannot mutate a source or setting.
type WritingTestService struct {
	deps   WritingTestDependencies
	config WritingTestConfig
	now    func() time.Time
}

func NewWritingTestService(deps WritingTestDependencies, config WritingTestConfig) *WritingTestService {
	if deps.Store == nil || deps.Variants == nil || deps.Preparation == nil || deps.FailedPreparation == nil || deps.Pricing == nil || deps.Queue == nil {
		panic("experiment: writing test dependencies are required")
	}
	if config.QuoteTTL <= 0 {
		config.QuoteTTL = 10 * time.Minute
	}
	if config.Retention <= 0 {
		config.Retention = 30 * 24 * time.Hour
	}
	return &WritingTestService{deps: deps, config: config, now: time.Now}
}
func (s *WritingTestService) prepare(ctx context.Context, r TestStart) (TestPlan, error) {
	if err := ValidateTestShape(r); err != nil {
		return TestPlan{}, err
	}
	variants := make([]FrozenTestVariant, len(r.Entrants))
	for i, ref := range r.Entrants {
		v, err := s.deps.Variants.ResolveTestVariant(ctx, r.UserID, r.Factor, ref)
		if err != nil {
			return TestPlan{}, err
		}
		if v.Reference != ref {
			return TestPlan{}, ErrTestEntrant
		}
		variants[i] = v
	}
	plan, err := s.deps.Preparation.PrepareWritingTest(ctx, r, variants)
	if err != nil {
		return TestPlan{}, err
	}
	if err := ValidWritingTestPlan(r, plan); err != nil {
		return TestPlan{}, err
	}
	price, err := s.deps.Pricing.PriceWritingTest(ctx, r.UserID, plan.Calls)
	if err != nil {
		return TestPlan{}, err
	}
	if price.Credits < 0 {
		return TestPlan{}, ErrTestMaterialInvalid
	}
	plan.EstimateCredits, plan.Free = price.Credits, price.Free
	return plan, nil
}
func (s *WritingTestService) Estimate(ctx context.Context, r TestStart) (TestQuote, error) {
	r.RequestKey = "estimate"
	r.QuoteKey = ""
	plan, err := s.prepare(ctx, r)
	if err != nil {
		return TestQuote{}, err
	}
	now := s.now().UTC()
	quote := TestQuote{Key: newID(), UserID: r.UserID, Fingerprint: WritingTestStartFingerprint(r), Start: r, Plan: plan, Credits: plan.EstimateCredits, Free: plan.Free, CreatedAt: now, ExpiresAt: now.Add(s.config.QuoteTTL), Retention: s.config.Retention}
	if err := s.deps.Store.PutTestQuote(ctx, quote); err != nil {
		return TestQuote{}, err
	}
	return publicTestQuote(quote), nil
}
func publicTestQuote(q TestQuote) TestQuote {
	return TestQuote{Key: q.Key, Credits: q.Credits, Free: q.Free, ExpiresAt: q.ExpiresAt}
}
func (s *WritingTestService) EstimateFailed(ctx context.Context, r TestRetryQuoteRequest) (TestQuote, error) {
	if err := ValidateRetryQuoteShape(r); err != nil {
		return TestQuote{}, err
	}
	found, err := s.deps.Store.GetTest(ctx, r.UserID, r.TestID)
	if err != nil {
		return TestQuote{}, err
	}
	if err := validateFailedSelection(found, r.ExpectedRevision, r.CandidateIDs); err != nil {
		return TestQuote{}, err
	}
	// Recheck live source/model rights without replacing the frozen variants.
	for _, candidate := range found.Candidates {
		if slices.Contains(r.CandidateIDs, candidate.ID) {
			if _, err := s.deps.Variants.ResolveTestVariant(ctx, r.UserID, found.Factor, candidate.Ref); err != nil {
				return TestQuote{}, err
			}
		}
	}
	plan, err := s.deps.FailedPreparation.PrepareFailedTestCandidates(ctx, r)
	if err != nil {
		return TestQuote{}, err
	}
	if err := ValidateWritingTestRetryPlan(found, plan); err != nil {
		return TestQuote{}, err
	}
	price := TestCost{Credits: 0, Free: true}
	if len(plan.Calls) > 0 {
		price, err = s.deps.Pricing.PriceWritingTest(ctx, r.UserID, plan.Calls)
		if err != nil {
			return TestQuote{}, err
		}
	}
	if price.Credits < 0 {
		return TestQuote{}, ErrTestMaterialInvalid
	}
	plan.EstimateCredits, plan.Free = price.Credits, price.Free
	now := s.now().UTC()
	retry := TestRetry{TestMutation: TestMutation{UserID: r.UserID, TestID: r.TestID, ExpectedRevision: r.ExpectedRevision}, CandidateIDs: r.CandidateIDs}
	quote := TestQuote{Key: newID(), UserID: r.UserID, TestID: r.TestID, Fingerprint: WritingTestRetryFingerprint(retry), Retry: r, Plan: plan, Credits: price.Credits, Free: price.Free, CreatedAt: now, ExpiresAt: now.Add(s.config.QuoteTTL), Retention: s.config.Retention}
	if err := s.deps.Store.PutTestQuote(ctx, quote); err != nil {
		return TestQuote{}, err
	}
	return publicTestQuote(quote), nil
}

// ValidateWritingTestRetryPlan prevents any frozen entrant or common input from being
// replaced while pricing a new failed-only execution attempt.
func ValidateWritingTestRetryPlan(found WritingTest, plan TestPlan) error {
	if plan.EstimateCredits < 0 || len(plan.Snapshot.Variants) != found.Count || plan.Snapshot.Hash != found.CommonHash || plan.Snapshot.PromptVersion != found.PromptVersion || !bytes.Equal(plan.Snapshot.Common, found.CommonSnapshot) {
		return ErrTestMaterialInvalid
	}
	seen := map[string]bool{}
	for _, variant := range plan.Snapshot.Variants {
		if seen[variant.SemanticKey] {
			return ErrTestDuplicate
		}
		seen[variant.SemanticKey] = true
		matched := false
		for _, candidate := range found.Candidates {
			if candidate.Ref == variant.Reference {
				matched = true
				if candidate.SourceRevision != variant.Revision || candidate.SemanticKey != variant.SemanticKey || !bytes.Equal(candidate.FrozenVariant, variant.Content) || candidate.Identity == nil || candidate.Identity.Synthetic != variant.Synthetic {
					return ErrTestMaterialInvalid
				}
				break
			}
		}
		if !matched {
			return ErrTestMaterialInvalid
		}
	}
	for _, call := range plan.Calls {
		if call.Ref.ProviderID == "" || call.Ref.ModelID == "" || (call.Stage != StageObserve && call.Stage != StageWrite) || call.Count <= 0 || call.PromptTokens <= 0 || call.CompletionTokens <= 0 {
			return ErrTestMaterialInvalid
		}
	}
	return nil
}
func validateFailedSelection(found WritingTest, revision uint32, ids []string) error {
	if found.Revision != revision {
		return ErrTestRevisionConflict
	}
	if found.PurgeFence != 0 || !(found.Status == TestPartial || found.Status == TestFailed) {
		return ErrTestStateInvalid
	}
	for _, id := range ids {
		valid := false
		for _, candidate := range found.Candidates {
			if candidate.ID == id {
				valid = candidate.Status == string(TestCandidateFailed)
				break
			}
		}
		if !valid {
			return ErrTestEntrant
		}
	}
	return nil
}
func (s *WritingTestService) Start(ctx context.Context, r TestStart) (WritingTest, error) {
	if err := ValidateTestShape(r); err != nil {
		return WritingTest{}, err
	}
	if existing, err := s.deps.Store.TestByRequest(ctx, r.UserID, r.RequestKey); err == nil {
		// Store verifies the fingerprint even when a quote is now expired or purged.
		admitted, _, err := s.deps.Store.AdmitTest(ctx, r, TestPlan{})
		if err != nil {
			return WritingTest{}, err
		}
		if existing.ID != admitted.ID {
			return WritingTest{}, ErrTestOperation
		}
		return s.dispatch(ctx, admitted)
	} else if !errors.Is(err, ErrTestNotFound) {
		return WritingTest{}, err
	}
	quote, err := s.deps.Store.GetTestQuote(ctx, r.UserID, r.QuoteKey)
	if err != nil {
		return WritingTest{}, ErrTestQuoteRequired
	}
	if quote.TestID != "" || quote.Fingerprint != WritingTestStartFingerprint(r) || !quote.ExpiresAt.After(s.now()) || quote.Plan.Snapshot.Hash == "" {
		return WritingTest{}, ErrTestQuoteRequired
	}
	current, err := s.prepare(ctx, r)
	if err != nil {
		return WritingTest{}, err
	}
	if !sameWritingTestPlan(current, quote.Plan) {
		return WritingTest{}, ErrTestQuoteRequired
	}
	admitted, _, err := s.deps.Store.AdmitTest(ctx, r, quote.Plan)
	if err != nil {
		return WritingTest{}, err
	}
	return s.dispatch(ctx, admitted)
}
func sameWritingTestPlan(a, b TestPlan) bool {
	if a.EstimateCredits != b.EstimateCredits || a.Free != b.Free || a.Snapshot.Hash != b.Snapshot.Hash || a.Snapshot.PromptVersion != b.Snapshot.PromptVersion || a.Snapshot.AssignmentsHash != b.Snapshot.AssignmentsHash || !bytes.Equal(a.Snapshot.Common, b.Snapshot.Common) || len(a.Snapshot.Variants) != len(b.Snapshot.Variants) || !slices.Equal(a.Calls, b.Calls) {
		return false
	}
	for i, v := range a.Snapshot.Variants {
		w := b.Snapshot.Variants[i]
		if v.Reference != w.Reference || v.Revision != w.Revision || v.SemanticKey != w.SemanticKey || v.Synthetic != w.Synthetic || !bytes.Equal(v.Content, w.Content) {
			return false
		}
	}
	return true
}
func (s *WritingTestService) Retry(ctx context.Context, r TestRetry) (WritingTest, error) {
	if r.RequestKey == "" {
		return WritingTest{}, ErrTestOperation
	}
	if err := ValidateRetryQuoteShape(TestRetryQuoteRequest{UserID: r.UserID, TestID: r.TestID, CandidateIDs: r.CandidateIDs}); err != nil {
		return WritingTest{}, err
	}
	if _, err := s.deps.Store.TestByRequest(ctx, r.UserID, r.RequestKey); err == nil {
		found, _, err := s.deps.Store.ReserveTestRetry(ctx, r, TestPlan{})
		if err != nil {
			return WritingTest{}, err
		}
		return s.dispatch(ctx, found)
	} else if !errors.Is(err, ErrTestNotFound) {
		return WritingTest{}, err
	}
	quote, err := s.deps.Store.GetTestQuote(ctx, r.UserID, r.QuoteKey)
	if err != nil {
		return WritingTest{}, ErrTestQuoteRequired
	}
	if quote.TestID != r.TestID || quote.Fingerprint != WritingTestRetryFingerprint(r) || quote.Retry.ExpectedRevision != r.ExpectedRevision || !quote.ExpiresAt.After(s.now()) {
		return WritingTest{}, ErrTestQuoteRequired
	}
	found, err := s.deps.Store.GetTest(ctx, r.UserID, r.TestID)
	if err != nil {
		return WritingTest{}, err
	}
	if err := validateFailedSelection(found, r.ExpectedRevision, r.CandidateIDs); err != nil {
		return WritingTest{}, err
	}
	for _, candidate := range found.Candidates {
		if slices.Contains(r.CandidateIDs, candidate.ID) {
			if _, err := s.deps.Variants.ResolveTestVariant(ctx, r.UserID, found.Factor, candidate.Ref); err != nil {
				return WritingTest{}, err
			}
		}
	}
	admitted, _, err := s.deps.Store.ReserveTestRetry(ctx, r, quote.Plan)
	if err != nil {
		return WritingTest{}, err
	}
	return s.dispatch(ctx, admitted)
}
func (s *WritingTestService) dispatch(ctx context.Context, found WritingTest) (WritingTest, error) {
	if found.Status != TestQueued {
		return ProjectWritingTest(found), nil
	}
	work, err := s.deps.Store.PreparedTestWork(ctx, found.UserID, found.ID)
	if err != nil {
		return WritingTest{}, err
	}
	jobID, err := s.deps.Queue.StartWritingTestJob(ctx, work)
	if err != nil {
		return WritingTest{}, err
	}
	// After durable admission, finish the bounded bind/activate handoff even if
	// the browser disconnects. Explicit test cancellation still wins its store fence.
	admissionCtx, stopAdmission := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer stopAdmission()
	found, err = s.deps.Store.BindTestJob(admissionCtx, work.Fence, jobID)
	if err != nil {
		actual, readErr := s.deps.Store.GetTest(admissionCtx, work.Fence.UserID, work.Fence.TestID)
		if readErr == nil && actual.JobID == jobID && actual.PurgeFence == work.Fence.PurgeFence && (actual.Status == TestQueued || actual.Status == TestRunning) {
			found, err = actual, nil // The bind committed and its response was lost.
		} else if readErr == nil && (actual.Status == TestCancelled || actual.PurgeFence != 0 || (actual.JobID != "" && actual.JobID != jobID)) {
			// A confirmed cancellation or newer epoch owns this aggregate. Release
			// this unactivated job; never guess about an unreadable commit.
			cleanupErr := s.deps.Queue.CancelWritingTestJob(admissionCtx, work.Fence.UserID, jobID)
			return WritingTest{}, errors.Join(err, cleanupErr)
		}
		if err != nil {
			return WritingTest{}, errors.Join(err, readErr)
		}
	}
	if err = s.deps.Queue.ActivateWritingTestJob(admissionCtx, found.UserID, jobID); err != nil {
		return WritingTest{}, err
	}
	return ProjectWritingTest(found), nil
}
func (s *WritingTestService) Get(ctx context.Context, user, id string) (WritingTest, error) {
	found, err := s.deps.Store.GetTest(ctx, user, id)
	if err != nil {
		return WritingTest{}, err
	}
	return ProjectWritingTest(found), nil
}
func (s *WritingTestService) List(ctx context.Context, user string, limit int, cursor string) ([]WritingTest, string, error) {
	found, next, err := s.deps.Store.ListTests(ctx, user, limit, cursor)
	for i := range found {
		found[i] = ProjectWritingTest(found[i])
	}
	return found, next, err
}
func (s *WritingTestService) Decide(ctx context.Context, r MatchDecision) (WritingTest, error) {
	found, err := s.deps.Store.DecideMatch(ctx, r)
	return ProjectWritingTest(found), err
}
func (s *WritingTestService) Cancel(ctx context.Context, r TestMutation) (WritingTest, error) {
	found, err := s.deps.Store.CancelTest(ctx, r)
	if err != nil {
		return WritingTest{}, err
	}
	if found.JobID != "" {
		if err = s.deps.Queue.CancelWritingTestJob(ctx, r.UserID, found.JobID); err != nil {
			return WritingTest{}, err
		}
	}
	return ProjectWritingTest(found), nil
}
