package rpc

import (
	"context"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func (h *Handler) StartClipBrowserCompositionRender(ctx context.Context, req *connect.Request[v1.StartClipBrowserCompositionRenderRequest]) (*connect.Response[v1.StartClipBrowserCompositionRenderResponse], error) {
	user, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrRenderUnavailable)
	}
	id, c, e := h.generation.StartBrowserCompositionRender(ctx, user, req.Msg.ProjectId, req.Msg.BatchId, int(req.Msg.ExpectedRevision), req.Msg.CompositionVersion)
	if e != nil {
		return nil, toConnectError(e)
	}
	out := connect.NewResponse(&v1.StartClipBrowserCompositionRenderResponse{RenderId: id, CompositionVersion: c.Version, SnapshotFingerprint: c.SnapshotFingerprint, ComponentVersion: c.Components, FontVersion: c.Fonts, AssetVersion: c.Assets})
	out.Header().Set("Cache-Control", "private, no-store")
	return out, nil
}
