package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func (h *Handler) CancelClipBrowserRender(ctx context.Context, req *connect.Request[v1.CancelClipBrowserRenderRequest]) (*connect.Response[v1.CancelClipBrowserRenderResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrRenderUnavailable)
	}
	cancelled, err := h.generation.CancelBrowserRender(ctx, user, req.Msg.RenderId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.CancelClipBrowserRenderResponse{Cancelled: cancelled}), nil
}

func (h *Handler) PrepareClipRenderUpload(ctx context.Context, req *connect.Request[v1.PrepareClipRenderUploadRequest]) (*connect.Response[v1.PrepareClipRenderUploadResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrRenderUnavailable)
	}
	put, err := h.generation.PrepareBrowserUpload(ctx, user, req.Msg.RenderId, req.Msg.Bytes)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.PrepareClipRenderUploadResponse{PutUrl: put.URL, Headers: put.Headers}), nil
}

func (h *Handler) CompleteClipRenderUpload(ctx context.Context, req *connect.Request[v1.CompleteClipRenderUploadRequest]) (*connect.Response[v1.CompleteClipRenderUploadResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrRenderUnavailable)
	}
	p, err := h.generation.CompleteBrowserUpload(ctx, user, req.Msg.RenderId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.CompleteClipRenderUploadResponse{Project: projectProto(p)}), nil
}
