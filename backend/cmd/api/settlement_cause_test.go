package main

import (
	"testing"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

func TestFailedJobSettlementCauseFollowsDurableFailureReason(t *testing.T) {
	for _, tc := range []struct{ reason, want string }{
		{llm.FailureReasonModelRateLimited, "provider"},
		{llm.FailureReasonModelUnavailable, "provider"},
		{job.FailureReasonInterrupted, "service"},
		{job.FailureReasonPanicked, "service"},
		{llm.FailureReasonUnknown, "unknown"},
	} {
		if got := failureSettlementCause(tc.reason); got != tc.want {
			t.Errorf("%s -> %s, want %s", tc.reason, got, tc.want)
		}
	}
}
