// Package rpc is the purpose context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/purpose"
)

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Handler struct{ service *purpose.Service }

func NewHandler(service *purpose.Service) *Handler { return &Handler{service: service} }

func (h *Handler) ListPurposes(ctx context.Context, _ *connect.Request[postpilotv1.ListPurposesRequest]) (*connect.Response[postpilotv1.ListPurposesResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	purposes, err := h.service.List(ctx, userID)
	if err != nil {
		return nil, toConnectError("list purposes", err)
	}
	out := make([]*postpilotv1.Purpose, 0, len(purposes))
	for _, p := range purposes {
		out = append(out, toProtoPurpose(p))
	}
	return connect.NewResponse(&postpilotv1.ListPurposesResponse{Purposes: out}), nil
}

func (h *Handler) CreatePurpose(ctx context.Context, req *connect.Request[postpilotv1.CreatePurposeRequest]) (*connect.Response[postpilotv1.CreatePurposeResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	created, err := h.service.Create(ctx, userID, req.Msg.GetName(), req.Msg.GetDescription(), req.Msg.GetInstructions())
	if err != nil {
		return nil, toConnectError("create purpose", err)
	}
	return connect.NewResponse(&postpilotv1.CreatePurposeResponse{Purpose: toProtoPurpose(created)}), nil
}

func (h *Handler) UpdatePurpose(ctx context.Context, req *connect.Request[postpilotv1.UpdatePurposeRequest]) (*connect.Response[postpilotv1.UpdatePurposeResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	// Presence, not emptiness, is the edit unit: an explicitly sent empty description is a
	// real instruction to clear it, and an absent one is not part of this edit at all.
	patch := purpose.Patch{}
	if req.Msg.Name != nil {
		patch.Name = req.Msg.Name
	}
	if req.Msg.Description != nil {
		patch.Description = req.Msg.Description
	}
	if req.Msg.Instructions != nil {
		patch.Instructions = req.Msg.Instructions
	}
	updated, err := h.service.Update(ctx, userID, req.Msg.GetId(), patch)
	if err != nil {
		return nil, toConnectError("update purpose", err)
	}
	return connect.NewResponse(&postpilotv1.UpdatePurposeResponse{Purpose: toProtoPurpose(updated)}), nil
}

func (h *Handler) DeletePurpose(ctx context.Context, req *connect.Request[postpilotv1.DeletePurposeRequest]) (*connect.Response[postpilotv1.DeletePurposeResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	detached, err := h.service.Delete(ctx, userID, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError("delete purpose", err)
	}
	return connect.NewResponse(&postpilotv1.DeletePurposeResponse{DetachedPosts: int32(detached)}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return userID, nil
}

// toConnectError maps the context's sentinels to wire codes. A foreign purpose is NotFound
// like an unknown one — the two must not be distinguishable.
func toConnectError(op string, err error) error {
	var tooLong *purpose.FieldTooLongError
	switch {
	case errors.As(err, &tooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "purpose field is too long", "PURPOSE_FIELD_TOO_LONG", map[string]string{
			"field": tooLong.Field, "max": strconv.Itoa(tooLong.Max), "actual": strconv.Itoa(tooLong.Chars),
		})
	case errors.Is(err, purpose.ErrNameRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "purpose name is required", "PURPOSE_NAME_REQUIRED", nil)
	case errors.Is(err, purpose.ErrInstructionsRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "purpose instructions are required", "PURPOSE_INSTRUCTIONS_REQUIRED", nil)
	case errors.Is(err, purpose.ErrDuplicateName):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "purpose name already exists", "PURPOSE_NAME_TAKEN", nil)
	case errors.Is(err, purpose.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "purpose not found", "PURPOSE_NOT_FOUND", nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", "UNKNOWN_FAILURE", nil)
	}
}

func toProtoPurpose(p purpose.Purpose) *postpilotv1.Purpose {
	if p.ID == "" {
		return nil
	}
	return &postpilotv1.Purpose{
		Id: p.ID, Name: p.Name, Description: p.Description, Instructions: p.Instructions,
		PostCount: int32(p.PostCount),
		CreatedAt: p.CreatedAt.UTC().Format(timeLayout), UpdatedAt: p.UpdatedAt.UTC().Format(timeLayout),
	}
}
