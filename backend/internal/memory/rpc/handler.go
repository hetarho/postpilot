// Package rpc is the memory context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Handler struct{ service *memory.Service }

func NewHandler(service *memory.Service) *Handler { return &Handler{service: service} }

func (h *Handler) ListMemories(ctx context.Context, _ *connect.Request[postpilotv1.ListMemoriesRequest]) (*connect.Response[postpilotv1.ListMemoriesResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	memories, err := h.service.List(ctx, userID)
	if err != nil {
		return nil, toConnectError("list memories", err)
	}
	out := make([]*postpilotv1.Memory, 0, len(memories))
	for _, m := range memories {
		out = append(out, toProtoMemory(m))
	}
	return connect.NewResponse(&postpilotv1.ListMemoriesResponse{Memories: out}), nil
}

func (h *Handler) CreateMemory(ctx context.Context, req *connect.Request[postpilotv1.CreateMemoryRequest]) (*connect.Response[postpilotv1.CreateMemoryResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := fromProtoKind(req.Msg.GetKind())
	if err != nil {
		return nil, toConnectError("create memory", err)
	}
	created, deduplicated, err := h.service.Create(ctx, userID, req.Msg.GetText(), kind, req.Msg.GetTags(), req.Msg.GetSourcePostSlug())
	if err != nil {
		return nil, toConnectError("create memory", err)
	}
	return connect.NewResponse(&postpilotv1.CreateMemoryResponse{
		Memory: toProtoMemory(created), Deduplicated: deduplicated,
	}), nil
}

func (h *Handler) UpdateMemory(ctx context.Context, req *connect.Request[postpilotv1.UpdateMemoryRequest]) (*connect.Response[postpilotv1.UpdateMemoryResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	// Presence is the edit unit. For the tags that means MESSAGE presence: an absent patch
	// leaves the set alone, and a present one replaces it — which is what lets a user clear
	// every tag off a memory.
	patch := memory.Patch{}
	if req.Msg.Text != nil {
		patch.Text = req.Msg.Text
	}
	if req.Msg.Kind != nil {
		kind, err := fromProtoKind(req.Msg.GetKind())
		if err != nil {
			return nil, toConnectError("update memory", err)
		}
		patch.Kind = &kind
	}
	if sent := req.Msg.GetTags(); sent != nil {
		tags := sent.GetTags()
		if tags == nil {
			tags = []string{}
		}
		patch.Tags = &tags
	}
	updated, err := h.service.Update(ctx, userID, req.Msg.GetId(), patch)
	if err != nil {
		return nil, toConnectError("update memory", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateMemoryResponse{Memory: toProtoMemory(updated)}), nil
}

func (h *Handler) DeleteMemory(ctx context.Context, req *connect.Request[postpilotv1.DeleteMemoryRequest]) (*connect.Response[postpilotv1.DeleteMemoryResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.Delete(ctx, userID, req.Msg.GetId()); err != nil {
		return nil, toConnectError("delete memory", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteMemoryResponse{}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return userID, nil
}

// fromProtoKind refuses UNSPECIFIED rather than defaulting to one of the five: the kind
// decides whether a fact is a candidate for every post or only on tag overlap, and a client
// that forgot the field would otherwise have that decided for it (MEM-12).
func fromProtoKind(kind postpilotv1.MemoryKind) (memory.Kind, error) {
	switch kind {
	case postpilotv1.MemoryKind_MEMORY_KIND_PREFERENCE:
		return memory.KindPreference, nil
	case postpilotv1.MemoryKind_MEMORY_KIND_PERSONA:
		return memory.KindPersona, nil
	case postpilotv1.MemoryKind_MEMORY_KIND_PLACE:
		return memory.KindPlace, nil
	case postpilotv1.MemoryKind_MEMORY_KIND_PERSON:
		return memory.KindPerson, nil
	case postpilotv1.MemoryKind_MEMORY_KIND_HISTORY:
		return memory.KindHistory, nil
	default:
		return "", memory.ErrInvalidKind
	}
}

func toProtoKind(kind memory.Kind) postpilotv1.MemoryKind {
	switch kind {
	case memory.KindPreference:
		return postpilotv1.MemoryKind_MEMORY_KIND_PREFERENCE
	case memory.KindPersona:
		return postpilotv1.MemoryKind_MEMORY_KIND_PERSONA
	case memory.KindPlace:
		return postpilotv1.MemoryKind_MEMORY_KIND_PLACE
	case memory.KindPerson:
		return postpilotv1.MemoryKind_MEMORY_KIND_PERSON
	case memory.KindHistory:
		return postpilotv1.MemoryKind_MEMORY_KIND_HISTORY
	default:
		return postpilotv1.MemoryKind_MEMORY_KIND_UNSPECIFIED
	}
}

// toConnectError maps the context's sentinels to wire codes. A foreign memory is NotFound
// like an unknown one — the two must not be distinguishable.
func toConnectError(op string, err error) error {
	var tooLong *memory.TextTooLongError
	var tooManyTags *memory.TooManyTagsError
	var atCap *memory.AccountCapError
	switch {
	case errors.As(err, &tooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "memory text is too long", postpilotv1.FailureReason_MEMORY_TEXT_TOO_LONG, map[string]string{
			"max": strconv.Itoa(tooLong.Max), "actual": strconv.Itoa(tooLong.Chars),
		})
	case errors.As(err, &tooManyTags):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "memory carries too many tags", postpilotv1.FailureReason_MEMORY_TAGS_TOO_MANY, map[string]string{
			"max": strconv.Itoa(tooManyTags.Max), "actual": strconv.Itoa(tooManyTags.Count),
		})
	case errors.As(err, &atCap):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "memory limit reached", postpilotv1.FailureReason_MEMORY_LIMIT_REACHED, map[string]string{
			"max": strconv.Itoa(atCap.Max),
		})
	case errors.Is(err, memory.ErrInvalidText):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "memory text is required", postpilotv1.FailureReason_MEMORY_TEXT_REQUIRED, nil)
	case errors.Is(err, memory.ErrInvalidKind):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "memory kind is invalid", postpilotv1.FailureReason_MEMORY_KIND_INVALID, nil)
	case errors.Is(err, memory.ErrInvalidTag):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "memory tag is empty", postpilotv1.FailureReason_MEMORY_TAG_REQUIRED, nil)
	case errors.Is(err, memory.ErrDuplicateText):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "memory text already exists", postpilotv1.FailureReason_MEMORY_TEXT_TAKEN, nil)
	case errors.Is(err, memory.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "memory not found", postpilotv1.FailureReason_MEMORY_NOT_FOUND, nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
}

func toProtoMemory(m memory.Memory) *postpilotv1.Memory {
	if m.ID == "" {
		return nil
	}
	return &postpilotv1.Memory{
		Id: m.ID, Text: m.Text, Kind: toProtoKind(m.Kind),
		Tags: append([]string(nil), m.Tags...), SourcePostSlugs: append([]string(nil), m.SourcePostSlugs...),
		CreatedAt:  m.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt:  m.UpdatedAt.UTC().Format(timeLayout),
		LastSeenAt: m.LastSeenAt.UTC().Format(timeLayout),
	}
}
