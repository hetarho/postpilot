// Package rpc is the voice context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

type Handler struct{ service *voice.Service }

func NewHandler(service *voice.Service) *Handler { return &Handler{service: service} }

func (h *Handler) GetVoiceProfile(ctx context.Context, _ *connect.Request[postpilotv1.GetVoiceProfileRequest]) (*connect.Response[postpilotv1.GetVoiceProfileResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.service.Get(ctx, userID)
	if err != nil {
		return nil, toConnectError("get voice profile", err)
	}
	return connect.NewResponse(&postpilotv1.GetVoiceProfileResponse{Profile: toProtoProfile(profile)}), nil
}

func (h *Handler) UpdateVoiceProfile(ctx context.Context, req *connect.Request[postpilotv1.UpdateVoiceProfileRequest]) (*connect.Response[postpilotv1.UpdateVoiceProfileResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	var profile voice.Profile
	switch {
	case req.Msg.Styleguide != nil && req.Msg.Rules != nil:
		profile, err = h.service.Update(ctx, userID, req.Msg.GetStyleguide(), req.Msg.GetRules())
	case req.Msg.Styleguide != nil:
		profile, err = h.service.UpdateStyleguide(ctx, userID, req.Msg.GetStyleguide())
	case req.Msg.Rules != nil:
		profile, err = h.service.UpdateRules(ctx, userID, req.Msg.GetRules())
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one profile field is required"))
	}
	if err != nil {
		return nil, toConnectError("update voice profile", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateVoiceProfileResponse{Profile: toProtoProfile(profile)}), nil
}

func (h *Handler) AddVoiceSample(ctx context.Context, req *connect.Request[postpilotv1.AddVoiceSampleRequest]) (*connect.Response[postpilotv1.AddVoiceSampleResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	ref := llm.ModelRef{
		ProviderID: req.Msg.GetModel().GetProviderId(), ModelID: req.Msg.GetModel().GetModelId(),
	}
	sample, jobID, err := h.service.AddSample(ctx, userID, req.Msg.GetLabel(), req.Msg.GetBody(), ref)
	if err != nil {
		return nil, toConnectError("add voice sample", err)
	}
	return connect.NewResponse(&postpilotv1.AddVoiceSampleResponse{
		Sample: toProtoSample(sample), JobId: jobID,
	}), nil
}

func (h *Handler) DeleteVoiceSample(ctx context.Context, req *connect.Request[postpilotv1.DeleteVoiceSampleRequest]) (*connect.Response[postpilotv1.DeleteVoiceSampleResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	jobID, err := h.service.DeleteSample(ctx, userID, req.Msg.GetSampleId())
	if err != nil {
		return nil, toConnectError("delete voice sample", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteVoiceSampleResponse{JobId: jobID}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	return userID, nil
}

func toConnectError(op string, err error) error {
	var tooShort *voice.SampleTooShortError
	switch {
	case errors.As(err, &tooShort):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, voice.ErrAnalyzeModelRequired):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, voice.ErrSampleNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	default:
		slog.Error(op+" failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New(op+" failed"))
	}
}

func toProtoProfile(profile voice.Profile) *postpilotv1.VoiceProfile {
	samples := make([]*postpilotv1.VoiceSample, 0, len(profile.Samples))
	for _, sample := range profile.Samples {
		samples = append(samples, toProtoSample(sample))
	}
	updated := ""
	if !profile.UpdatedAt.IsZero() {
		updated = profile.UpdatedAt.UTC().Format(timeLayout)
	}
	return &postpilotv1.VoiceProfile{
		Styleguide: profile.Styleguide, Rules: profile.Rules, UpdatedAt: updated,
		Samples: samples, ActiveJobId: profile.ActiveJobID,
	}
}

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func toProtoSample(sample voice.Sample) *postpilotv1.VoiceSample {
	return &postpilotv1.VoiceSample{
		Id: sample.ID, Label: sample.Label, Chars: int32(sample.Chars),
		CreatedAt: sample.CreatedAt.UTC().Format(timeLayout),
	}
}

var _ postpilotv1connect.VoiceServiceHandler = (*Handler)(nil)
