package app

import (
	"context"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

type WritingTestDispatch interface {
	AuthorizeDispatch(context.Context, string, string) error
}
type WritingTestExecutionModels struct {
	models   generation.LLM
	dispatch WritingTestDispatch
}

func NewWritingTestExecutionModels(models generation.LLM, dispatch WritingTestDispatch) *WritingTestExecutionModels {
	if models == nil || dispatch == nil {
		panic("experiment/app: writing test models and durable dispatch authorizer are required")
	}
	return &WritingTestExecutionModels{models: models, dispatch: dispatch}
}
func (m *WritingTestExecutionModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return m.models.Resolve(ref)
}
func (m *WritingTestExecutionModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	work, ok := usage.WorkFromContext(ctx)
	if !ok || work.Kind != WritingTestJobKind {
		return llm.Response{}, job.ErrDispatchRefused
	}
	if err := m.dispatch.AuthorizeDispatch(ctx, work.UserID, work.JobID); err != nil {
		return llm.Response{}, err
	}
	return m.models.Complete(ctx, ref, request)
}
