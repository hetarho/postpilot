// Package workerclient keeps Connect/protobuf at the worker's transport edge.
package workerclient

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

type Client struct {
	rpc       postpilotv1connect.ClipMediaWorkerServiceClient
	id, token string
}

func New(url, id, token string) *Client {
	httpClient := &http.Client{Timeout: clip.MediaUnaryTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{rpc: postpilotv1connect.NewClipMediaWorkerServiceClient(httpClient, url, connect.WithReadMaxBytes(clip.MediaRequestMaxBytes), connect.WithSendMaxBytes(clip.MediaRequestMaxBytes)), id: id, token: token}
}

func request[T any](c *Client, message *T) *connect.Request[T] {
	r := connect.NewRequest(message)
	r.Header().Set("X-Media-Worker-ID", c.id)
	r.Header().Set("Authorization", "Bearer "+c.token)
	return r
}
func leaseMessage(a clip.MediaLeaseCredentials) *pb.MediaLeaseCredentials {
	return &pb.MediaLeaseCredentials{StageId: a.StageID, AttemptId: a.AttemptID, Token: a.Token}
}

func failure(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated, connect.CodePermissionDenied:
		return clip.ErrMediaUnauthenticated
	case connect.CodeFailedPrecondition:
		return clip.ErrMediaIncompatible
	case connect.CodeInvalidArgument, connect.CodeResourceExhausted:
		return clip.ErrInvalid
	case connect.CodeAborted:
		return clip.ErrMediaLeaseLost
	case connect.CodeCanceled:
		return clip.ErrMediaCancelled
	case connect.CodeAlreadyExists:
		return clip.ErrMediaConflict
	case connect.CodeUnimplemented:
		return clip.ErrMediaUnsupported
	case connect.CodeDeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return clip.ErrMediaUnavailable
	}
}

// PollDelay jitters the fixed idle cadence; work never holds a unary call open.
func PollDelay() time.Duration {
	return clip.MediaPollInterval + time.Duration(rand.Int64N(int64(clip.MediaPollJitter)+1))
}

func (c *Client) Claim(ctx context.Context, p clip.MediaWorkerProfile) (*clip.MediaWork, error) {
	start := time.Now()
	r, err := c.rpc.ClaimMediaStage(ctx, request(c, &pb.ClaimMediaStageRequest{Profile: &pb.MediaWorkerProfile{Operation: string(p.Operation), ContractVersion: int32(p.ContractVersion), RendererVersion: p.RendererVersion, AssetVersion: p.AssetVersion, Profile: p.Profile, RuntimeManifest: p.RuntimeManifest}}))
	if err != nil {
		return nil, failure(err)
	}
	w := r.Msg.Work
	if w == nil {
		return nil, nil
	}
	if w.Lease == nil || w.ContractVersion != int32(p.ContractVersion) || w.Operation != string(p.Operation) || w.RendererVersion != p.RendererVersion || w.AssetVersion != p.AssetVersion || w.InputDigest != clip.MediaPayloadDigest(w.Payload) || w.LeaseRemainingMs <= 0 || w.LeaseRemainingMs > clip.MediaStageTimeoutMax.Milliseconds() || w.HeartbeatAfterMs <= 0 || w.StageRemainingMs <= 0 || w.StageRemainingMs > clip.MediaStageTimeoutMax.Milliseconds() {
		return nil, clip.ErrMediaIncompatible
	}
	expires := start.Add(time.Duration(w.LeaseRemainingMs) * time.Millisecond)
	if !expires.After(time.Now()) {
		return nil, clip.ErrMediaLeaseLost
	}
	return &clip.MediaWork{Credentials: clip.MediaLeaseCredentials{StageID: w.Lease.StageId, AttemptID: w.Lease.AttemptId, WorkerID: c.id, Token: w.Lease.Token}, Operation: clip.MediaOperation(w.Operation), ContractVersion: int(w.ContractVersion), RendererVersion: w.RendererVersion, AssetVersion: w.AssetVersion, InputDigest: w.InputDigest, Payload: w.Payload, LeaseRemaining: time.Until(expires), LeaseExpiresAt: expires, HeartbeatAfter: time.Duration(w.HeartbeatAfterMs) * time.Millisecond, StageRemaining: max(time.Duration(w.StageRemainingMs)*time.Millisecond-time.Since(start), 0)}, nil
}

func (c *Client) Renew(ctx context.Context, a clip.MediaLeaseCredentials, progress int) (time.Time, error) {
	start := time.Now()
	r, err := c.rpc.RenewMediaStage(ctx, request(c, &pb.RenewMediaStageRequest{Lease: leaseMessage(a), Progress: int32(progress)}))
	if err != nil {
		return time.Time{}, failure(err)
	}
	if r.Msg.Cancelled {
		return time.Time{}, clip.ErrMediaCancelled
	}
	if r.Msg.LeaseRemainingMs <= 0 || r.Msg.LeaseRemainingMs > clip.MediaStageTimeoutMax.Milliseconds() {
		return time.Time{}, clip.ErrMediaLeaseLost
	}
	expires := start.Add(time.Duration(r.Msg.LeaseRemainingMs) * time.Millisecond)
	if !expires.After(time.Now()) {
		return time.Time{}, clip.ErrMediaLeaseLost
	}
	return expires, nil
}
func (c *Client) Complete(ctx context.Context, a clip.MediaLeaseCredentials, result string) error {
	_, err := c.rpc.CompleteMediaStage(ctx, request(c, &pb.CompleteMediaStageRequest{Lease: leaseMessage(a), Result: result}))
	return failure(err)
}
func (c *Client) Fail(ctx context.Context, a clip.MediaLeaseCredentials, code clip.MediaFailure) error {
	_, err := c.rpc.FailMediaStage(ctx, request(c, &pb.FailMediaStageRequest{Lease: leaseMessage(a), Failure: string(code)}))
	return failure(err)
}
func (c *Client) Status(ctx context.Context) (clip.MediaRuntimeStatus, error) {
	r, err := c.rpc.GetMediaRuntimeStatus(ctx, request(c, &pb.GetMediaRuntimeStatusRequest{}))
	if err != nil {
		return clip.MediaRuntimeStatus{}, failure(err)
	}
	s := r.Msg
	if s.ContractVersion != clip.MediaContractVersion || s.RendererVersion != clip.MediaRendererVersion || s.AssetVersion != clip.MediaAssetVersion {
		return clip.MediaRuntimeStatus{}, clip.ErrMediaIncompatible
	}
	if !s.Ready {
		return clip.MediaRuntimeStatus{}, clip.ErrMediaUnavailable
	}
	return clip.MediaRuntimeStatus{Waiting: s.Waiting, Active: s.Active, OwnActive: s.OwnActive}, nil
}
