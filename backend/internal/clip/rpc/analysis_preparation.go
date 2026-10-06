package rpc

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// Absence is a legal older/native-only deployment. No browser request ever
// falls back to a native preparation operation.
func (h *Handler) WithAnalysisPreparations(a *clipapp.AnalysisPreparations) *Handler {
	h.analysisPreparations = a
	return h
}

func analysisPreparationMessage(p clip.AnalysisPreparation) *pb.ClipAnalysisPreparationResponse {
	cfg := clip.DefaultMediaConfig(clip.Environment{})
	out := &pb.ClipAnalysisPreparationResponse{PreparationId: p.ID, ProjectId: p.ProjectID, BatchId: p.BatchID, ExpectedRevision: int32(p.ExpectedRevision), State: p.State, ExpiresAt: p.ExpiresAt.UTC().Format(time.RFC3339Nano), OriginalMeasurementProvenance: clip.BrowserOriginalProvenance, Progress: int32(p.Progress), Failure: p.Failure, JobId: p.ParentJobID, Profile: &pb.ClipAnalysisProfile{Version: clip.BrowserAnalysisProfileVersion, IntervalMs: int32(cfg.ChunkDurationMS), LongEdge: int32(cfg.LongEdge), FramesPerSecond: int32(cfg.FPS), MaxCopyBytes: cfg.AnalysisMaxBytes, VideoCodec: clip.AnalysisVideoCodec, PixelFormat: clip.AnalysisPixelFormat, AudioCodec: clip.AnalysisAudioCodec, AudioRate: int32(cfg.AudioRate), AudioChannels: clip.AnalysisAudioChannels, AudioBitrate: int32(cfg.AudioBitrate), Qualified: clip.BrowserAnalysisQualified(p.ProfileVersion)}}
	for _, c := range p.Copies {
		out.Copies = append(out.Copies, &pb.ClipAnalysisCopySlot{Slot: c.Slot, SourceId: c.SourceID, Fingerprint: c.Fingerprint, Ordinal: int32(c.Index), OffsetMs: int32(c.OffsetMS), DurationMs: int32(c.DurationMS), Width: int32(c.Width), Height: int32(c.Height), HasAudio: c.HasAudio, State: c.State, Bytes: c.Bytes, Sha256: c.Digest})
	}
	return out
}
func analysisPreparationResponse(p clip.AnalysisPreparation) *connect.Response[pb.ClipAnalysisPreparationResponse] {
	r := connect.NewResponse(analysisPreparationMessage(p))
	r.Header().Set("Cache-Control", "private, no-store")
	return r
}
func (h *Handler) BeginClipAnalysisPreparation(ctx context.Context, r *connect.Request[pb.BeginClipAnalysisPreparationRequest]) (*connect.Response[pb.ClipAnalysisPreparationResponse], error) {
	user, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.analysisPreparations == nil {
		return nil, toConnectError(clip.ErrMediaUnsupported)
	}
	in := clip.AnalysisPreparationInput{ProjectID: r.Msg.ProjectId, BatchID: r.Msg.BatchId, QuoteID: r.Msg.QuoteId, ExpectedRevision: int(r.Msg.ExpectedRevision), ProfileVersion: r.Msg.ProfileVersion}
	for _, m := range r.Msg.Originals {
		if m == nil {
			return nil, toConnectError(clip.ErrInvalid)
		}
		in.Originals = append(in.Originals, clip.BrowserOriginalMeasurement{SourceID: m.SourceId, Fingerprint: m.Fingerprint, Info: clip.MediaInfo{DurationMS: int(m.DurationMs), Width: int(m.Width), Height: int(m.Height), FrameRateNumerator: int(m.FrameRateNumerator), FrameRateDenominator: int(m.FrameRateDenominator), CadenceVerified: m.CadenceVerified, DecodedFrames: int(m.DecodedFrames), HasAudio: m.HasAudio, AudioRate: int(m.AudioRate), AudioChannels: int(m.AudioChannels)}})
	}
	p, e := h.analysisPreparations.Begin(ctx, user, in)
	if e != nil {
		return nil, toConnectError(e)
	}
	return analysisPreparationResponse(p), nil
}
func (h *Handler) ReserveClipAnalysisCopy(ctx context.Context, r *connect.Request[pb.ReserveClipAnalysisCopyRequest]) (*connect.Response[pb.ReserveClipAnalysisCopyResponse], error) {
	user, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.analysisPreparations == nil {
		return nil, toConnectError(clip.ErrMediaUnsupported)
	}
	a, expires, e := h.analysisPreparations.Reserve(ctx, user, r.Msg.PreparationId, r.Msg.Slot, r.Msg.Bytes, r.Msg.Sha256)
	if e != nil {
		return nil, toConnectError(e)
	}
	out := connect.NewResponse(&pb.ReserveClipAnalysisCopyResponse{Slot: a.Slot, PutUrl: a.URL, Headers: a.Headers, ExpiresAt: expires.UTC().Format(time.RFC3339Nano)})
	out.Header().Set("Cache-Control", "private, no-store")
	return out, nil
}
func (h *Handler) CompleteClipAnalysisPreparation(ctx context.Context, r *connect.Request[pb.CompleteClipAnalysisPreparationRequest]) (*connect.Response[pb.ClipAnalysisPreparationResponse], error) {
	user, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.analysisPreparations == nil {
		return nil, toConnectError(clip.ErrMediaUnsupported)
	}
	p, e := h.analysisPreparations.Complete(ctx, user, r.Msg.PreparationId)
	if e != nil {
		return nil, toConnectError(e)
	}
	return analysisPreparationResponse(p), nil
}
func (h *Handler) CancelClipAnalysisPreparation(ctx context.Context, r *connect.Request[pb.CancelClipAnalysisPreparationRequest]) (*connect.Response[pb.ClipAnalysisPreparationResponse], error) {
	user, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.analysisPreparations == nil {
		return nil, toConnectError(clip.ErrMediaUnsupported)
	}
	p, e := h.analysisPreparations.Cancel(ctx, user, r.Msg.PreparationId)
	if e != nil {
		return nil, toConnectError(e)
	}
	return analysisPreparationResponse(p), nil
}
