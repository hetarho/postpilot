package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"time"
)

func sourceBatchProto(b clip.SourceBatch) *v1.ClipSourceBatch {
	out := &v1.ClipSourceBatch{Id: b.ID, ProjectId: b.ProjectID, State: b.State, ExpiresAt: b.ExpiresAt.UTC().Format(time.RFC3339Nano)}
	for _, v := range b.Sources {
		out.Sources = append(out.Sources, &v1.ClipSource{Id: v.ID, State: v.State, ActualBytes: v.ActualBytes, Metadata: &v1.ClipSourceMetadata{Filename: v.Filename, ContentType: v.ContentType, Bytes: v.Bytes, DurationMs: int32(v.DurationMS), Width: int32(v.Width), Height: int32(v.Height), Fingerprint: v.Fingerprint}})
	}
	return out
}
func (h *Handler) CreateClipSourceBatch(ctx context.Context, req *connect.Request[v1.CreateClipSourceBatchRequest]) (*connect.Response[v1.CreateClipSourceBatchResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.sources == nil {
		return nil, toConnectError(clip.ErrSourceState)
	}
	manifest := make([]clip.SourceMetadata, 0, len(req.Msg.Sources))
	for _, v := range req.Msg.Sources {
		manifest = append(manifest, clip.SourceMetadata{Filename: v.GetFilename(), ContentType: v.GetContentType(), Bytes: v.GetBytes(), DurationMS: int(v.GetDurationMs()), Width: int(v.GetWidth()), Height: int(v.GetHeight()), Fingerprint: v.GetFingerprint()})
	}
	upload, err := h.sources.Create(ctx, user, req.Msg.ProjectId, manifest)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := &v1.CreateClipSourceBatchResponse{Batch: sourceBatchProto(upload.Batch)}
	for _, v := range upload.Uploads {
		out.Uploads = append(out.Uploads, &v1.ClipSourceUpload{SourceId: v.SourceID, PutUrl: v.URL, Headers: v.Headers, ExpiresAt: v.ExpiresAt.UTC().Format(time.RFC3339Nano)})
	}
	return connect.NewResponse(out), nil
}
func (h *Handler) ConfirmClipSource(ctx context.Context, req *connect.Request[v1.ConfirmClipSourceRequest]) (*connect.Response[v1.ConfirmClipSourceResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.sources == nil {
		return nil, toConnectError(clip.ErrSourceState)
	}
	b, err := h.sources.Confirm(ctx, user, req.Msg.BatchId, req.Msg.SourceId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.ConfirmClipSourceResponse{Batch: sourceBatchProto(b)}), nil
}
func (h *Handler) DiscardClipSourceBatch(ctx context.Context, req *connect.Request[v1.DiscardClipSourceBatchRequest]) (*connect.Response[v1.DiscardClipSourceBatchResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.sources == nil {
		return nil, toConnectError(clip.ErrSourceState)
	}
	if err := h.sources.Discard(ctx, user, req.Msg.BatchId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.DiscardClipSourceBatchResponse{}), nil
}
