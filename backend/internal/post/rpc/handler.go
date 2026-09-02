// Package rpc is the post context's transport edge: proto ↔ domain mapping and the
// translation of domain errors into Connect codes.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
)

// timeLayout is how timestamps cross the wire. RFC3339 rather than an epoch so the
// client can render one without knowing a unit.
const timeLayout = time.RFC3339

// Handler implements postpilotv1connect.PostServiceHandler.
type Handler struct {
	svc *post.Service
}

// NewHandler returns the PostService implementation.
func NewHandler(svc *post.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) SavePostDraft(ctx context.Context, req *connect.Request[postpilotv1.SavePostDraftRequest]) (*connect.Response[postpilotv1.SavePostDraftResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}

	targetLanguage, err := optionalLanguageFromProto(req.Msg.TargetLanguage)
	if err != nil {
		reason := "POST_TARGET_LANGUAGE_UNSUPPORTED"
		if req.Msg.TargetLanguage != nil && *req.Msg.TargetLanguage == postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED {
			reason = "POST_TARGET_LANGUAGE_REQUIRED"
		}
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid post target language", reason, nil)
	}
	saved, err := h.svc.SaveDraft(ctx, userID, req.Msg.GetSlug(), req.Msg.GetTitle(), req.Msg.GetMemo(), req.Msg.VoiceId, req.Msg.PurposeId, targetLanguage)
	if err != nil {
		return nil, toConnectError("save draft", err)
	}
	return connect.NewResponse(&postpilotv1.SavePostDraftResponse{Post: toProtoPost(saved)}), nil
}

