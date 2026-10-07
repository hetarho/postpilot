package app

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

type guardedTestModels struct{ calls int }

func (m *guardedTestModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Ref: ref}, true
}
func (m *guardedTestModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	m.calls++
	return llm.Response{Text: "complete"}, nil
}

type guardedTestDispatch struct {
	user, job string
	calls     int
	err       error
}

func (d *guardedTestDispatch) AuthorizeDispatch(_ context.Context, user, id string) error {
	d.user, d.job = user, id
	d.calls++
	return d.err
}
func TestWritingTestExecutionModelsRequiresOwnedDurableDispatchBeforeAnyProviderCall(t *testing.T) {
	models := &guardedTestModels{}
	dispatch := &guardedTestDispatch{}
	guard := NewWritingTestExecutionModels(models, dispatch)
	ref := llm.ModelRef{ProviderID: "p", ModelID: "writer"}
	for _, ctx := range []context.Context{t.Context(), usage.WithWork(t.Context(), usage.Work{UserID: "alice", JobID: "job", Kind: job.KindGenerate})} {
		if _, err := guard.Complete(ctx, ref, llm.Request{}); !errors.Is(err, job.ErrDispatchRefused) {
			t.Fatal("unowned call admitted", err)
		}
	}
	if dispatch.calls != 0 || models.calls != 0 {
		t.Fatal("invalid scope reached dispatch/provider")
	}
	ctx := usage.WithWork(t.Context(), usage.Work{UserID: "alice", JobID: "test-job", Kind: WritingTestJobKind})
	dispatch.err = job.ErrDispatchRefused
	if _, err := guard.Complete(ctx, ref, llm.Request{}); !errors.Is(err, job.ErrDispatchRefused) || models.calls != 0 {
		t.Fatal("cancelled call dispatched", err)
	}
	dispatch.err = nil
	if response, err := guard.Complete(ctx, ref, llm.Request{}); err != nil || response.Text != "complete" || models.calls != 1 || dispatch.user != "alice" || dispatch.job != "test-job" {
		t.Fatal(response, err, dispatch, models)
	}
}
