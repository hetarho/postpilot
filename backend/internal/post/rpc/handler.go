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

	saved, err := h.svc.SaveDraft(ctx, userID, req.Msg.GetSlug(), req.Msg.GetTitle(), req.Msg.GetMemo())
	if err != nil {
		return nil, toConnectError("save draft", err)
	}
	return connect.NewResponse(&postpilotv1.SavePostDraftResponse{Post: toProtoPost(saved)}), nil
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
			Slug:      s.Slug,
			Title:     s.Title,
			Status:    s.Status,
			UpdatedAt: s.UpdatedAt.UTC().Format(timeLayout),
			ActiveJob: toProtoActiveJob(s.ActiveJob),
		})
	}
	return connect.NewResponse(&postpilotv1.ListPostsResponse{Posts: posts}), nil
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
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	return userID, nil
}

// toConnectError maps a domain error to a wire code. Anything unrecognized is Internal
// with the detail kept in the log — an unexpected failure must not leak a SQL string or
// a bucket name to a client.
func toConnectError(op string, err error) error {
	switch {
	case errors.Is(err, post.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("not found"))
	case errors.Is(err, post.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, errors.New("not yours"))
	case errors.Is(err, post.ErrDuplicateFilename):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("a photo with that filename is already attached"))
	case errors.Is(err, post.ErrInvalidImage):
		// The client's own numbers, or an object that cannot be one of our photos.
		// Safe to echo: it says nothing the caller did not send.
		return connect.NewError(connect.CodeInvalidArgument, errors.New(err.Error()))
	case errors.Is(err, post.ErrObjectMissing):
		// FailedPrecondition, not NotFound: the upload record is fine, the object just
		// is not there yet — the client should retry the PUT, not give up.
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("upload not found in storage — retry the upload"))
	default:
		slog.Error(op+" failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New(op+" failed"))
	}
}

func toProtoPost(p post.Post) *postpilotv1.Post {
	images := make([]*postpilotv1.Image, 0, len(p.Images))
	for _, img := range p.Images {
		images = append(images, toProtoImage(img))
	}
	return &postpilotv1.Post{
		Slug:      p.Slug,
		Title:     p.Title,
		Memo:      p.Memo,
		Status:    p.Status,
		Images:    images,
		CreatedAt: p.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt: p.UpdatedAt.UTC().Format(timeLayout),
		ActiveJob: toProtoActiveJob(p.ActiveJob),
	}
}

func toProtoActiveJob(found *post.ActiveJob) *postpilotv1.GenerationJob {
	if found == nil {
		return nil
	}
	return &postpilotv1.GenerationJob{
		Id: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage,
		ProgressDone: int32(found.ProgressDone), ProgressTotal: int32(found.ProgressTotal),
		Error: found.Error, PostSlug: found.PostSlug, ObserveModel: toProtoModelRef(found.ObserveModel),
		WriteModel: toProtoModelRef(found.WriteModel), CreatedAt: found.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt: found.UpdatedAt.UTC().Format(timeLayout),
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
