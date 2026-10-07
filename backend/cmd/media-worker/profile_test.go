package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/worker"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/config"
)

func healthToken() string {
	var token [32]byte
	for i := range token {
		token[i] = byte(i)
	}
	return base64.RawURLEncoding.EncodeToString(token[:])
}

type healthPeer struct {
	postpilotv1connect.UnimplementedClipMediaWorkerServiceHandler
	t      *testing.T
	status *pb.GetMediaRuntimeStatusResponse
}

func (h healthPeer) GetMediaRuntimeStatus(_ context.Context, req *connect.Request[pb.GetMediaRuntimeStatusRequest]) (*connect.Response[pb.GetMediaRuntimeStatusResponse], error) {
	if req.Header().Get("X-Media-Worker-ID") != "worker" || req.Header().Get("Authorization") != "Bearer "+healthToken() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("worker credential rejected"))
	}
	return connect.NewResponse(h.status), nil
}

func TestLiveHealthRetainsOwnerConfigAssetsAndPrivateAuthorization(t *testing.T) {
	for _, mode := range []string{"valid", "wrong-worker", "changed-config", "changed-token", "changed-asset", "private-unready", "stopped"} {
		t.Run(mode, func(t *testing.T) {
			cfg, profile := healthFixture(t, clip.NativeWorkerRole, mode != "private-unready")
			release, err := worker.LockWorkRoot(cfg.WorkRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			binding, err := profileBinding(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err = worker.PublishProfile(cfg.WorkRoot, binding, profile); err != nil {
				t.Fatal(err)
			}
			check := healthHandler(cfg)
			request := httptest.NewRequest(http.MethodGet, "http://worker/health", nil)
			request.Header.Set("X-Media-Worker-ID", cfg.ID)
			request.Header.Set("X-Media-Worker-Config", healthEnvironmentStamp())
			request.Header.Set("Authorization", "Bearer "+cfg.Token)
			switch mode {
			case "wrong-worker":
				request.Header.Set("X-Media-Worker-ID", "other")
			case "changed-config":
				request.Header.Set("X-Media-Worker-Config", "other-config")
			case "changed-token":
				request.Header.Set("Authorization", "Bearer "+strings.Repeat("b", 43))
			case "changed-asset":
				err = os.WriteFile(cfg.FFmpegPath, []byte("changed runtime tool"), 0600)
			case "stopped":
				release()
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, err := check(t.Context(), request)
			if mode == "valid" {
				if err != nil || !strings.Contains(string(raw), `"OwnActive":1`) {
					t.Fatal("live worker status changed", err, string(raw))
				}
			} else if err == nil {
				t.Fatal("unsafe live worker health accepted", mode, string(raw))
			}
		})
	}
}

func healthFixture(t *testing.T, role string, ready bool) (config.WorkerConfig, clip.MediaWorkerProfile) {
	t.Helper()
	root := t.TempDir()
	profile := clip.MediaWorkerProfile{WorkerID: "worker", Operation: clip.MediaRender, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profile: clip.MediaCPUProfile, RuntimeManifest: `{"validated":true}`}
	if role == clip.AnalysisVerificationRole {
		profile.Operation, profile.RendererVersion, profile.AssetVersion, profile.Profile = clip.MediaVerifyAnalysis, clip.AnalysisVerificationRenderer, clip.AnalysisVerificationAssets, clip.AnalysisVerificationProfile
	}
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewClipMediaWorkerServiceHandler(healthPeer{t: t, status: &pb.GetMediaRuntimeStatusResponse{Ready: ready, ContractVersion: int32(profile.ContractVersion), RendererVersion: profile.RendererVersion, AssetVersion: profile.AssetVersion, Profiles: []string{profile.Profile}, Waiting: 4, Active: 2, OwnActive: 1}}))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	for key, value := range map[string]string{"MEDIA_API_URL": server.URL, "MEDIA_WORKER_ID": "worker", "MEDIA_WORKER_TOKEN": healthToken(), "MEDIA_WORKER_ROLE": role, "MEDIA_ACCEL": "cpu", "CLIP_WORK_ROOT": root, "CLIP_OVERLAY_DIR": ""} {
		t.Setenv(key, value)
	}
	// These deliberately cannot be executed or parsed as fonts. A healthy
	// cached path must not construct the renderer or run capability tools.
	for _, key := range []string{"CLIP_FFMPEG_PATH", "CLIP_FFPROBE_PATH", "CLIP_RESVG_PATH", "CLIP_FONT_PATH", "CLIP_FONT_PAPERLOGY_PATH", "CLIP_FONT_JUA_PATH", "CLIP_FONT_NANUM_MYEONGJO_PATH", "CLIP_FONT_NANUM_MYEONGJO_BOLD_PATH"} {
		path := filepath.Join(root, key)
		if err := os.WriteFile(path, []byte("unexecutable fixture artifact"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, path)
	}
	cfg, err := config.LoadWorkerConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.WorkRoot = worker.WorkRoot(cfg.WorkRoot, cfg.ID)
	if err = os.MkdirAll(cfg.WorkRoot, 0700); err != nil {
		t.Fatal(err)
	}
	return cfg, profile
}

func workerCommand(t *testing.T, command string) error {
	t.Helper()
	before := os.Args
	os.Args = []string{"media-worker", command}
	defer func() { os.Args = before }()
	return run()
}

func TestHealthUsesActiveValidatedProfileAndPrivateStatusWithoutRenderer(t *testing.T) {
	for _, role := range []string{clip.NativeWorkerRole, clip.AnalysisVerificationRole} {
		t.Run(role, func(t *testing.T) {
			cfg, profile := healthFixture(t, role, true)
			release, err := worker.LockWorkRoot(cfg.WorkRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if err = workerCommand(t, "health"); err == nil {
				t.Fatal("unvalidated startup reported healthy")
			}
			binding, err := profileBinding(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err = worker.PublishProfile(cfg.WorkRoot, binding, profile); err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{"health", "status"} {
				if err = workerCommand(t, command); err != nil {
					t.Fatal(command, err)
				}
			}
			t.Setenv("CLIP_DECODE_THREADS", "1")
			if err = workerCommand(t, "health"); err == nil {
				t.Fatal("changed worker configuration reused readiness")
			}
		})
	}
}

func TestHealthRejectsUnavailablePrivateAPI(t *testing.T) {
	cfg, profile := healthFixture(t, clip.NativeWorkerRole, false)
	release, err := worker.LockWorkRoot(cfg.WorkRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	binding, err := profileBinding(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.PublishProfile(cfg.WorkRoot, binding, profile); err != nil {
		t.Fatal(err)
	}
	if err = workerCommand(t, "health"); !errors.Is(err, clip.ErrMediaUnavailable) {
		t.Fatal("private readiness was bypassed", err)
	}
}

func TestStatusWithoutActiveWorkerRetainsOfflineValidation(t *testing.T) {
	healthFixture(t, clip.NativeWorkerRole, true)
	err := workerCommand(t, "status")
	if err == nil || !strings.Contains(err.Error(), "worker renderer settings") {
		t.Fatal("offline status skipped capability validation", err)
	}
	if err = workerCommand(t, "health"); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatal("stopped worker became healthy", err)
	}
}

func TestStartupCannotPublishAssetsChangedDuringValidation(t *testing.T) {
	cfg, profile := healthFixture(t, clip.NativeWorkerRole, true)
	release, err := worker.LockWorkRoot(cfg.WorkRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before, err := profileBinding(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(cfg.FFmpegPath, []byte("replacement after capability check"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = publishValidatedProfile(cfg, before, profile); err == nil || !strings.Contains(err.Error(), "changed during") {
		t.Fatal("old validated capabilities were bound to new bytes", err)
	}
	after, err := profileBinding(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = worker.ActiveProfile(cfg.WorkRoot, after); err == nil {
		t.Fatal("failed capability provenance published readiness")
	}
}
