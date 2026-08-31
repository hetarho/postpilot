package rpc

import (
	"testing"

	"github.com/postpilot/backend/internal/publishing"
)

func TestPublishJobProjectsOnlyStructuredFailure(t *testing.T) {
	params := map[string]string{"attempt": "2"}
	got := toProtoJob(publishing.Job{
		ErrorCode:    "legacy_code",
		ErrorMessage: "legacy prose",
		Failure: publishing.Failure{
			Reason:          "PUBLISH_AGENT_UNAVAILABLE",
			Params:          params,
			TechnicalDetail: "redacted (17 bytes)",
		},
	})
	params["attempt"] = "mutated"

	if got.GetErrorCode() != "" || got.GetErrorMessage() != "" {
		t.Fatalf("legacy wire fields leaked: code=%q message=%q", got.GetErrorCode(), got.GetErrorMessage())
	}
	if got.GetFailure().GetReason() != "PUBLISH_AGENT_UNAVAILABLE" || got.GetFailure().GetParams()["attempt"] != "2" || got.GetFailure().GetTechnicalDetail() != "redacted (17 bytes)" {
		t.Fatalf("structured failure=%+v", got.GetFailure())
	}
}

func TestPublishJobOmitsEmptyFailure(t *testing.T) {
	if got := toProtoJob(publishing.Job{}); got.GetFailure() != nil {
		t.Fatalf("empty failure=%+v", got.GetFailure())
	}
}
