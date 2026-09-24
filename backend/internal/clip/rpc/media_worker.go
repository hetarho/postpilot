package rpc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

const MediaWorkerIdentityHeader = "X-Media-Worker-ID"

type mediaPrincipalKey struct{}

type MediaWorkerService interface {
	Claim(context.Context, clip.MediaWorkerProfile) (*clip.MediaWork, error)
	Renew(context.Context, clip.MediaLeaseCredentials, int) (time.Duration, error)
	Complete(context.Context, clip.MediaLeaseCredentials, string) error
	Fail(context.Context, clip.MediaLeaseCredentials, clip.MediaFailure) error
	Status(context.Context, string) (clip.MediaRuntimeStatus, error)
}

type MediaWorkerHandler struct{ service MediaWorkerService }

// NewMediaWorkerServer has its own mux, no session auth, CORS or public routes.
// Authentication runs outside Connect, before reading/decompressing the body.
func NewMediaWorkerServer(addr string, credentials map[string]string, service MediaWorkerService) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewClipMediaWorkerServiceHandler(&MediaWorkerHandler{service: service}, connect.WithReadMaxBytes(clip.MediaRequestMaxBytes)))
	hashes := make(map[string][sha256.Size]byte, len(credentials))
	for id, token := range credentials {
		hashes[id] = sha256.Sum256([]byte(token))
	}
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids, tokens := r.Header.Values(MediaWorkerIdentityHeader), r.Header.Values("Authorization")
		if len(ids) != 1 || len(tokens) != 1 || !strings.HasPrefix(tokens[0], "Bearer ") {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		want, found := hashes[ids[0]]
		got := sha256.Sum256([]byte(strings.TrimPrefix(tokens[0], "Bearer ")))
		if subtle.ConstantTimeCompare(got[:], want[:]) != 1 || !found {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), mediaPrincipalKey{}, ids[0]), clip.MediaUnaryTimeout)
		defer cancel()
		r.Body = http.MaxBytesReader(w, r.Body, clip.MediaRequestMaxBytes)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
	return &http.Server{Addr: addr, Handler: root, ReadHeaderTimeout: clip.MediaUnaryTimeout, ReadTimeout: clip.MediaUnaryTimeout, WriteTimeout: clip.MediaUnaryTimeout, IdleTimeout: clip.MediaUnaryTimeout}
}

func workerIdentity(ctx context.Context) (string, error) {
	id, ok := ctx.Value(mediaPrincipalKey{}).(string)
	if !ok || id == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("worker authentication required"))
	}
	return id, nil
}
func workerLease(ctx context.Context, in *pb.MediaLeaseCredentials) (clip.MediaLeaseCredentials, error) {
	id, err := workerIdentity(ctx)
	if err != nil {
		return clip.MediaLeaseCredentials{}, err
	}
	if in == nil || !clip.ValidMediaLabel(in.StageId) || !clip.ValidMediaLabel(in.AttemptId) || !clip.ValidMediaLabel(in.Token) {
		return clip.MediaLeaseCredentials{}, mediaWorkerError(clip.ErrInvalid)
	}
	return clip.MediaLeaseCredentials{StageID: in.StageId, AttemptID: in.AttemptId, WorkerID: id, Token: in.Token}, nil
}

// Only closed messages cross this edge. SQL, storage, tokens, signed links and
// provider/subprocess error prose are neither logged nor returned.
func mediaWorkerError(err error) error {
	if err == nil {
		return nil
	}
	code, message := connect.CodeInternal, "media operation failed"
	switch {
	case errors.Is(err, clip.ErrInvalid):
		code, message = connect.CodeInvalidArgument, "invalid media request"
	case errors.Is(err, clip.ErrMediaIncompatible):
		code, message = connect.CodeFailedPrecondition, "incompatible media runtime"
	case errors.Is(err, clip.ErrMediaLeaseLost):
		code, message = connect.CodeAborted, "media lease lost"
	case errors.Is(err, clip.ErrMediaCancelled):
		code, message = connect.CodeCanceled, "media stage cancelled"
	case errors.Is(err, clip.ErrMediaConflict):
		code, message = connect.CodeAlreadyExists, "media receipt conflicts"
	case errors.Is(err, clip.ErrMediaUnsupported):
		code, message = connect.CodeUnimplemented, "media operation unavailable"
	case errors.Is(err, context.DeadlineExceeded):
		code, message = connect.CodeDeadlineExceeded, "media request expired"
	case errors.Is(err, context.Canceled):
		code, message = connect.CodeCanceled, "media request cancelled"
	}
	return connect.NewError(code, errors.New(message))
}

