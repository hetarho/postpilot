// Package rpc is the voice context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
	created, err := h.service.CreateVoice(ctx, userID, req.Msg.GetName())
	if err != nil {
		return nil, toConnectError("create voice", err)
	}
	return connect.NewResponse(&postpilotv1.CreateVoiceResponse{Voice: toProtoVoice(created)}), nil
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

func (h *Handler) AddVoiceSample(ctx context.Context, req *connect.Request[postpilotv1.AddVoiceSampleRequest]) (*connect.Response[postpilotv1.AddVoiceSampleResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	sample, err := h.service.AddSample(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetLabel(), req.Msg.GetBody())
	if err != nil {
		return nil, toConnectError("add voice sample", err)
	}
	return connect.NewResponse(&postpilotv1.AddVoiceSampleResponse{Sample: toProtoSample(sample)}), nil
}

func (h *Handler) DeleteVoiceSample(ctx context.Context, req *connect.Request[postpilotv1.DeleteVoiceSampleRequest]) (*connect.Response[postpilotv1.DeleteVoiceSampleResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.DeleteSample(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetSampleId()); err != nil {
		return nil, toConnectError("delete voice sample", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteVoiceSampleResponse{}), nil
}

func (h *Handler) GetVoiceSample(ctx context.Context, req *connect.Request[postpilotv1.GetVoiceSampleRequest]) (*connect.Response[postpilotv1.GetVoiceSampleResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	sample, photoURL, err := h.service.GetSample(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetSampleId())
	if err != nil {
		return nil, toConnectError("get voice sample", err)
	}
	return connect.NewResponse(&postpilotv1.GetVoiceSampleResponse{
		Sample: toProtoSample(sample), Body: sample.Body, PhotoUrl: photoURL,
		PhotoWidth: int32(sample.PhotoWidth), PhotoHeight: int32(sample.PhotoHeight),
	}), nil
}

func (h *Handler) ListVoicePrompts(ctx context.Context, _ *connect.Request[postpilotv1.ListVoicePromptsRequest]) (*connect.Response[postpilotv1.ListVoicePromptsResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	prompts := h.service.Prompts()
	out := make([]*postpilotv1.VoicePrompt, 0, len(prompts))
	for _, prompt := range prompts {
		out = append(out, &postpilotv1.VoicePrompt{Key: prompt.Key, Part: toProtoPart(prompt.Part), Photo: prompt.Photo, Text: prompt.Text})
	}
	return connect.NewResponse(&postpilotv1.ListVoicePromptsResponse{Prompts: out}), nil
}

func (h *Handler) CreateVoicePhotoUpload(ctx context.Context, req *connect.Request[postpilotv1.CreateVoicePhotoUploadRequest]) (*connect.Response[postpilotv1.CreateVoicePhotoUploadResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	upload, url, err := h.service.CreatePhotoUpload(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetPromptKey())
	if err != nil {
		return nil, toConnectError("create voice photo upload", err)
	}
	return connect.NewResponse(&postpilotv1.CreateVoicePhotoUploadResponse{
		UploadId: upload.ID, PutUrl: url, ContentType: voice.PhotoContentType, ExpiresAt: upload.ExpiresAt.UTC().Format(timeLayout),
	}), nil
}

func (h *Handler) AnswerVoicePrompt(ctx context.Context, req *connect.Request[postpilotv1.AnswerVoicePromptRequest]) (*connect.Response[postpilotv1.AnswerVoicePromptResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	sample, err := h.service.AnswerPrompt(ctx, userID, req.Msg.GetVoiceId(), voice.Answer{
		PromptKey: req.Msg.GetPromptKey(), Body: req.Msg.GetBody(), UploadID: req.Msg.GetUploadId(),
		PhotoWidth: int(req.Msg.GetPhotoWidth()), PhotoHeight: int(req.Msg.GetPhotoHeight()),
	})
	if err != nil {
		return nil, toConnectError("answer voice prompt", err)
	}
	return connect.NewResponse(&postpilotv1.AnswerVoicePromptResponse{Sample: toProtoSample(sample)}), nil
}

func (h *Handler) AnalyzeVoice(ctx context.Context, req *connect.Request[postpilotv1.AnalyzeVoiceRequest]) (*connect.Response[postpilotv1.AnalyzeVoiceResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	ref := llm.ModelRef{ProviderID: req.Msg.GetModel().GetProviderId(), ModelID: req.Msg.GetModel().GetModelId()}
	jobID, err := h.service.AnalyzeVoice(ctx, userID, req.Msg.GetVoiceId(), ref)
	if err != nil {
		return nil, toConnectError("analyze voice", err)
	}
	return connect.NewResponse(&postpilotv1.AnalyzeVoiceResponse{JobId: jobID}), nil
}

func (h *Handler) RestorePreviousVoiceAnalysis(ctx context.Context, req *connect.Request[postpilotv1.RestorePreviousVoiceAnalysisRequest]) (*connect.Response[postpilotv1.RestorePreviousVoiceAnalysisResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.service.RestorePreviousAnalysis(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, toConnectError("restore previous voice analysis", err)
	}
	return connect.NewResponse(&postpilotv1.RestorePreviousVoiceAnalysisResponse{Profile: toProtoProfile(profile)}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return userID, nil
}

// toConnectError maps the context's sentinels to wire codes. A foreign voice is NotFound
// like an unknown one; a tombstone and every lifecycle refusal are FailedPrecondition so the
// client can offer the restore/reassign path instead of retrying.
func toConnectError(op string, err error) error {
	// The credit refusal is matched by type here rather than mapped by each service: the
	// gate lives at one seam (job enqueue), so its failure must translate identically
	// wherever it surfaces.
	var credits *plan.InsufficientCreditsError
	if errors.As(err, &credits) {
		return rpcserver.AppErrorFrom(connect.CodeResourceExhausted, credits)
	}
	var tooShort *voice.SampleTooShortError
	var badName *voice.VoiceNameError
	switch {
	case errors.As(err, &tooShort):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice sample is too short", postpilotv1.FailureReason_VOICE_SAMPLE_TOO_SHORT, map[string]string{"actual": fmt.Sprint(tooShort.Chars), "min": fmt.Sprint(voice.SampleMinChars)})
	case errors.As(err, &badName):
		if badName.Chars == 0 {
			return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice name is required", postpilotv1.FailureReason_VOICE_NAME_REQUIRED, nil)
		}
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice name is too long", postpilotv1.FailureReason_VOICE_NAME_TOO_LONG, map[string]string{"actual": fmt.Sprint(badName.Chars), "max": fmt.Sprint(voice.VoiceNameMaxChars)})
	case errors.Is(err, voice.ErrVoiceRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voice is required", postpilotv1.FailureReason_VOICE_REQUIRED, nil)
	case errors.Is(err, voice.ErrVoiceNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice not found", postpilotv1.FailureReason_VOICE_NOT_FOUND, nil)
	case errors.Is(err, voice.ErrSampleNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice sample not found", postpilotv1.FailureReason_VOICE_SAMPLE_NOT_FOUND, nil)
	case errors.Is(err, voice.ErrNoPreviousAnalysis):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "the voice has no previous analysis", postpilotv1.FailureReason_VOICE_NO_PREVIOUS_ANALYSIS, nil)
	case errors.Is(err, voice.ErrVoiceNotReady):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "the voice needs more 학습 글", postpilotv1.FailureReason_VOICE_NOT_READY, nil)
	case errors.Is(err, voice.ErrPromptNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice prompt not found", postpilotv1.FailureReason_VOICE_PROMPT_NOT_FOUND, nil)
	case errors.Is(err, voice.ErrPromptAnswered):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "the prompt already holds an answer", postpilotv1.FailureReason_VOICE_PROMPT_ANSWERED, nil)
	case errors.Is(err, voice.ErrAnswerRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "an answer is required", postpilotv1.FailureReason_VOICE_ANSWER_REQUIRED, nil)
	case errors.Is(err, voice.ErrPhotoRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "a photo prompt needs an uploaded photo", postpilotv1.FailureReason_VOICE_PHOTO_REQUIRED, nil)
	case errors.Is(err, voice.ErrInvalidPhoto):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid uploaded image", postpilotv1.FailureReason_UPLOAD_INVALID, nil)
	case errors.Is(err, voice.ErrVoiceNameTaken):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "voice name already exists", postpilotv1.FailureReason_VOICE_NAME_TAKEN, nil)
	case errors.Is(err, voice.ErrVoiceDeleted):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice is deleted", postpilotv1.FailureReason_VOICE_DELETED, nil)
	case errors.Is(err, voice.ErrVoiceNotMade):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice is not made yet", postpilotv1.FailureReason_VOICE_NOT_MADE, nil)
	case errors.Is(err, voice.ErrVoiceBusy):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice has unfinished work", postpilotv1.FailureReason_VOICE_BUSY, nil)
	case errors.Is(err, voice.ErrAnalyzeModelRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "an enabled analyze model is required", postpilotv1.FailureReason_VOICE_ANALYZE_MODEL_REQUIRED, nil)
	case errors.Is(err, voice.ErrInvalidLifecycle):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice state does not allow this operation", postpilotv1.FailureReason_VOICE_INVALID_LIFECYCLE, nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
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
	analyzed := ""
	if v.AnalyzedAt != nil {
		analyzed = v.AnalyzedAt.UTC().Format(timeLayout)
	}
	return &postpilotv1.Voice{
		Id: v.ID, Name: v.Name, IsDefault: v.IsDefault, Deleted: v.Deleted(),
		CreatedAt: v.CreatedAt.UTC().Format(timeLayout), UpdatedAt: v.UpdatedAt.UTC().Format(timeLayout), DeletedAt: deleted,
		Made: v.Made, MaterialCount: int32(v.SampleCount), AnalyzedAt: analyzed, ReadinessPercent: int32(v.ReadinessPercent),
	}
}

func toProtoProfile(profile voice.Profile) *postpilotv1.VoiceProfile {
	samples := make([]*postpilotv1.VoiceSample, 0, len(profile.Samples))
	for _, sample := range profile.Samples {
		samples = append(samples, toProtoSample(sample))
	}
	out := &postpilotv1.VoiceProfile{
		Voice:       toProtoVoice(profile.Voice),
		Samples:     samples,
		ActiveJobId: profile.ActiveJobID,
		Made:        profile.Analysis != nil,
		Readiness:   toProtoReadiness(profile.Readiness),
		HasPrevious: profile.HasPrevious,
		Notice:      toProtoNotice(profile.Notice),
	}
	if profile.Analysis != nil {
		out.Analysis = toProtoAnalysis(*profile.Analysis)
	}
	return out
}

func toProtoNotice(notice voice.Notice) *postpilotv1.VoiceNotice {
	switch notice.Kind {
	case voice.NoticeAdded:
		return &postpilotv1.VoiceNotice{Kind: postpilotv1.VoiceNoticeKind_VOICE_NOTICE_KIND_ADDED, Count: int32(notice.Count)}
	case voice.NoticeChanged:
		return &postpilotv1.VoiceNotice{Kind: postpilotv1.VoiceNoticeKind_VOICE_NOTICE_KIND_CHANGED}
	}
	return nil
}

func toProtoAnalysis(analysis voice.Analysis) *postpilotv1.VoiceAnalysis {
	ai := &postpilotv1.VoiceAiPart{Impression: analysis.AI.Impression, SignaturePhrases: analysis.AI.SignaturePhrases}
	for _, tic := range analysis.AI.Tics {
		ai.Tics = append(ai.Tics, &postpilotv1.VoiceTic{Phrase: tic.Phrase, When: tic.When})
	}
	for _, example := range analysis.AI.Examples {
		ai.Examples = append(ai.Examples, &postpilotv1.VoiceAiExample{Field: toProtoAIField(example.Field), Sentence: example.Sentence, MaterialId: example.MaterialID})
	}
	return &postpilotv1.VoiceAnalysis{
		Counted:       ToProtoFingerprint(analysis.Counted),
		Ai:            ai,
		MaterialCount: int32(len(analysis.MaterialIDs)),
		AnalyzeModel:  analysis.AnalyzeModel,
		CreatedAt:     analysis.CreatedAt.UTC().Format(timeLayout),
	}
}

// toProtoAIField maps the three fields the domain has, pinned by a test that walks the
// generated enum (ARCH-3).
func toProtoAIField(field voice.AIField) postpilotv1.VoiceAiField {
	switch field {
	case voice.AIImpression:
		return postpilotv1.VoiceAiField_VOICE_AI_FIELD_IMPRESSION
	case voice.AITics:
		return postpilotv1.VoiceAiField_VOICE_AI_FIELD_TICS
	case voice.AISignaturePhrases:
		return postpilotv1.VoiceAiField_VOICE_AI_FIELD_SIGNATURE_PHRASES
	}
	return postpilotv1.VoiceAiField_VOICE_AI_FIELD_UNSPECIFIED
}

func toProtoExample(example voice.Example) *postpilotv1.VoiceExample {
	if example.Sentence == "" {
		return nil
	}
	return &postpilotv1.VoiceExample{Sentence: example.Sentence, MaterialId: example.MaterialID}
}

// ToProtoFingerprint is the counted items on the wire, shared by every screen that shows them
// (the analysis, ②'s comparison, 검증).
func ToProtoFingerprint(f voice.Fingerprint) *postpilotv1.VoiceFingerprint {
	suffixes := make([]*postpilotv1.VoiceSuffix, 0, len(f.Endings.Suffixes))
	for _, suffix := range f.Endings.Suffixes {
		suffixes = append(suffixes, &postpilotv1.VoiceSuffix{Text: suffix.Text, Count: int32(suffix.Count)})
	}
	words := make([]*postpilotv1.VoiceWordRate, 0, len(f.Adverbs.Words))
	for _, word := range f.Adverbs.Words {
		words = append(words, &postpilotv1.VoiceWordRate{Word: word.Word, PerHundred: word.PerHundred})
	}
	return &postpilotv1.VoiceFingerprint{
		Sentences: int32(f.Sentences),
		Endings: &postpilotv1.VoiceEndingsItem{Unknown: f.Endings.Unknown, Da: f.Endings.Da, Haeyo: f.Endings.Haeyo, Seumnida: f.Endings.Seumnida,
			Other: f.Endings.Other, Suffixes: suffixes, Example: toProtoExample(f.Endings.Example)},
		Marks: &postpilotv1.VoiceMarksItem{Unknown: f.Marks.Unknown, Exclaim: f.Marks.Exclaim, Question: f.Marks.Question, Tilde: f.Marks.Tilde,
			Ellipsis: f.Marks.Ellipsis, Period: f.Marks.Period, None: f.Marks.None, Repeat: f.Marks.Repeat, Example: toProtoExample(f.Marks.Example)},
		Emoji: &postpilotv1.VoiceEmojiItem{Unknown: f.Emoji.Unknown, Emoji: f.Emoji.Emoji, Hh: f.Emoji.Hh, Kk: f.Emoji.Kk, Tears: f.Emoji.Tears,
			Example: toProtoExample(f.Emoji.Example)},
		Shape: &postpilotv1.VoiceShapeItem{Unknown: f.Shape.Unknown, AverageChars: f.Shape.AverageChars, ParagraphAverage: f.Shape.ParagraphAverage,
			ParagraphMin: int32(f.Shape.ParagraphMin), ParagraphMax: int32(f.Shape.ParagraphMax), LineBreakShare: f.Shape.LineBreakShare,
			OwnLine: f.Shape.OwnLine, Example: toProtoExample(f.Shape.Example)},
		Openings: &postpilotv1.VoiceOpeningsItem{Unknown: f.OpenClose.Unknown, Openings: f.OpenClose.Openings, Closings: f.OpenClose.Closings,
			Example: toProtoExample(f.OpenClose.Example)},
		Adverbs: &postpilotv1.VoiceAdverbsItem{Unknown: f.Adverbs.Unknown, None: f.Adverbs.None, Words: words, Example: toProtoExample(f.Adverbs.Example)},
		Person: &postpilotv1.VoicePersonItem{Unknown: f.Person.Unknown, Jeo: f.Person.Jeo, Uri: f.Person.Uri, Na: f.Person.Na,
			Dominant: f.Person.Dominant, Example: toProtoExample(f.Person.Example)},
		Headings: &postpilotv1.VoiceHeadingsItem{Unknown: f.Headings.Unknown, Count: int32(f.Headings.Count), EmojiShare: f.Headings.EmojiShare,
			QuestionShare: f.Headings.QuestionShare, NumberedShare: f.Headings.NumberedShare, ListShare: f.Headings.ListShare,
			Marker: f.Headings.Marker, Example: toProtoExample(f.Headings.Example)},
	}
}

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func toProtoSample(sample voice.Sample) *postpilotv1.VoiceSample {
	kind := postpilotv1.VoiceSampleKind_VOICE_SAMPLE_KIND_POST
	if sample.Kind == voice.SampleKindAnswer {
		kind = postpilotv1.VoiceSampleKind_VOICE_SAMPLE_KIND_ANSWER
	}
	return &postpilotv1.VoiceSample{
		Id: sample.ID, Label: sample.Label, Chars: int32(sample.Chars),
		CreatedAt: sample.CreatedAt.UTC().Format(timeLayout),
		Kind:      kind, PromptKey: sample.PromptKey, HasPhoto: sample.HasPhoto(),
	}
}

func toProtoReadiness(readiness voice.Readiness) *postpilotv1.VoiceReadiness {
	missing := make([]postpilotv1.VoicePromptPart, 0, len(readiness.MissingParts))
	for _, part := range readiness.MissingParts {
		missing = append(missing, toProtoPart(part))
	}
	return &postpilotv1.VoiceReadiness{
		Percent: int32(readiness.Percent), Sentences: int32(readiness.Sentences), Needed: int32(readiness.Needed), MissingParts: missing,
	}
}

// toProtoPart maps the three parts the domain has; a fourth is a compile-time impossibility,
// pinned by a test that walks the generated enum (ARCH-3).
func toProtoPart(part voice.PromptPart) postpilotv1.VoicePromptPart {
	switch part {
	case voice.PartOpening:
		return postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_OPENING
	case voice.PartDescription:
		return postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_DESCRIPTION
	case voice.PartClosing:
		return postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_CLOSING
	}
	return postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_UNSPECIFIED
}

var _ postpilotv1connect.VoiceServiceHandler = (*Handler)(nil)
