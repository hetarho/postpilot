package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobrpc "github.com/postpilot/backend/internal/job/rpc"
)

func (h *Handler) CancelClipJob(ctx context.Context, req *connect.Request[v1.CancelClipJobRequest]) (*connect.Response[v1.CancelClipJobResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.jobs == nil || h.generation == nil {
		return nil, toConnectError(clip.ErrCancellationPolicy)
	}
	j, err := h.jobs.CancelClipJob(ctx, user, req.Msg.ProjectId, req.Msg.JobId)
	if errors.Is(err, job.ErrNotFound) {
		err = clip.ErrNotFound
	}
	if errors.Is(err, job.ErrCancellationUnavailable) {
		err = clip.ErrCancellationPolicy
	}
	if err != nil {
		return nil, toConnectError(err)
	}
	a, err := h.generation.AccountingForAttempt(ctx, user, req.Msg.ProjectId, req.Msg.JobId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.CancelClipJobResponse{Job: jobrpc.ToProto(j), Accounting: accountingProto(a), Accepted: j.CancelRequestedAt != nil}), nil
}
