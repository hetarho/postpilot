package rpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

// AdminHandler implements postpilotv1connect.AdminServiceHandler.
//
// It carries no authorization of its own: both procedures are in the interceptor's
// master-only set, so a non-master call never reaches these functions. Keeping the check
// there rather than here is what makes "which procedures are privileged" answerable by
// reading one map.
type AdminHandler struct {
	svc *auth.Service
}

func NewAdminHandler(svc *auth.Service) *AdminHandler { return &AdminHandler{svc: svc} }

func (h *AdminHandler) ListUsers(ctx context.Context, _ *connect.Request[postpilotv1.ListUsersRequest]) (*connect.Response[postpilotv1.ListUsersResponse], error) {
	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		slog.Error("list users failed", "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not list accounts", "UNKNOWN_FAILURE", nil)
	}

	out := make([]*postpilotv1.PlanUser, 0, len(users))
	for _, user := range users {
		out = append(out, toProtoUser(user))
	}
	return connect.NewResponse(&postpilotv1.ListUsersResponse{Users: out}), nil
}

func (h *AdminHandler) SetUserPlan(ctx context.Context, req *connect.Request[postpilotv1.SetUserPlanRequest]) (*connect.Response[postpilotv1.SetUserPlanResponse], error) {
	target, ok := planrpc.FromProto(req.Msg.GetPlan())
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a plan is required", "PLAN_REQUIRED", nil)
	}
	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a user id is required", "USER_ID_REQUIRED", nil)
	}

	switch err := h.svc.SetUserPlan(ctx, userID, target); {
	case errors.Is(err, auth.ErrUserNotFound):
		return nil, rpcserver.NewAppError(connect.CodeNotFound, "account not found", "USER_NOT_FOUND", nil)
	case errors.Is(err, auth.ErrLastMaster):
		return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition,
			"the last master account cannot be demoted", "LAST_MASTER", nil)
	case err != nil:
		slog.Error("set user plan failed", "user_id", userID, "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not change the plan", "UNKNOWN_FAILURE", nil)
	}

	// The echo carries the id and the new tier only: created_at did not change, and the
	// caller is updating a row it already has.
	return connect.NewResponse(&postpilotv1.SetUserPlanResponse{
		User: &postpilotv1.PlanUser{Id: userID, Plan: planrpc.ToProto(target)},
	}), nil
}

func toProtoUser(user auth.User) *postpilotv1.PlanUser {
	return &postpilotv1.PlanUser{
		Id:        user.ID,
		Plan:      planrpc.ToProto(user.Plan),
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339),
	}
}

var _ postpilotv1connect.AdminServiceHandler = (*AdminHandler)(nil)