func (h *MediaWorkerHandler) ClaimMediaStage(ctx context.Context, r *connect.Request[pb.ClaimMediaStageRequest]) (*connect.Response[pb.ClaimMediaStageResponse], error) {
	id, err := workerIdentity(ctx)
	if err != nil {
		return nil, err
	}
	p := r.Msg.Profile
	if p == nil {
		return nil, mediaWorkerError(clip.ErrInvalid)
	}
	work, err := h.service.Claim(ctx, clip.MediaWorkerProfile{WorkerID: id, Operation: clip.MediaOperation(p.Operation), ContractVersion: int(p.ContractVersion), RendererVersion: p.RendererVersion, AssetVersion: p.AssetVersion, Profile: p.Profile, RuntimeManifest: p.RuntimeManifest})
	if err != nil {
		return nil, mediaWorkerError(err)
	}
	out := &pb.ClaimMediaStageResponse{RetryAfterMs: clip.MediaPollInterval.Milliseconds()}
	if work != nil {
		out.Work = &pb.MediaWork{Lease: &pb.MediaLeaseCredentials{StageId: work.Credentials.StageID, AttemptId: work.Credentials.AttemptID, Token: work.Credentials.Token}, Operation: string(work.Operation), ContractVersion: int32(work.ContractVersion), RendererVersion: work.RendererVersion, AssetVersion: work.AssetVersion, InputDigest: work.InputDigest, Payload: work.Payload, LeaseRemainingMs: max(work.LeaseRemaining.Milliseconds(), 0), HeartbeatAfterMs: work.HeartbeatAfter.Milliseconds(), StageRemainingMs: max(work.StageRemaining.Milliseconds(), 0)}
	}
	return connect.NewResponse(out), nil
}
func (h *MediaWorkerHandler) RenewMediaStage(ctx context.Context, r *connect.Request[pb.RenewMediaStageRequest]) (*connect.Response[pb.RenewMediaStageResponse], error) {
	lease, err := workerLease(ctx, r.Msg.Lease)
	if err != nil {
		return nil, err
	}
	remaining, err := h.service.Renew(ctx, lease, int(r.Msg.Progress))
	if errors.Is(err, clip.ErrMediaCancelled) {
		return connect.NewResponse(&pb.RenewMediaStageResponse{Cancelled: true}), nil
	}
	if err != nil {
		return nil, mediaWorkerError(err)
	}
	return connect.NewResponse(&pb.RenewMediaStageResponse{LeaseRemainingMs: remaining.Milliseconds()}), nil
}
func (h *MediaWorkerHandler) CompleteMediaStage(ctx context.Context, r *connect.Request[pb.CompleteMediaStageRequest]) (*connect.Response[pb.CompleteMediaStageResponse], error) {
	lease, err := workerLease(ctx, r.Msg.Lease)
	if err != nil {
		return nil, err
	}
	if err = h.service.Complete(ctx, lease, r.Msg.Result); err != nil {
		return nil, mediaWorkerError(err)
	}
	return connect.NewResponse(&pb.CompleteMediaStageResponse{}), nil
}
func (h *MediaWorkerHandler) FailMediaStage(ctx context.Context, r *connect.Request[pb.FailMediaStageRequest]) (*connect.Response[pb.FailMediaStageResponse], error) {
	lease, err := workerLease(ctx, r.Msg.Lease)
	if err != nil {
		return nil, err
	}
	failure := clip.MediaFailure(r.Msg.Failure)
	if !failure.Valid() {
		return nil, mediaWorkerError(clip.ErrInvalid)
	}
	if err = h.service.Fail(ctx, lease, failure); err != nil {
		return nil, mediaWorkerError(err)
	}
	return connect.NewResponse(&pb.FailMediaStageResponse{}), nil
}
func (h *MediaWorkerHandler) GetMediaRuntimeStatus(ctx context.Context, _ *connect.Request[pb.GetMediaRuntimeStatusRequest]) (*connect.Response[pb.GetMediaRuntimeStatusResponse], error) {
	id, err := workerIdentity(ctx)
	if err != nil {
		return nil, err
	}
	status, err := h.service.Status(ctx, id)
	if err != nil {
		return nil, mediaWorkerError(err)
	}
	return connect.NewResponse(&pb.GetMediaRuntimeStatusResponse{ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profiles: []string{clip.MediaCPUProfile}, Waiting: status.Waiting, Active: status.Active, OwnActive: status.OwnActive, Ready: true}), nil
}
func (h *MediaWorkerHandler) GetMediaArtifactAccess(context.Context, *connect.Request[pb.GetMediaArtifactAccessRequest]) (*connect.Response[pb.GetMediaArtifactAccessResponse], error) {
	return nil, mediaWorkerError(clip.ErrMediaUnsupported)
}
func (h *MediaWorkerHandler) ReserveMediaOutputs(context.Context, *connect.Request[pb.ReserveMediaOutputsRequest]) (*connect.Response[pb.ReserveMediaOutputsResponse], error) {
	return nil, mediaWorkerError(clip.ErrMediaUnsupported)
}
