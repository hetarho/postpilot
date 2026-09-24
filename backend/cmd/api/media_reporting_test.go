package main

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestMediaStagesAndFailuresUseTheOwnedReportingVocabulary(t *testing.T) {
	r := jobReporting{}
	for _, stage := range []string{"prepare_wait", "prepare", "prepare_retry", "render_wait", "render", "render_retry"} {
		if got, owned := r.SafeStage(clip.JobKindGenerate, stage); !owned || got != stage {
			t.Fatal(stage, got, owned)
		}
	}
	if got, _ := r.SafeStage(clip.JobKindRender, "private-host/token"); got != "unknown" {
		t.Fatal(got)
	}
	for code, reason := range map[clip.MediaFailure]string{
		clip.MediaFailureWaitExpired:       "CLIP_MEDIA_UNAVAILABLE",
		clip.MediaFailureAttemptsExhausted: "CLIP_MEDIA_RETRY_EXHAUSTED",
		clip.MediaFailureDeadlineExceeded:  "CLIP_MEDIA_TIMEOUT",
	} {
		f := r.Failure(&clip.StageFailure{Stage: "render", Cause: &clip.MediaStageFailure{Code: code}})
		if f.Reason != reason || f.TechnicalDetail != "" || len(f.Params) != 0 {
			t.Fatal(f)
		}
	}
}