func (h *Handler) SavePostContent(ctx context.Context, req *connect.Request[postpilotv1.SavePostContentRequest]) (*connect.Response[postpilotv1.SavePostContentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	content, err := fromProtoContent(req.Msg.GetContent())
	if err != nil {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid post content", "POST_CONTENT_INVALID", nil)
	}
	saved, err := h.svc.SaveContent(ctx, userID, req.Msg.GetSlug(), content, req.Msg.GetExpectedRevision())
	if err != nil {
		return nil, toConnectError("save post content", err)
	}
	return connect.NewResponse(&postpilotv1.SavePostContentResponse{Post: toProtoPost(saved)}), nil
}

func (h *Handler) SavePostGenerationOptions(ctx context.Context, req *connect.Request[postpilotv1.SavePostGenerationOptionsRequest]) (*connect.Response[postpilotv1.SavePostGenerationOptionsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	saved, err := h.svc.SaveGenerationOptions(ctx, userID, req.Msg.GetSlug(), optionalTargetLength(req.Msg.TargetLength))
	if err != nil {
		return nil, toConnectError("save post generation options", err)
	}
	return connect.NewResponse(&postpilotv1.SavePostGenerationOptionsResponse{Post: toProtoPost(saved)}), nil
}

func (h *Handler) FinalizePost(ctx context.Context, req *connect.Request[postpilotv1.FinalizePostRequest]) (*connect.Response[postpilotv1.FinalizePostResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	saved, err := h.svc.Finalize(ctx, userID, req.Msg.GetSlug(), req.Msg.GetExpectedRevision())
	if err != nil {
		return nil, toConnectError("finalize post", err)
	}
	return connect.NewResponse(&postpilotv1.FinalizePostResponse{Post: toProtoPost(saved)}), nil
}

func (h *Handler) GetPost(ctx context.Context, req *connect.Request[postpilotv1.GetPostRequest]) (*connect.Response[postpilotv1.GetPostResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}

	found, err := h.svc.Get(ctx, userID, req.Msg.GetSlug())
	if err != nil {
		return nil, toConnectError("get post", err)
	}
	return connect.NewResponse(&postpilotv1.GetPostResponse{Post: toProtoPost(found)}), nil
}

func (h *Handler) ListPosts(ctx context.Context, _ *connect.Request[postpilotv1.ListPostsRequest]) (*connect.Response[postpilotv1.ListPostsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}

	summaries, err := h.svc.List(ctx, userID)
	if err != nil {
		return nil, toConnectError("list posts", err)
	}

	posts := make([]*postpilotv1.PostSummary, 0, len(summaries))
	for _, s := range summaries {
		posts = append(posts, &postpilotv1.PostSummary{
			Slug:                s.Slug,
			Title:               s.Title,
			Status:              s.Status,
			UpdatedAt:           s.UpdatedAt.UTC().Format(timeLayout),
			ActiveJob:           toProtoActiveJob(s.ActiveJob),
			PendingExperimentId: s.PendingExperimentID,
			Voice:               toProtoVoiceRef(s.Voice),
			Purpose:             toProtoPurposeRef(s.Purpose),
			TargetLanguage:      languageToProto(s.TargetLanguage),
			ContentLanguage:     optionalLanguageToProto(s.ContentLanguage),
		})
	}
	return connect.NewResponse(&postpilotv1.ListPostsResponse{Posts: posts}), nil
}

func (h *Handler) DeletePost(ctx context.Context, req *connect.Request[postpilotv1.DeletePostRequest]) (*connect.Response[postpilotv1.DeletePostResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeletePost(ctx, userID, req.Msg.GetSlug()); err != nil {
		return nil, toConnectError("delete post", err)
	}
	return connect.NewResponse(&postpilotv1.DeletePostResponse{}), nil
}

func (h *Handler) CreateUpload(ctx context.Context, req *connect.Request[postpilotv1.CreateUploadRequest]) (*connect.Response[postpilotv1.CreateUploadResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}

	upload, putURL, contentType, err := h.svc.CreateUpload(ctx, userID, req.Msg.GetPostSlug(), req.Msg.GetFilename())
	if err != nil {
		return nil, toConnectError("create upload", err)
	}
	return connect.NewResponse(&postpilotv1.CreateUploadResponse{
		UploadId:    upload.ID,
		PutUrl:      putURL,
		ContentType: contentType,
		ExpiresAt:   upload.ExpiresAt.UTC().Format(timeLayout),
	}), nil
}

func (h *Handler) ConfirmUpload(ctx context.Context, req *connect.Request[postpilotv1.ConfirmUploadRequest]) (*connect.Response[postpilotv1.ConfirmUploadResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}

	image, err := h.svc.ConfirmUpload(ctx, userID, req.Msg.GetUploadId(), req.Msg.GetWidth(), req.Msg.GetHeight())
	if err != nil {
		return nil, toConnectError("confirm upload", err)
	}
	// No view URL here: the client already holds the bytes it just uploaded, and the
	// next GetPost mints a fresh one.
	return connect.NewResponse(&postpilotv1.ConfirmUploadResponse{Image: toProtoImage(image)}), nil
}

func (h *Handler) DeleteImage(ctx context.Context, req *connect.Request[postpilotv1.DeleteImageRequest]) (*connect.Response[postpilotv1.DeleteImageResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.svc.DeleteImage(ctx, userID, req.Msg.GetImageId()); err != nil {
		return nil, toConnectError("delete image", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteImageResponse{}), nil
}

// actingUser reads the user the interceptor authenticated. Reaching a handler without
// one would mean the procedure was mounted outside the interceptor — a wiring bug, not
// a client error, so it fails closed and loudly.
func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return userID, nil
}

// toConnectError maps a domain error to a wire code. Anything unrecognized is Internal
// with the detail kept in the log — an unexpected failure must not leak a SQL string or
// a bucket name to a client.
func toConnectError(op string, err error) error {
	switch {
	case errors.Is(err, post.ErrNotFound):
		if op == "confirm upload" {
			return rpcserver.NewAppError(connect.CodeNotFound, "upload not found", "UPLOAD_NOT_FOUND", nil)
		}
		return rpcserver.NewAppError(connect.CodeNotFound, "post resource not found", "POST_NOT_FOUND", nil)
	case errors.Is(err, post.ErrForbidden):
		return rpcserver.NewAppError(connect.CodePermissionDenied, "post belongs to another user", "POST_FORBIDDEN", nil)
	case errors.Is(err, post.ErrDuplicateFilename):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "photo filename already exists", "POST_FILENAME_TAKEN", nil)
	case errors.Is(err, post.ErrInvalidImage):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid uploaded image", "UPLOAD_INVALID", nil)
	case errors.Is(err, post.ErrObjectMissing):
		// FailedPrecondition, not NotFound: the upload record is fine, the object just
		// is not there yet — the client should retry the PUT, not give up.
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "uploaded object is missing", "UPLOAD_OBJECT_MISSING", nil)
	case errors.Is(err, post.ErrPostBusy):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post has an active job", "POST_BUSY", nil)
	case errors.Is(err, post.ErrPostPublishing):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post has a live publish job", "POST_PUBLISHING", nil)
	case errors.Is(err, post.ErrStaleContentRevision):
		return rpcserver.NewAppError(connect.CodeAborted, "post content revision is stale", "POST_CONTENT_STALE", nil)
	case errors.Is(err, post.ErrNoMachineBaseline):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post has no machine baseline", "POST_MACHINE_BASELINE_REQUIRED", nil)
	case errors.Is(err, post.ErrPostNotFinalized):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post is not finalized", "POST_NOT_FINALIZED", nil)
	case errors.Is(err, post.ErrInvalidContent):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid post content", "POST_CONTENT_INVALID", nil)
	case errors.Is(err, post.ErrVoiceRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "post voice is required", "VOICE_REQUIRED", nil)
	case errors.Is(err, post.ErrLanguageRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "post target language is required", "POST_TARGET_LANGUAGE_REQUIRED", nil)
	case errors.Is(err, post.ErrVoiceNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voice not found", "VOICE_NOT_FOUND", nil)
	case errors.Is(err, post.ErrPurposeNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "purpose not found", "PURPOSE_NOT_FOUND", nil)
	case errors.Is(err, post.ErrVoiceDeleted):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voice is deleted", "VOICE_DELETED", nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", "UNKNOWN_FAILURE", nil)
	}
}

func toProtoPost(p post.Post) *postpilotv1.Post {
	images := make([]*postpilotv1.Image, 0, len(p.Images))
	for _, img := range p.Images {
		images = append(images, toProtoImage(img))
	}
	observations := make([]*postpilotv1.Observation, 0, len(p.Observations))
	for _, observation := range p.Observations {
		observations = append(observations, toProtoObservation(observation))
	}
	return &postpilotv1.Post{
		Slug:                    p.Slug,
		Title:                   p.Title,
		Memo:                    p.Memo,
		Status:                  p.Status,
		Images:                  images,
		CreatedAt:               p.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt:               p.UpdatedAt.UTC().Format(timeLayout),
		ActiveJob:               toProtoActiveJob(p.ActiveJob),
		Content:                 toProtoContent(p.Content),
		Observations:            observations,
		PendingExperimentId:     p.PendingExperimentID,
		ContentRevision:         p.ContentRevision,
		MachineBaselineRevision: p.MachineBaselineRevision,
		CanFinalize:             p.Content != nil,
		TargetLength:            protoTargetLength(p.TargetLength),
		FinalizedRevision:       p.FinalizedRevision,
		FinalizedAt:             formatOptionalTime(p.FinalizedAt),
		Voice:                   toProtoVoiceRef(p.Voice),
		MachineBaselineVoiceId:  p.MachineBaselineVoiceID,
		Purpose:                 toProtoPurposeRef(p.Purpose),
		TargetLanguage:          languageToProto(p.TargetLanguage),
		ContentLanguage:         optionalLanguageToProto(p.ContentLanguage),
	}
}

// toProtoPurposeRef leaves the field unset when the post has no purpose: 없음 is the
// default and the client reads absence, not an empty object, as "none".
func toProtoPurposeRef(ref post.PurposeRef) *postpilotv1.PurposeRef {
	if ref.ID == "" {
		return nil
	}
	return &postpilotv1.PurposeRef{Id: ref.ID, Name: ref.Name}
}

func toProtoVoiceRef(ref post.VoiceRef) *postpilotv1.VoiceRef {
	if ref.ID == "" {
		return nil
	}
	return &postpilotv1.VoiceRef{Id: ref.ID, Name: ref.Name, Deleted: ref.Deleted, SourceLanguage: languageToProto(ref.SourceLanguage)}
}

func optionalLanguageFromProto(value *postpilotv1.ContentLanguage) (*post.Language, error) {
	if value == nil {
		return nil, nil
	}
	language, err := languageFromProto(*value)
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func languageFromProto(value postpilotv1.ContentLanguage) (post.Language, error) {
	switch value {
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:
		return post.LanguageKorean, nil
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH:
		return post.LanguageEnglish, nil
	default:
		return "", post.ErrLanguageRequired
	}
}

func languageToProto(value post.Language) postpilotv1.ContentLanguage {
	switch value {
	case post.LanguageKorean:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case post.LanguageEnglish:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
}

func optionalLanguageToProto(value *post.Language) postpilotv1.ContentLanguage {
	if value == nil {
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
	return languageToProto(*value)
}

func optionalTargetLength(value *int32) *int {
	if value == nil {
		return nil
	}
	result := int(*value)
	return &result
}

func protoTargetLength(value *int) *int32 {
	if value == nil {
		return nil
	}
	result := int32(*value)
	return &result
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(timeLayout)
}

func fromProtoContent(content *postpilotv1.PostContent) (post.PostContent, error) {
	if content == nil {
		return post.PostContent{}, errors.New("content is required")
	}
	out := post.PostContent{Title: content.GetTitle(), Summary: content.GetSummary(), Tags: append([]string(nil), content.GetTags()...)}
	for _, block := range content.GetBlocks() {
		if block == nil {
			return post.PostContent{}, errors.New("content contains an empty block")
		}
		out.Blocks = append(out.Blocks, post.Block{Type: fromProtoBlockType(block.GetType()), Content: block.GetContent(), Level: block.GetLevel(), File: block.GetFile(), Alt: block.GetAlt(), Caption: block.GetCaption(), Items: append([]string(nil), block.GetItems()...)})
	}
	return out, nil
}

func fromProtoBlockType(value postpilotv1.BlockType) post.BlockType {
	switch value {
	case postpilotv1.BlockType_TEXT:
		return post.BlockText
	case postpilotv1.BlockType_HEADING:
		return post.BlockHeading
	case postpilotv1.BlockType_IMAGE:
		return post.BlockImage
	case postpilotv1.BlockType_QUOTE:
		return post.BlockQuote
	case postpilotv1.BlockType_LIST:
		return post.BlockList
	default:
		return ""
	}
}

func toProtoContent(content *post.PostContent) *postpilotv1.PostContent {
	if content == nil {
		return nil
	}
	blocks := make([]*postpilotv1.Block, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		blocks = append(blocks, &postpilotv1.Block{
			Type: toProtoBlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return &postpilotv1.PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags, Blocks: blocks}
}

func toProtoBlockType(value post.BlockType) postpilotv1.BlockType {
	switch value {
	case post.BlockText:
		return postpilotv1.BlockType_TEXT
	case post.BlockHeading:
		return postpilotv1.BlockType_HEADING
	case post.BlockImage:
		return postpilotv1.BlockType_IMAGE
	case post.BlockQuote:
		return postpilotv1.BlockType_QUOTE
	case post.BlockList:
		return postpilotv1.BlockType_LIST
	default:
		return postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED
	}
}

func toProtoObservation(value post.Observation) *postpilotv1.Observation {
	return &postpilotv1.Observation{
		File: value.File, Scene: value.Scene, Mood: value.Mood, VisibleText: value.VisibleText,
		Objects: value.Objects, PeoplePresent: value.PeoplePresent,
	}
}

func toProtoActiveJob(found *post.ActiveJob) *postpilotv1.GenerationJob {
	if found == nil {
		return nil
	}
	return &postpilotv1.GenerationJob{
		Id: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage,
		ProgressDone: int32(found.ProgressDone), ProgressTotal: int32(found.ProgressTotal),
		PostSlug: found.PostSlug, ObserveModel: toProtoModelRef(found.ObserveModel),
		WriteModel: toProtoModelRef(found.WriteModel), CreatedAt: found.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt: found.UpdatedAt.UTC().Format(timeLayout), TargetLanguage: languageToProto(found.TargetLanguage),
		Failure: activeJobFailureToProto(found.Failure),
	}
}

func activeJobFailureToProto(found *post.Failure) *postpilotv1.Failure {
	if found == nil || found.Reason == "" {
		return nil
	}
	params := make(map[string]string, len(found.Params))
	for key, value := range found.Params {
		params[key] = value
	}
	return &postpilotv1.Failure{
		Reason: found.Reason, Params: params, TechnicalDetail: found.TechnicalDetail,
	}
}

func toProtoModelRef(value string) *postpilotv1.ModelRef {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return nil
	}
	return &postpilotv1.ModelRef{ProviderId: providerID, ModelId: modelID}
}

// toProtoImage deliberately omits r2_key: the client addresses a photo by id and reads
// it through a presigned URL, so the storage key is never its business.
func toProtoImage(img post.Image) *postpilotv1.Image {
	return &postpilotv1.Image{
		Id:       img.ID,
		Filename: img.Filename,
		Width:    img.Width,
		Height:   img.Height,
		Bytes:    img.Bytes,
		ViewUrl:  img.ViewURL,
	}
}

var _ postpilotv1connect.PostServiceHandler = (*Handler)(nil)
