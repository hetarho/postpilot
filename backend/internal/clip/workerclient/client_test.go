package workerclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

type delayedRenew struct {
	postpilotv1connect.UnimplementedClipMediaWorkerServiceHandler
	started chan time.Time
}

func (h *delayedRenew) RenewMediaStage(ctx context.Context, _ *connect.Request[pb.RenewMediaStageRequest]) (*connect.Response[pb.RenewMediaStageResponse], error) {
	h.started <- time.Now()
	select {
	case <-time.After(40 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return connect.NewResponse(&pb.RenewMediaStageResponse{LeaseRemainingMs: 1000}), nil
}
func TestRenewUsesRequestStartInsteadOfAPIEpochOrResponseArrival(t *testing.T) {
	h := &delayedRenew{started: make(chan time.Time, 1)}
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewClipMediaWorkerServiceHandler(h))
	s := httptest.NewServer(mux)
	defer s.Close()
	c := New(s.URL, "worker", "token")
	expires, err := c.Renew(t.Context(), clip.MediaLeaseCredentials{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if expires.After((<-h.started).Add(time.Second)) || !expires.After(time.Now()) {
		t.Fatal("unsafe expiry", expires)
	}
	for range 50 {
		if d := PollDelay(); d < clip.MediaPollInterval || d > clip.MediaPollInterval+clip.MediaPollJitter {
			t.Fatal("unbounded jitter", d)
		}
	}
}
func TestWorkerTransportDoesNotForwardSecretsThroughRedirects(t *testing.T) {
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	_, err := New(redirect.URL, "worker", "secret").Status(t.Context())
	if reached || !errors.Is(err, clip.ErrMediaUnavailable) {
		t.Fatal("redirect leaked authority", err)
	}
}

type runtimeStatusPeer struct {
	postpilotv1connect.UnimplementedClipMediaWorkerServiceHandler
	status *pb.GetMediaRuntimeStatusResponse
}

func (s runtimeStatusPeer) GetMediaRuntimeStatus(context.Context, *connect.Request[pb.GetMediaRuntimeStatusRequest]) (*connect.Response[pb.GetMediaRuntimeStatusResponse], error) {
	return connect.NewResponse(s.status), nil
}

func TestRuntimeHealthPreservesNativeV3AndRequiresExplicitVerificationProfile(t *testing.T) {
	for _, name := range []string{"legacy native", "native", "native wrong profile", "verification", "verification omitted", "verification native profile", "verification native versions", "invalid occupancy"} {
		t.Run(name, func(t *testing.T) {
			profile := clip.MediaWorkerProfile{Operation: clip.MediaRender, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profile: clip.MediaCPUProfile}
			if strings.HasPrefix(name, "verification") {
				profile.Operation, profile.RendererVersion, profile.AssetVersion, profile.Profile = clip.MediaVerifyAnalysis, clip.AnalysisVerificationRenderer, clip.AnalysisVerificationAssets, clip.AnalysisVerificationProfile
			}
			response := &pb.GetMediaRuntimeStatusResponse{Ready: true, ContractVersion: int32(profile.ContractVersion), RendererVersion: profile.RendererVersion, AssetVersion: profile.AssetVersion, Profiles: []string{profile.Profile}, Waiting: 2, Active: 1}
			allowed := name == "legacy native" || name == "native" || name == "verification"
			switch name {
			case "legacy native", "verification omitted":
				response.Profiles = nil
			case "native wrong profile":
				response.Profiles = []string{clip.AnalysisVerificationProfile}
			case "verification native profile":
				response.Profiles = []string{clip.MediaCPUProfile}
			case "verification native versions":
				response.RendererVersion, response.AssetVersion = clip.MediaRendererVersion, clip.MediaAssetVersion
			case "invalid occupancy":
				response.OwnActive = 2
			}
			mux := http.NewServeMux()
			mux.Handle(postpilotv1connect.NewClipMediaWorkerServiceHandler(runtimeStatusPeer{status: response}))
			peer := httptest.NewServer(mux)
			defer peer.Close()
			status, e := New(peer.URL, "worker", "token").StatusForProfile(t.Context(), profile)
			if allowed {
				if e != nil || status != (clip.MediaRuntimeStatus{Waiting: 2, Active: 1}) {
					t.Fatal("compatible health peer rejected", status, e)
				}
			} else if !errors.Is(e, clip.ErrMediaIncompatible) {
				t.Fatal("incompatible health peer accepted", e)
			}
		})
	}
}
