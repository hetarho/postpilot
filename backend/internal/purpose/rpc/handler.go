// Package rpc is the purpose context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
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
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	return userID, nil
}

// Korean field labels for the length messages. The wire error is what the create/edit form
// shows verbatim, so the field has to be named in the user's language rather than by its
// proto name.
var fieldLabels = map[string]string{"name": "이름", "description": "설명", "instructions": "작성 지침"}

// toConnectError maps the context's sentinels to wire codes. A foreign purpose is NotFound
// like an unknown one — the two must not be distinguishable.
func toConnectError(op string, err error) error {
	var tooLong *purpose.FieldTooLongError
	switch {
	case errors.As(err, &tooLong):
		label, ok := fieldLabels[tooLong.Field]
		if !ok {
			label = tooLong.Field
		}
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s은(는) %d자까지 쓸 수 있어요 (지금 %d자)", label, tooLong.Max, tooLong.Chars))
	case errors.Is(err, purpose.ErrNameRequired):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("용도 이름을 입력해 주세요"))
	case errors.Is(err, purpose.ErrInstructionsRequired):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("작성 지침을 입력해 주세요"))
	case errors.Is(err, purpose.ErrDuplicateName):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("같은 이름의 용도가 이미 있어요"))
	case errors.Is(err, purpose.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("용도를 찾을 수 없어요"))
	default:
		slog.Error(op+" failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New(op+" failed"))
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
