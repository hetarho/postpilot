package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (h *Handler) PrepareClipPreview(ctx context.Context, req *connect.Request[v1.PrepareClipPreviewRequest]) (*connect.Response[v1.PrepareClipPreviewResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPreviewUnavailable)
	}
	if req.Msg.Plan == nil {
		return nil, toConnectError(clip.ErrInvalid)
	}
	bytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req.Msg.Plan)
	if err != nil {
		return nil, toConnectError(clip.ErrInvalid)
	}
	digest := sha256.Sum256(bytes)
	if req.Msg.DraftHash != hex.EncodeToString(digest[:]) {
		return nil, toConnectError(clip.ErrInvalid)
	}
	result, err := h.generation.PreparePreview(ctx, user, req.Msg.ProjectId, int(req.Msg.ExpectedRevision), req.Msg.DraftHash, correctionPlan(req.Msg.Plan), req.Msg.ElementIds, int(req.Msg.AssetOffset))
	if err != nil {
		return nil, previewConnectError(err)
	}
	out := &v1.PrepareClipPreviewResponse{DraftHash: result.DraftHash, CanvasWidth: int32(result.Canvas.Width), CanvasHeight: int32(result.Canvas.Height), NextOffset: int32(result.NextOffset)}
	for _, a := range result.Assets {
		out.Assets = append(out.Assets, &v1.ClipPreviewAsset{Key: a.Key, InstanceId: a.InstanceID, Png: a.PNG, X: int32(a.X), Y: int32(a.Y), Width: int32(a.Width), Height: int32(a.Height), StartMs: int32(a.StartMS), EndMs: int32(a.EndMS), InMs: int32(a.InMS), OutMs: int32(a.OutMS), Dy: a.DY, Layer: int32(a.Layer), RepresentativeFrame: a.RepresentativeFrame})
	}
	for _, p := range result.Parity {
		switch p {
		case clip.PreviewSourceContrast:
			out.Parity = append(out.Parity, v1.ClipPreviewParity_CLIP_PREVIEW_PARITY_SOURCE_CONTRAST_FINAL_ONLY)
		case clip.PreviewAudioNormalization:
			out.Parity = append(out.Parity, v1.ClipPreviewParity_CLIP_PREVIEW_PARITY_AUDIO_NORMALIZATION_FINAL_ONLY)
		case clip.PreviewFrameTiming:
			out.Parity = append(out.Parity, v1.ClipPreviewParity_CLIP_PREVIEW_PARITY_BROWSER_FRAME_TIMING)
		}
	}
	if !previewResponseFits(out, h.generation.PreviewResponseLimit()) {
		return nil, toConnectError(clip.ErrPreviewTooLarge)
	}
	response := connect.NewResponse(out)
	response.Header().Set("Cache-Control", "private, no-store")
	return response, nil
}

func previewConnectError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return rpcserver.NewAppError(connect.CodeDeadlineExceeded, "clip preview preparation timed out", postpilotv1.FailureReason_CLIP_PREVIEW_TIMEOUT, nil)
	}
	if errors.Is(err, context.Canceled) {
		return connect.NewError(connect.CodeCanceled, context.Canceled)
	}
	return toConnectError(err)
}

// PNG bytes expand in Connect JSON. Bound the uncompressed response in both
// supported codecs, including all manifest fields, before writing any body.
func previewResponseFits(out *v1.PrepareClipPreviewResponse, limit int) bool {
	if proto.Size(out) > limit {
		return false
	}
	encoded, err := (protojson.MarshalOptions{EmitUnpopulated: true}).Marshal(out)
	return err == nil && len(encoded) <= limit
}

// GetClipCaptionPreview hands ② each caption's SVG fragment, its box and the
// safe area, built by the one style registry the renderer uses (CDS-83).
func (h *Handler) GetClipCaptionPreview(ctx context.Context, req *connect.Request[v1.GetClipCaptionPreviewRequest]) (*connect.Response[v1.GetClipCaptionPreviewResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPreviewUnavailable)
	}
	if req.Msg.Plan == nil {
		return nil, toConnectError(clip.ErrInvalid)
	}
	result, err := h.generation.CaptionPreviewOf(ctx, user, req.Msg.ProjectId, int(req.Msg.ExpectedRevision), correctionPlan(req.Msg.Plan))
	if err != nil {
		return nil, previewConnectError(err)
	}
	out := &v1.GetClipCaptionPreviewResponse{
		Ratio:    result.Ratio,
		Canvas:   &v1.ClipCanvasBox{Width: float64(result.Canvas.Width), Height: float64(result.Canvas.Height)},
		SafeArea: canvasBox(result.Canvas.Safe),
	}
	for _, f := range result.Fragments {
		out.Captions = append(out.Captions, &v1.ClipCaptionFragment{InstanceId: f.InstanceID, Svg: f.SVG,
			Box: canvasBox(f.Box), FontSize: f.FontSize, Style: f.Style, RepresentativeFrame: f.Sequence})
	}
	response := connect.NewResponse(out)
	response.Header().Set("Cache-Control", "private, no-store")
	return response, nil
}

func canvasBox(r clip.Region) *v1.ClipCanvasBox {
	return &v1.ClipCanvasBox{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

// GetClipCaptionStyleSamples hands ① every approved caption style drawn once by
// the same registry the render draws with, so the offer shows each style's own
// look (CDS-80, CDS-83). It carries nothing of this project but its ratio.
func (h *Handler) GetClipCaptionStyleSamples(ctx context.Context, req *connect.Request[v1.GetClipCaptionStyleSamplesRequest]) (*connect.Response[v1.GetClipCaptionStyleSamplesResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPreviewUnavailable)
	}
	result, err := h.generation.CaptionStyleSamples(ctx, user, req.Msg.ProjectId)
	if err != nil {
		return nil, previewConnectError(err)
	}
	out := &v1.GetClipCaptionStyleSamplesResponse{
		Ratio:  result.Ratio,
		Canvas: &v1.ClipCanvasBox{Width: float64(result.Canvas.Width), Height: float64(result.Canvas.Height)},
	}
	for _, f := range result.Fragments {
		out.Samples = append(out.Samples, &v1.ClipCaptionFragment{InstanceId: f.Style, Svg: f.SVG,
			Box: canvasBox(f.Box), FontSize: f.FontSize, Style: f.Style, RepresentativeFrame: f.Sequence})
	}
	response := connect.NewResponse(out)
	response.Header().Set("Cache-Control", "private, no-store")
	return response, nil
}
