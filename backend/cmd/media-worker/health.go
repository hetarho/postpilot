package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/postpilot/backend/internal/clip/worker"
	"github.com/postpilot/backend/internal/platform/config"
)

var healthEnvironmentKeys = []string{
	"MEDIA_API_URL", "MEDIA_WORKER_ID", "MEDIA_WORKER_ROLE", "MEDIA_ACCEL",
	"MEDIA_WORKER_CONCURRENCY", "MEDIA_DRAIN_TIMEOUT", "CLIP_WORK_ROOT",
	"CLIP_FFMPEG_PATH", "CLIP_FFPROBE_PATH", "CLIP_RESVG_PATH", "CLIP_OVERLAY_DIR",
	"CLIP_FONT_PATH", "CLIP_FONT_PAPERLOGY_PATH", "CLIP_FONT_JUA_PATH",
	"CLIP_FONT_NANUM_MYEONGJO_PATH", "CLIP_FONT_NANUM_MYEONGJO_BOLD_PATH",
	"CLIP_WORK_STALE_AGE", "CLIP_MEDIA_TIMEOUT", "CLIP_ENCODE_THREADS", "CLIP_DECODE_THREADS",
}

func healthEnvironmentStamp() string {
	values := make(map[string]string, len(healthEnvironmentKeys))
	for _, key := range healthEnvironmentKeys {
		values[key] = os.Getenv(key)
	}
	raw, _ := json.Marshal(values)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func healthHandler(cfg config.WorkerConfig) func(context.Context, *http.Request) ([]byte, error) {
	stamp := healthEnvironmentStamp()
	return func(ctx context.Context, request *http.Request) ([]byte, error) {
		if request.Header.Get("X-Media-Worker-ID") != cfg.ID || request.Header.Get("X-Media-Worker-Config") != stamp {
			return nil, errors.New("worker health configuration changed")
		}
		token, found := strings.CutPrefix(request.Header.Get("Authorization"), "Bearer ")
		if !found || len(token) < 1 || len(token) > 128 {
			return nil, errors.New("worker health credentials unavailable")
		}
		binding, err := profileBinding(cfg)
		if err != nil {
			return nil, err
		}
		profile, err := worker.ActiveProfile(cfg.WorkRoot, binding)
		if err != nil {
			return nil, err
		}
		current := cfg
		current.Token = token
		return statusJSON(ctx, current, profile)
	}
}
