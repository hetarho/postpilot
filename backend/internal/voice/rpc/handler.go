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

func (h *Handler) ListVoiceProfileVersions(ctx context.Context, _ *connect.Request[postpilotv1.ListVoiceProfileVersionsRequest]) (*connect.Response[postpilotv1.ListVoiceProfileVersionsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	versions, err := h.service.ListVersions(ctx, userID)
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
	profile, err := h.service.UpdateOverride(ctx, userID, fromProtoLayer(req.Msg.GetLayer()), req.Msg.GetField(), req.Msg.Value)
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
	profile, err := h.service.RestoreVersion(ctx, userID, req.Msg.GetVersion())
	if err != nil {
		return nil, toConnectError("restore voice profile", err)
	}
	return connect.NewResponse(&postpilotv1.RestoreVoiceProfileResponse{Profile: toProtoProfile(profile)}), nil
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
	return &postpilotv1.StructuredVoiceProfile{Meta: &postpilotv1.VoiceProfileMeta{Version: p.Version, UpdatedAt: updated, SourceCount: int32(p.SourceCount)}, Lexical: &postpilotv1.VoiceLexical{PreferredWords: words, BannedWords: bannedWords, BannedPatterns: bannedPatterns, Description: toProtoValue(p.Lexical.Description)}, Endings: &postpilotv1.VoiceEndings{BaseRegister: toProtoValue(p.Endings.BaseRegister), Distribution: ending, BannedEndings: p.Endings.BannedEndings, SignatureEndings: p.Endings.SignatureEndings, Constraints: p.Endings.Constraints}, Syntax: &postpilotv1.VoiceSyntax{AverageSentenceChars: p.Syntax.AverageSentenceChars, SentenceLength: toProtoValue(p.Syntax.SentenceLength), ConnectiveStyle: toProtoValue(p.Syntax.ConnectiveStyle), PreferredConnectives: p.Syntax.PreferredConnectives, Nominalization: toProtoValue(p.Syntax.Nominalization), PassiveTendency: toProtoValue(p.Syntax.PassiveTendency)}, Structure: &postpilotv1.VoiceStructure{IntroPattern: toProtoValue(p.Structure.IntroPattern), ClosingPattern: toProtoValue(p.Structure.ClosingPattern), ParagraphSentencesMin: int32(p.Structure.ParagraphSentencesMin), ParagraphSentencesMax: int32(p.Structure.ParagraphSentencesMax), HeadingHabit: toProtoValue(p.Structure.HeadingHabit), ListHabit: toProtoValue(p.Structure.ListHabit), EmojiUse: toProtoValue(p.Structure.EmojiUse)}, Axes: &postpilotv1.VoiceAxes{Involvement: toProtoAxis(p.Axes.Involvement), Narrativity: toProtoAxis(p.Axes.Narrativity), PersuasionOvertness: toProtoAxis(p.Axes.PersuasionOvertness), Abstractness: toProtoAxis(p.Axes.Abstractness), AddresseeFocus: toProtoAxis(p.Axes.AddresseeFocus), Humor: toProtoAxis(p.Axes.Humor)}, ContrastRules: rules, FewShotBank: sources, FeedbackLog: feedback, Empty: p.Empty}
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
