package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/voice"
)

type ValidationHandler struct{ service *voice.Service }

func NewValidationHandler(service *voice.Service) *ValidationHandler {
	return &ValidationHandler{service: service}
}
func (h *ValidationHandler) StartVoiceRuleComparison(ctx context.Context, req *connect.Request[postpilotv1.StartVoiceRuleComparisonRequest]) (*connect.Response[postpilotv1.StartVoiceRuleComparisonResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	id, jobID, err := h.service.StartRuleComparison(ctx, userID, req.Msg.GetRuleId(), req.Msg.GetSourceId(), optionalLength(req.Msg.TargetLength), learningModel(req.Msg.GetWriteModel()))
	if err != nil {
		return nil, validationError("start rule comparison", err)
	}
	return connect.NewResponse(&postpilotv1.StartVoiceRuleComparisonResponse{ComparisonId: id, JobId: jobID}), nil
}
func (h *ValidationHandler) GetVoiceRuleComparison(ctx context.Context, req *connect.Request[postpilotv1.GetVoiceRuleComparisonRequest]) (*connect.Response[postpilotv1.GetVoiceRuleComparisonResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.GetRuleComparison(ctx, userID, req.Msg.GetComparisonId())
	if err != nil {
		return nil, validationError("get rule comparison", err)
	}
	return connect.NewResponse(&postpilotv1.GetVoiceRuleComparisonResponse{Comparison: toProtoComparison(found)}), nil
}
func (h *ValidationHandler) DecideVoiceRuleComparison(ctx context.Context, req *connect.Request[postpilotv1.DecideVoiceRuleComparisonRequest]) (*connect.Response[postpilotv1.DecideVoiceRuleComparisonResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.DecideRuleComparison(ctx, userID, req.Msg.GetComparisonId(), req.Msg.GetChosenSide())
	if err != nil {
		return nil, validationError("decide rule comparison", err)
	}
	return connect.NewResponse(&postpilotv1.DecideVoiceRuleComparisonResponse{Comparison: toProtoComparison(found)}), nil
}
func (h *ValidationHandler) RetryVoiceRuleComparison(ctx context.Context, req *connect.Request[postpilotv1.RetryVoiceRuleComparisonRequest]) (*connect.Response[postpilotv1.RetryVoiceRuleComparisonResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	jobID, err := h.service.RetryRuleComparison(ctx, userID, req.Msg.GetComparisonId())
	if err != nil {
		return nil, validationError("retry rule comparison", err)
	}
	return connect.NewResponse(&postpilotv1.RetryVoiceRuleComparisonResponse{JobId: jobID}), nil
}
func (h *ValidationHandler) StartVoiceProfileValidation(ctx context.Context, req *connect.Request[postpilotv1.StartVoiceProfileValidationRequest]) (*connect.Response[postpilotv1.StartVoiceProfileValidationResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	id, jobID, err := h.service.StartValidation(ctx, userID, req.Msg.GetVoiceId(), learningModel(req.Msg.GetAnalyzeModel()), learningModel(req.Msg.GetWriteModel()), req.Msg.GetJudgeEnabled())
	if err != nil {
		return nil, validationError("start profile validation", err)
	}
	return connect.NewResponse(&postpilotv1.StartVoiceProfileValidationResponse{ValidationId: id, JobId: jobID}), nil
}
func (h *ValidationHandler) GetVoiceProfileValidation(ctx context.Context, req *connect.Request[postpilotv1.GetVoiceProfileValidationRequest]) (*connect.Response[postpilotv1.GetVoiceProfileValidationResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.GetValidation(ctx, userID, req.Msg.GetValidationId())
	if err != nil {
		return nil, validationError("get profile validation", err)
	}
	return connect.NewResponse(&postpilotv1.GetVoiceProfileValidationResponse{Validation: toProtoValidation(found)}), nil
}
func (h *ValidationHandler) ListVoiceProfileValidations(ctx context.Context, req *connect.Request[postpilotv1.ListVoiceProfileValidationsRequest]) (*connect.Response[postpilotv1.ListVoiceProfileValidationsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.service.ListValidations(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, validationError("list profile validations", err)
	}
	out := make([]*postpilotv1.VoiceProfileValidation, 0, len(items))
	for _, item := range items {
		out = append(out, toProtoValidation(item))
	}
	return connect.NewResponse(&postpilotv1.ListVoiceProfileValidationsResponse{Validations: out}), nil
}
func (h *ValidationHandler) RetryVoiceProfileValidation(ctx context.Context, req *connect.Request[postpilotv1.RetryVoiceProfileValidationRequest]) (*connect.Response[postpilotv1.RetryVoiceProfileValidationResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	jobID, err := h.service.RetryValidation(ctx, userID, req.Msg.GetValidationId())
	if err != nil {
		return nil, validationError("retry profile validation", err)
	}
	return connect.NewResponse(&postpilotv1.RetryVoiceProfileValidationResponse{JobId: jobID}), nil
}

func toProtoComparison(v voice.RuleComparison) *postpilotv1.VoiceRuleComparison {
	candidates := make([]*postpilotv1.VoiceComparisonCandidate, 0, len(v.Candidates))
	for _, c := range v.Candidates {
		candidates = append(candidates, &postpilotv1.VoiceComparisonCandidate{Id: c.ID, Side: c.DisplaySide, Output: c.Output, Status: c.Status, Error: "", Failure: toProtoFailure(c.Failure)})
	}
	return &postpilotv1.VoiceRuleComparison{Id: v.ID, RuleId: v.RuleID, ProfileVersion: v.ProfileVersion, TargetLength: protoOptionalLength(v.TargetLength), Status: v.Status, JobId: v.JobID, Candidates: candidates, ChosenSide: v.ChosenSide, CreatedAt: v.CreatedAt.UTC().Format(timeLayout), VoiceId: v.VoiceID, SourceLanguage: languageToProto(v.SourceLanguage)}
}

func optionalLength(value *int32) *int {
	if value == nil {
		return nil
	}
	result := int(*value)
	return &result
}

func protoOptionalLength(value *int) *int32 {
	if value == nil {
		return nil
	}
	result := int32(*value)
	return &result
}
func toProtoValidation(v voice.ProfileValidation) *postpilotv1.VoiceProfileValidation {
	items := make([]*postpilotv1.VoiceValidationItem, 0, len(v.Items))
	for _, item := range v.Items {
		var values map[string]bool
		_ = json.Unmarshal([]byte(item.ScoresJSON), &values)
		scores := make([]*postpilotv1.VoiceValidationScore, 0, len(values))
		for _, key := range []string{"endings", "sentence_rhythm", "opening_closing", "vocabulary", "addressee"} {
			if matched, ok := values[key]; ok {
				scores = append(scores, &postpilotv1.VoiceValidationScore{Dimension: key, Matched: matched})
			}
		}
		items = append(items, &postpilotv1.VoiceValidationItem{Id: item.ID, SourceId: item.SourceID, Original: item.Original, NeutralSummary: item.NeutralSummary, Regenerated: item.Regenerated, Scores: scores, Status: item.Status, Error: "", Failure: toProtoFailure(item.Failure)})
	}
	finished := ""
	if v.FinishedAt != nil {
		finished = v.FinishedAt.UTC().Format(timeLayout)
	}
	return &postpilotv1.VoiceProfileValidation{Id: v.ID, ProfileVersion: v.ProfileVersion, JudgeEnabled: v.JudgeEnabled, Status: v.Status, JobId: v.JobID, Items: items, YCount: int32(v.YCount), TotalCount: int32(v.TotalCount), CreatedAt: v.CreatedAt.UTC().Format(timeLayout), FinishedAt: finished, VoiceId: v.VoiceID, SourceLanguage: languageToProto(v.SourceLanguage)}
}
func validationError(op string, err error) error {
	switch {
	case errors.Is(err, voice.ErrComparisonNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice rule comparison not found", "VOICE_COMPARISON_NOT_FOUND", nil)
	case errors.Is(err, voice.ErrValidationNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice profile validation not found", "VOICE_VALIDATION_NOT_FOUND", nil)
	case errors.Is(err, voice.ErrRuleNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice rule not found", "VOICE_RULE_NOT_FOUND", nil)
	case errors.Is(err, voice.ErrInsufficientSources):
		minimum := voice.DefaultValidationPostCount
		var insufficient *voice.InsufficientSourcesError
		if errors.As(err, &insufficient) && insufficient.Minimum > 0 {
			minimum = insufficient.Minimum
		}
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "not enough authored sources", "VOICE_INSUFFICIENT_SOURCES", map[string]string{"min": fmt.Sprint(minimum)})
	case errors.Is(err, voice.ErrInvalidLifecycle):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice validation state does not allow this operation", "VOICE_INVALID_LIFECYCLE", nil)
	case errors.Is(err, voice.ErrAnalyzeModelRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "enabled analyze and write models are required", "VOICE_ANALYZE_MODEL_REQUIRED", nil)
	default:
		return toConnectError(op, err)
	}
}

var _ postpilotv1connect.VoiceValidationServiceHandler = (*ValidationHandler)(nil)
