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
	"github.com/postpilot/backend/internal/plan"
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
	svc    AdminAccounts
	combos EstimatorAssigner
	rates  RateReader
}

type AdminAccounts interface {
	ListUsers(ctx context.Context) ([]auth.User, error)
	SetUserPlan(ctx context.Context, userID string, target plan.Plan) error
}

// EstimatorAssigner points an estimator combo at two curated models. Declared here by its
// consumer and implemented by the model catalog, which owns the assignment.
//
// It speaks in ids and in the two sentinels below rather than in the catalog's own types, so
// the auth context imports nothing from it; the composition root translates.
type EstimatorAssigner interface {
	AssignCombo(ctx context.Context, combo, observeModelID, writeModelID string) error
}

// RateReader reports the rate a paid job admitted now would select (QUOTA-59). Declared by its
// consumer; the composition root passes the same source GetMyPlan prices estimates at, so the
// operator reads exactly the rate customers are priced at without being told it (QUOTA-65).
type RateReader interface {
	CurrentRate(ctx context.Context) (plan.RateSnapshot, error)
}

var (
	// ErrComboUnknown is a combo name off the four the product has.
	ErrComboUnknown = errors.New("unknown estimator combo")
	// ErrComboModelUnusable is a model that is not curated, or is curated but not
	// registered to the purpose the combo needs it for.
	ErrComboModelUnusable = errors.New("estimator combo model is not registered")
)

func NewAdminHandler(svc AdminAccounts, combos EstimatorAssigner, rates RateReader) *AdminHandler {
	return &AdminHandler{svc: svc, combos: combos, rates: rates}
}

// GetExchangeRate is the operator's one view of the rate behind credits (QUOTA-65). A missing or
// ineligible rate is a state to show, not a failed read: paid AI work is what it stops.
func (h *AdminHandler) GetExchangeRate(ctx context.Context, _ *connect.Request[postpilotv1.GetExchangeRateRequest]) (*connect.Response[postpilotv1.GetExchangeRateResponse], error) {
	unavailable := connect.NewResponse(&postpilotv1.GetExchangeRateResponse{Unavailable: true})
	if h.rates == nil {
		return unavailable, nil
	}
	rate, err := h.rates.CurrentRate(ctx)
	if err != nil || !rate.Valid() {
		if err != nil {
			slog.Warn("exchange rate unavailable", "err", err)
		}
		return unavailable, nil
	}
	return connect.NewResponse(&postpilotv1.GetExchangeRateResponse{Rate: &postpilotv1.PlanFXRate{
		Source: rate.Source, PublicationDate: rate.PublicationDate,
		ReferenceE4: rate.ReferenceE4, AppliedE4: rate.AppliedE4, Temporary: rate.Temporary,
	}}), nil
}

func (h *AdminHandler) ListUsers(ctx context.Context, _ *connect.Request[postpilotv1.ListUsersRequest]) (*connect.Response[postpilotv1.ListUsersResponse], error) {
	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		slog.Error("list users failed", "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not list accounts", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
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
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a plan is required", postpilotv1.FailureReason_PLAN_REQUIRED, nil)
	}
	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a user id is required", postpilotv1.FailureReason_USER_ID_REQUIRED, nil)
	}
	// A master leaves master only by ANOTHER master (QUOTA-63), whatever tier is asked for:
	// an operator must not be able to click away their own administration. The shell's
	// `api setplan` has no actor and stays the recovery path.
	caller, ok := auth.UserFromContext(ctx)
	if !ok {
		slog.Error("set user plan reached without an actor", "user_id", userID)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not change the plan", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
	if caller == userID {
		return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition,
			"an operator cannot change their own plan", postpilotv1.FailureReason_MASTER_SELF_PLAN, nil)
	}

	switch err := h.svc.SetUserPlan(ctx, userID, target); {
	case errors.Is(err, auth.ErrUserNotFound):
		return nil, rpcserver.NewAppError(connect.CodeNotFound, "account not found", postpilotv1.FailureReason_USER_NOT_FOUND, nil)
	case errors.Is(err, auth.ErrLastMaster):
		return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition,
			"the last master account cannot be demoted", postpilotv1.FailureReason_LAST_MASTER, nil)
	case err != nil:
		slog.Error("set user plan failed", "user_id", userID, "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not change the plan", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
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

// SetEstimatorCombo assigns the pair of models one combo is priced with (QUOTA-39).
//
// The refusals are the two the operator can act on: a combo name that is not one of the
// four levels, and a model that is not registered at that level for the purpose the combo
// needs. Both are stated as reasons rather than as prose, so the client renders its own copy.
func (h *AdminHandler) SetEstimatorCombo(ctx context.Context, req *connect.Request[postpilotv1.SetEstimatorComboRequest]) (*connect.Response[postpilotv1.SetEstimatorComboResponse], error) {
	combo := req.Msg.GetCombo()
	observe := req.Msg.GetObserveModelId()
	write := req.Msg.GetWriteModelId()
	if combo == "" || observe == "" || write == "" {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument,
			"a combo and both models are required", postpilotv1.FailureReason_COMBO_INCOMPLETE, nil)
	}

	switch err := h.combos.AssignCombo(ctx, combo, observe, write); {
	case errors.Is(err, ErrComboUnknown):
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument,
			"unknown estimator combo", postpilotv1.FailureReason_COMBO_UNKNOWN, nil)
	case errors.Is(err, ErrComboModelUnusable):
		return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition,
			"the model is not registered at the combo level for that stage", postpilotv1.FailureReason_MODEL_NOT_REGISTERED, nil)
	case err != nil:
		slog.Error("set estimator combo failed", "combo", combo, "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal,
			"could not assign the combo", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
	return connect.NewResponse(&postpilotv1.SetEstimatorComboResponse{}), nil
}
