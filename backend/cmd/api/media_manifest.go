package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/media"
	"github.com/postpilot/backend/internal/platform/config"
)

// The rollout can inspect this binary's real media assets without opening the
// database, starting a listener or loading any application/provider credentials.
func mediaManifest(ctx context.Context, out io.Writer) error {
	cfg, err := config.LoadWorkerRuntime()
	if err != nil {
		return err
	}
	env := clip.Environment{WorkRoot: cfg.WorkRoot, FFmpegPath: cfg.FFmpegPath, FFprobePath: cfg.FFprobePath, ResvgPath: cfg.ResvgPath, OverlayDir: cfg.OverlayDir, FontPaths: cfg.FontPaths, WorkStaleAge: cfg.WorkStaleAge, MediaTimeout: cfg.MediaTimeout, EncodeThreads: cfg.EncodeThreads, DecodeThreads: cfg.DecodeThreads}
	a, err := media.New(clip.DefaultMediaConfig(env), nil)
	if err != nil {
		return err
	}
	r, err := media.NewRenderer(a, clip.DefaultRenderConfig(env))
	if err != nil {
		return err
	}
	profile, err := r.RuntimeProfile(ctx, "cpu")
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(profile)
}
