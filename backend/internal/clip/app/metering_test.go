package app

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

func TestAdmitExecutionFailsClosedOutsideAChargedClipJob(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "p", ModelID: "o"}
	policy := &llm.ExecutionPolicy{Call: observePolicy()}
	plain := context.Background()
	if _, err := AdmitExecution(plain, ref, llm.Request{Execution: policy}); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("an execution policy with no job behind it is refused", err)
	}
	if got, err := AdmitExecution(plain, ref, llm.Request{}); err != nil || got != plain {
		t.Fatal("a plain call outside any job passes untouched", err)
	}
	post := usage.WithWork(plain, usage.Work{UserID: "alice", JobID: "post", Kind: "generate_post"})
	if _, err := AdmitExecution(post, ref, llm.Request{}); err != nil {
		t.Fatal("non-clip work is not the clip gate's business", err)
	}
	if _, err := AdmitExecution(post, ref, llm.Request{Execution: policy}); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("an execution policy inside non-clip work is refused", err)
	}
	render := usage.WithWork(plain, usage.Work{UserID: "alice", JobID: "render", Kind: job.KindRenderClip})
	if _, err := AdmitExecution(render, ref, llm.Request{}); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("a render job may call no model", err)
	}
	charged := usage.WithWork(plain, usage.Work{UserID: "alice", JobID: "gen", Kind: job.KindGenerateClip})
	if _, err := AdmitExecution(charged, ref, llm.Request{MaxTokens: 8192, Stage: "observe", Execution: policy}); err == nil {
		t.Fatal("a charged clip job with no reserved policy in its context cannot call")
	}
}
