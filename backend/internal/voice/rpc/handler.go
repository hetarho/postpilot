// Package rpc is the voice context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/voice"
)

type Handler struct{ service *voice.Service }

func NewHandler(service *voice.Service) *Handler { return &Handler{service: service} }

// --- directory ---

func (h *Handler) ListVoices(ctx context.Context, _ *connect.Request[postpilotv1.ListVoicesRequest]) (*connect.Response[postpilotv1.ListVoicesResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	voices, err := h.service.ListVoices(ctx, userID)
	if err != nil {
		return nil, toConnectError("list voices", err)
	}
	return connect.NewResponse(&postpilotv1.ListVoicesResponse{Voices: toProtoVoices(voices)}), nil
}

func (h *Handler) CreateVoice(ctx context.Context, req *connect.Request[postpilotv1.CreateVoiceRequest]) (*connect.Response[postpilotv1.CreateVoiceResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.SourceLanguage == nil {
		return nil, toConnectError("create voice", voice.ErrLanguageRequired)
	}
	sourceLanguage, err := languageFromProto(req.Msg.GetSourceLanguage())
	if err != nil {
		return nil, toConnectError("create voice", err)
	}
	// A seed is attached only when the client actually described a register; an absent
	// description must not drag the analyze-model requirement into plain creation.
	var seed *voice.VoiceSeed
	if strings.TrimSpace(req.Msg.GetDescription()) != "" {
		seed = &voice.VoiceSeed{
			Description: req.Msg.GetDescription(),
			AnalyzeModel: llm.ModelRef{
				ProviderID: req.Msg.GetAnalyzeModel().GetProviderId(), ModelID: req.Msg.GetAnalyzeModel().GetModelId(),
			},
		}
	}
	created, jobID, err := h.service.CreateVoice(ctx, userID, req.Msg.GetName(), sourceLanguage, seed)
	if err != nil {
		return nil, toConnectError("create voice", err)
	}
	return connect.NewResponse(&postpilotv1.CreateVoiceResponse{Voice: toProtoVoice(created), JobId: jobID}), nil
}

func (h *Handler) RenameVoice(ctx context.Context, req *connect.Request[postpilotv1.RenameVoiceRequest]) (*connect.Response[postpilotv1.RenameVoiceResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	renamed, err := h.service.RenameVoice(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetName())
	if err != nil {
		return nil, toConnectError("rename voice", err)
	}
	return connect.NewResponse(&postpilotv1.RenameVoiceResponse{Voice: toProtoVoice(renamed)}), nil
}

func (h *Handler) SetDefaultVoice(ctx context.Context, req *connect.Request[postpilotv1.SetDefaultVoiceRequest]) (*connect.Response[postpilotv1.SetDefaultVoiceResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	voices, err := h.service.SetDefaultVoice(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, toConnectError("set default voice", err)
	}
	return connect.NewResponse(&postpilotv1.SetDefaultVoiceResponse{Voices: toProtoVoices(voices)}), nil
}

func (h *Handler) DeleteVoice(ctx context.Context, req *connect.Request[postpilotv1.DeleteVoiceRequest]) (*connect.Response[postpilotv1.DeleteVoiceResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	deleted, err := h.service.DeleteVoice(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, toConnectError("delete voice", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteVoiceResponse{Voice: toProtoVoice(deleted)}), nil
}

func (h *Handler) RestoreVoice(ctx context.Context, req *connect.Request[postpilotv1.RestoreVoiceRequest]) (*connect.Response[postpilotv1.RestoreVoiceResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	restored, err := h.service.RestoreVoice(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, toConnectError("restore voice", err)
	}
	return connect.NewResponse(&postpilotv1.RestoreVoiceResponse{Voice: toProtoVoice(restored)}), nil
}

// --- profile ---

func (h *Handler) GetVoiceProfile(ctx context.Context, req *connect.Request[postpilotv1.GetVoiceProfileRequest]) (*connect.Response[postpilotv1.GetVoiceProfileResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.service.Get(ctx, userID, req.Msg.GetVoiceId())
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
	voiceID := req.Msg.GetVoiceId()
	var profile voice.Profile
	switch {
	case req.Msg.Styleguide != nil && req.Msg.Rules != nil:
		profile, err = h.service.Update(ctx, userID, voiceID, req.Msg.GetStyleguide(), req.Msg.GetRules())
	case req.Msg.Styleguide != nil:
		profile, err = h.service.UpdateStyleguide(ctx, userID, voiceID, req.Msg.GetStyleguide())
	case req.Msg.Rules != nil:
		profile, err = h.service.UpdateRules(ctx, userID, voiceID, req.Msg.GetRules())
	default:
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "at least one profile field is required", "VOICE_PROFILE_FIELD_REQUIRED", nil)
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
	sample, jobID, err := h.service.AddSample(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetLabel(), req.Msg.GetBody(), ref)
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
	jobID, err := h.service.DeleteSample(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetSampleId())
	if err != nil {
		return nil, toConnectError("delete voice sample", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteVoiceSampleResponse{JobId: jobID}), nil
}

func (h *Handler) ListVoiceProfileVersions(ctx context.Context, req *connect.Request[postpilotv1.ListVoiceProfileVersionsRequest]) (*connect.Response[postpilotv1.ListVoiceProfileVersionsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	versions, err := h.service.ListVersions(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, toConnectError("list voice versions", err)
	}
	out := make([]*postpilotv1.VoiceProfileVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, toProtoVersion(version))
	}
	return connect.NewResponse(&postpilotv1.ListVoiceProfileVersionsResponse{Versions: out}), nil
}

func (h *Handler) UpdateVoiceOverride(ctx context.Context, req *connect.Request[postpilotv1.UpdateVoiceOverrideRequest]) (*connect.Response[postpilotv1.UpdateVoiceOverrideResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.service.UpdateOverride(ctx, userID, req.Msg.GetVoiceId(), fromProtoLayer(req.Msg.GetLayer()), req.Msg.GetField(), req.Msg.Value)
	if err != nil {
		return nil, toConnectError("update voice override", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateVoiceOverrideResponse{Profile: toProtoProfile(profile)}), nil
}

func (h *Handler) RestoreVoiceProfile(ctx context.Context, req *connect.Request[postpilotv1.RestoreVoiceProfileRequest]) (*connect.Response[postpilotv1.RestoreVoiceProfileResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.service.RestoreVersion(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetVersion())
	if err != nil {
		return nil, toConnectError("restore voice profile", err)
	}
	return connect.NewResponse(&postpilotv1.RestoreVoiceProfileResponse{Profile: toProtoProfile(profile)}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return userID, nil
}

// toConnectError maps the context's sentinels to wire codes. A foreign voice is NotFound
// like an unknown one; a tombstone and every lifecycle refusal are FailedPrecondition so the
// client can offer the restore/reassign path instead of retrying.
func toConnectError(op string, err error) error {
	// Plan refusals are matched by type here rather than mapped by each service: the
	// admission gate lives at one seam (job enqueue), so its two failures must translate
	// identically wherever they surface.
	var quota *plan.QuotaError
	if errors.As(err, &quota) {
		return rpcserver.AppErrorFrom(connect.CodeResourceExhausted, quota)
	}
	var locked *plan.ModelLockedError
	if errors.As(err, &locked) {
		return rpcserver.AppErrorFrom(connect.CodePermissionDenied, locked)
	}
	var tooShort *voice.SampleTooShortError
	var badName *voice.VoiceNameError
	var longDescription *voice.VoiceDescriptionTooLongError
	var mismatch *voice.ContentLanguageMismatchError
	switch {
	case errors.As(err, &tooShort):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice sample is too short", "VOICE_SAMPLE_TOO_SHORT", map[string]string{"actual": fmt.Sprint(tooShort.Chars), "min": fmt.Sprint(voice.SampleMinChars)})
	case errors.As(err, &badName):
		if badName.Chars == 0 {
			return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice name is required", "VOICE_NAME_REQUIRED", nil)
		}
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice name is too long", "VOICE_NAME_TOO_LONG", map[string]string{"actual": fmt.Sprint(badName.Chars), "max": fmt.Sprint(voice.VoiceNameMaxChars)})
	case errors.As(err, &longDescription):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice description is too long", "VOICE_DESCRIPTION_TOO_LONG", map[string]string{"actual": fmt.Sprint(longDescription.Chars), "max": fmt.Sprint(voice.VoiceDescriptionMaxChars)})
	case errors.Is(err, voice.ErrVoiceRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice is required", "VOICE_REQUIRED", nil)
	case errors.Is(err, voice.ErrLanguageRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice source language is required", "VOICE_SOURCE_LANGUAGE_REQUIRED", nil)
	case errors.Is(err, voice.ErrLanguageUnsupported):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice source language is unsupported", "VOICE_SOURCE_LANGUAGE_UNSUPPORTED", nil)
	case errors.Is(err, voice.ErrVoiceNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice not found", "VOICE_NOT_FOUND", nil)
	case errors.Is(err, voice.ErrSampleNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice sample not found", "VOICE_SAMPLE_NOT_FOUND", nil)
	case errors.Is(err, voice.ErrSampleMutation):
		return rpcserver.NewAppError(connect.CodeInternal, "voice sample could not be updated", "VOICE_SAMPLE_MUTATION_FAILED", nil)
	case errors.Is(err, voice.ErrVoiceNameTaken):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "voice name already exists", "VOICE_NAME_TAKEN", nil)
	case errors.Is(err, voice.ErrVoiceDeleted):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice is deleted", "VOICE_DELETED", nil)
	case errors.Is(err, voice.ErrVoiceIsDefault):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "default voice cannot be deleted", "VOICE_DEFAULT_DELETE_FORBIDDEN", nil)
	case errors.Is(err, voice.ErrVoiceBusy):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice has unfinished work", "VOICE_BUSY", nil)
	case errors.Is(err, voice.ErrBaselineVoiceMismatch):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post baseline voice does not match current voice", "VOICE_BASELINE_MISMATCH", nil)
	case errors.As(err, &mismatch):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post content language does not match voice source language", "VOICE_CONTENT_LANGUAGE_MISMATCH", languageMismatchParams(mismatch))
	case errors.Is(err, voice.ErrAnalyzeModelRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "an enabled analyze model is required", "VOICE_ANALYZE_MODEL_REQUIRED", nil)
	case errors.Is(err, voice.ErrInvalidLifecycle):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice state does not allow this operation", "VOICE_INVALID_LIFECYCLE", nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", "UNKNOWN_FAILURE", nil)
	}
}

func languageMismatchParams(mismatch *voice.ContentLanguageMismatchError) map[string]string {
	params := map[string]string{}
	if mismatch.ContentLanguage.Valid() {
		params["content_language"] = string(mismatch.ContentLanguage)
	}
	if mismatch.SourceLanguage.Valid() {
		params["source_language"] = string(mismatch.SourceLanguage)
	}
	return params
}

func toProtoVoices(voices []voice.Voice) []*postpilotv1.Voice {
	out := make([]*postpilotv1.Voice, 0, len(voices))
	for _, v := range voices {
		out = append(out, toProtoVoice(v))
	}
	return out
}

func toProtoVoice(v voice.Voice) *postpilotv1.Voice {
	if v.ID == "" {
		return nil
	}
	deleted := ""
	if v.DeletedAt != nil {
		deleted = v.DeletedAt.UTC().Format(timeLayout)
	}
	return &postpilotv1.Voice{
		Id: v.ID, Name: v.Name, IsDefault: v.IsDefault, Deleted: v.Deleted(),
		CreatedAt: v.CreatedAt.UTC().Format(timeLayout), UpdatedAt: v.UpdatedAt.UTC().Format(timeLayout), DeletedAt: deleted,
		SourceLanguage: languageToProto(v.SourceLanguage),
	}
}

func languageFromProto(value postpilotv1.ContentLanguage) (voice.Language, error) {
	switch value {
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:
		return voice.LanguageKorean, nil
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH:
		return voice.LanguageEnglish, nil
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED:
		return "", voice.ErrLanguageRequired
	default:
		return "", voice.ErrLanguageUnsupported
	}
}

func languageToProto(value voice.Language) postpilotv1.ContentLanguage {
	switch value {
	case voice.LanguageKorean:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case voice.LanguageEnglish:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
}

func toProtoFailure(value *voice.Failure) *postpilotv1.Failure {
	if value == nil || value.Empty() {
		return nil
	}
	params := make(map[string]string, len(value.Params))
	for key, item := range value.Params {
		params[key] = item
	}
	return &postpilotv1.Failure{Reason: value.Reason, Params: params, TechnicalDetail: value.TechnicalDetail}
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
		Voice:      toProtoVoice(profile.Voice),
		Styleguide: profile.Styleguide, Rules: profile.Rules, UpdatedAt: updated,
		Samples: samples, ActiveJobId: profile.ActiveJobID,
		Structured: toProtoStructured(profile.Structured), LegacyManualGuidance: legacyGuidance(profile),
		FinalizedSourceCount: int32(profile.SourceCount), CanValidate: profile.CanValidate,
	}
}

func legacyGuidance(profile voice.Profile) string {
	if profile.Styleguide == "" {
		return profile.Rules
	}
	if profile.Rules == "" {
		return profile.Styleguide
	}
	return profile.Styleguide + "\n" + profile.Rules
}

func toProtoStructured(p voice.StructuredProfile) *postpilotv1.StructuredVoiceProfile {
	words := make([]*postpilotv1.WeightedWord, 0, len(p.Lexical.PreferredWords))
	for _, v := range p.Lexical.PreferredWords {
		words = append(words, &postpilotv1.WeightedWord{Word: v.Word, Alternatives: v.Alternatives, Weight: int32(v.Weight)})
	}
	bannedWords := make([]*postpilotv1.BannedItem, 0, len(p.Lexical.BannedWords))
	for _, v := range p.Lexical.BannedWords {
		bannedWords = append(bannedWords, &postpilotv1.BannedItem{Value: v.Value, Reason: v.Reason})
	}
	bannedPatterns := make([]*postpilotv1.BannedItem, 0, len(p.Lexical.BannedPatterns))
	for _, v := range p.Lexical.BannedPatterns {
		bannedPatterns = append(bannedPatterns, &postpilotv1.BannedItem{Value: v.Value, Reason: v.Reason})
	}
	ending := make([]*postpilotv1.EndingRatio, 0, len(p.Endings.Distribution))
	for _, v := range p.Endings.Distribution {
		ending = append(ending, &postpilotv1.EndingRatio{Ending: v.Ending, Ratio: v.Ratio})
	}
	rules := make([]*postpilotv1.VoiceContrastRule, 0, len(p.Rules))
	for _, v := range p.Rules {
		rules = append(rules, toProtoRule(v))
	}
	sources := make([]*postpilotv1.VoiceSource, 0, len(p.Sources))
	for _, v := range p.Sources {
		sources = append(sources, &postpilotv1.VoiceSource{Id: v.ID, PostSlug: v.PostSlug, Title: v.Title, Tags: v.Tags, Excerpt: v.Excerpt, HasEmbedding: v.EmbeddingRef != "", CreatedAt: v.CreatedAt.UTC().Format(timeLayout)})
	}
	feedback := make([]*postpilotv1.VoiceFeedbackRef, 0, len(p.Feedback))
	for _, v := range p.Feedback {
		feedback = append(feedback, &postpilotv1.VoiceFeedbackRef{Id: v.ID, PostSlug: v.PostSlug, Kind: v.Kind, Layer: toProtoLayer(voice.RuleLayer(v.Reason)), ProcessingState: v.ProcessingState, CreatedAt: v.CreatedAt.UTC().Format(timeLayout)})
	}
	updated := ""
	if !p.UpdatedAt.IsZero() {
		updated = p.UpdatedAt.UTC().Format(timeLayout)
	}
	return &postpilotv1.StructuredVoiceProfile{Meta: &postpilotv1.VoiceProfileMeta{Version: p.Version, UpdatedAt: updated, SourceCount: int32(p.SourceCount)}, Lexical: &postpilotv1.VoiceLexical{PreferredWords: words, BannedWords: bannedWords, BannedPatterns: bannedPatterns, Description: toProtoValue(p.Lexical.Description)}, Endings: &postpilotv1.VoiceEndings{BaseRegister: toProtoValue(p.Endings.BaseRegister), Distribution: ending, BannedEndings: p.Endings.BannedEndings, SignatureEndings: p.Endings.SignatureEndings, Constraints: p.Endings.Constraints}, Syntax: &postpilotv1.VoiceSyntax{AverageSentenceChars: p.Syntax.AverageSentenceChars, AverageSentenceWords: p.Syntax.AverageSentenceWords, SentenceLength: toProtoValue(p.Syntax.SentenceLength), ConnectiveStyle: toProtoValue(p.Syntax.ConnectiveStyle), PreferredConnectives: p.Syntax.PreferredConnectives, Nominalization: toProtoValue(p.Syntax.Nominalization), PassiveTendency: toProtoValue(p.Syntax.PassiveTendency)}, Structure: &postpilotv1.VoiceStructure{IntroPattern: toProtoValue(p.Structure.IntroPattern), ClosingPattern: toProtoValue(p.Structure.ClosingPattern), ParagraphSentencesMin: int32(p.Structure.ParagraphSentencesMin), ParagraphSentencesMax: int32(p.Structure.ParagraphSentencesMax), HeadingHabit: toProtoValue(p.Structure.HeadingHabit), ListHabit: toProtoValue(p.Structure.ListHabit), EmojiUse: toProtoValue(p.Structure.EmojiUse)}, Axes: &postpilotv1.VoiceAxes{Involvement: toProtoAxis(p.Axes.Involvement), Narrativity: toProtoAxis(p.Axes.Narrativity), PersuasionOvertness: toProtoAxis(p.Axes.PersuasionOvertness), Abstractness: toProtoAxis(p.Axes.Abstractness), AddresseeFocus: toProtoAxis(p.Axes.AddresseeFocus), Humor: toProtoAxis(p.Axes.Humor)}, ContrastRules: rules, FewShotBank: sources, FeedbackLog: feedback, Empty: p.Empty}
}
func toProtoValue(v voice.VoiceValue) *postpilotv1.VoiceValue {
	return &postpilotv1.VoiceValue{Value: v.Value, Source: toProtoSource(v.Source), Unknown: v.Unknown}
}
func toProtoSource(v voice.ValueSource) postpilotv1.VoiceValueSource {
	switch v {
	case voice.SourceMeasured:
		return postpilotv1.VoiceValueSource_VOICE_VALUE_SOURCE_MEASURED
	case voice.SourceAnalyzed:
		return postpilotv1.VoiceValueSource_VOICE_VALUE_SOURCE_ANALYZED
	case voice.SourceManual:
		return postpilotv1.VoiceValueSource_VOICE_VALUE_SOURCE_MANUAL
	default:
		return postpilotv1.VoiceValueSource_VOICE_VALUE_SOURCE_UNKNOWN
	}
}

// nil stays nil so the wire carries absence; the FE renders it as unknown.
func toProtoAxis(v *int) *int32 {
	if v == nil {
		return nil
	}
	value := int32(*v)
	return &value
}
func toProtoLayer(v voice.RuleLayer) postpilotv1.VoiceLayer {
	switch v {
	case voice.LayerLexical:
		return postpilotv1.VoiceLayer_VOICE_LAYER_LEXICAL
	case voice.LayerEndings:
		return postpilotv1.VoiceLayer_VOICE_LAYER_ENDINGS
	case voice.LayerSyntax:
		return postpilotv1.VoiceLayer_VOICE_LAYER_SYNTAX
	case voice.LayerStructure:
		return postpilotv1.VoiceLayer_VOICE_LAYER_STRUCTURE
	case voice.LayerAxes:
		return postpilotv1.VoiceLayer_VOICE_LAYER_AXES
	default:
		return postpilotv1.VoiceLayer_VOICE_LAYER_UNSPECIFIED
	}
}
func fromProtoLayer(v postpilotv1.VoiceLayer) voice.RuleLayer {
	switch v {
	case postpilotv1.VoiceLayer_VOICE_LAYER_LEXICAL:
		return voice.LayerLexical
	case postpilotv1.VoiceLayer_VOICE_LAYER_ENDINGS:
		return voice.LayerEndings
	case postpilotv1.VoiceLayer_VOICE_LAYER_SYNTAX:
		return voice.LayerSyntax
	case postpilotv1.VoiceLayer_VOICE_LAYER_STRUCTURE:
		return voice.LayerStructure
	case postpilotv1.VoiceLayer_VOICE_LAYER_AXES:
		return voice.LayerAxes
	default:
		return ""
	}
}
func toProtoRule(v voice.ContrastRule) *postpilotv1.VoiceContrastRule {
	status := postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_UNSPECIFIED
	switch v.Status {
	case voice.RuleCandidate:
		status = postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_CANDIDATE
	case voice.RuleActive:
		status = postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_ACTIVE
	case voice.RuleRetired:
		status = postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_RETIRED
	case voice.RuleRejected:
		status = postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_REJECTED
	}
	return &postpilotv1.VoiceContrastRule{Id: v.ID, Statement: v.Statement, Layer: toProtoLayer(v.Layer), EvidenceCount: int32(v.EvidenceCount), Status: status, Origin: v.Origin, CreatedAt: v.CreatedAt.UTC().Format(timeLayout), LastEvidenceAt: v.LastEvidenceAt.UTC().Format(timeLayout)}
}
func toProtoVersion(v voice.ProfileVersion) *postpilotv1.VoiceProfileVersion {
	return &postpilotv1.VoiceProfileVersion{Version: v.Version, Profile: toProtoStructured(v.Profile), Origin: v.Origin, RestoredFromVersion: v.RestoredFromVersion, CreatedAt: v.CreatedAt.UTC().Format(timeLayout)}
}

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func toProtoSample(sample voice.Sample) *postpilotv1.VoiceSample {
	return &postpilotv1.VoiceSample{
		Id: sample.ID, Label: sample.Label, Chars: int32(sample.Chars),
		CreatedAt: sample.CreatedAt.UTC().Format(timeLayout),
	}
}

var _ postpilotv1connect.VoiceServiceHandler = (*Handler)(nil)
