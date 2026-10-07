package main

import (
	"testing"
	"time"

	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/proto"
)

func (h *writingIntegrationHarness) settledTest(t *testing.T, id string, want v1.WritingTestStatus) *v1.WritingTest {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		current := h.get(t, id)
		var pending int
		if err := h.platform.db.Reader.QueryRow(`SELECT count(*) FROM writing_test_attempts WHERE test_id=? AND settled=0`, id).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if current.Status == want && pending == 0 {
			// The receipt may have committed between the first read and the
			// accounting query; consume the revision after that durable fence.
			return h.get(t, id)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("test %s did not settle in %s", id, want)
	return nil
}

func TestWritingTestsProductionFailedOnlyRetryPreservesThreeSuccessfulPosts(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	h.provider.mu.Lock()
	h.provider.failures = map[string]int{"writer-1": 1}
	h.provider.mu.Unlock()
	plan := h.plan(4)
	quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h, &v1.EstimateWritingTestRequest{Plan: plan}))
	if err != nil {
		t.Fatal(err)
	}
	start := &v1.StartWritingTestRequest{Plan: plan, RequestKey: "four-failure", QuoteKey: quote.Msg.QuoteKey}
	started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h, start))
	if err != nil {
		t.Fatal(err)
	}
	current := h.settledTest(t, started.Msg.Test.Id, v1.WritingTestStatus_WRITING_TEST_STATUS_PARTIAL)
	if len(current.Matches) != 0 {
		t.Fatal("partial generation opened matches")
	}
	successes := map[string]*v1.PostContent{}
	var failed []string
	for _, candidate := range current.Candidates {
		if candidate.Status == v1.WritingTestCandidateStatus_WRITING_TEST_CANDIDATE_STATUS_SUCCEEDED {
			successes[candidate.Id] = proto.Clone(candidate.Output).(*v1.PostContent)
		} else {
			failed = append(failed, candidate.Id)
		}
	}
	if len(successes) != 3 || len(failed) != 1 {
		t.Fatalf("successful=%d failed=%d", len(successes), len(failed))
	}
	retryQuote, err := h.client.EstimateFailedTestCandidates(t.Context(), writingRPCRequest(h, &v1.EstimateFailedTestCandidatesRequest{TestId: current.Id, ExpectedRevision: current.Revision, CandidateIds: failed}))
	if err != nil {
		t.Fatal(err)
	}
	retry := &v1.RetryFailedTestCandidatesRequest{TestId: current.Id, ExpectedRevision: current.Revision, RequestKey: "only-failed", CandidateIds: failed, QuoteKey: retryQuote.Msg.QuoteKey}
	if _, err := h.client.RetryFailedTestCandidates(t.Context(), writingRPCRequest(h, retry)); err != nil {
		t.Fatal(err)
	}
	current = h.settledTest(t, current.Id, v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW)
	for _, candidate := range current.Candidates {
		if original := successes[candidate.Id]; original != nil && !proto.Equal(original, candidate.Output) {
			t.Fatal("retry changed successful output")
		}
	}
	if _, err := h.client.RetryFailedTestCandidates(t.Context(), writingRPCRequest(h, retry)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h, start)); err != nil {
		t.Fatal(err)
	}
	h.provider.mu.Lock()
	calls := len(h.provider.requests)
	h.provider.mu.Unlock()
	var admissions, events int
	if err := h.platform.db.Reader.QueryRow(`SELECT count(*) FROM usage_admissions`).Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if err := h.platform.db.Reader.QueryRow(`SELECT count(*) FROM usage_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if calls != 5 || events != 5 || admissions != 2 {
		t.Fatalf("calls=%d events=%d admissions=%d", calls, events, admissions)
	}
}
