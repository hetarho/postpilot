// media-worker is an execution-only composition root: it does not open SQLite,
// run migrations, load accounts/providers or serve user-facing endpoints.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/media"
	"github.com/postpilot/backend/internal/clip/worker"
	"github.com/postpilot/backend/internal/clip/workerclient"
	"github.com/postpilot/backend/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("media worker stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func environment(c config.WorkerConfig) clip.Environment {
	return clip.Environment{WorkRoot: c.WorkRoot, FFmpegPath: c.FFmpegPath, FFprobePath: c.FFprobePath, ResvgPath: c.ResvgPath, OverlayDir: c.OverlayDir, FontPaths: c.FontPaths, WorkStaleAge: c.WorkStaleAge, MediaTimeout: c.MediaTimeout, EncodeThreads: c.EncodeThreads, DecodeThreads: c.DecodeThreads}
}
func run() error {
	command := "run"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if len(os.Args) > 2 || (command != "run" && command != "health" && command != "status") {
		return errors.New("usage: media-worker [run|health|status]")
	}
	cfg, err := config.LoadWorkerConfig()
	if err != nil {
		return err
	}
	if cfg.Accel == "nvenc" {
		return errors.New("nvenc profile is not approved; use MEDIA_ACCEL=cpu or auto")
	}
	cfg.WorkRoot = worker.WorkRoot(cfg.WorkRoot, cfg.ID)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	mcfg := clip.DefaultMediaConfig(environment(cfg))
	adapter, err := media.New(mcfg, nil)
	if err != nil {
		return errors.New("invalid media workspace or resource settings")
	}
	renderer, err := media.NewRenderer(adapter, clip.DefaultRenderConfig(environment(cfg)))
	if err != nil {
		return errors.New("media fonts or renderer assets are unavailable")
	}
	profile, err := renderer.RuntimeProfile(ctx, cfg.Accel)
	if err != nil {
		return err
	}
	profile.WorkerID = cfg.ID
	client := workerclient.New(cfg.APIURL, cfg.ID, cfg.Token)
	if command != "run" {
		check, cancel := context.WithTimeout(ctx, clip.MediaUnaryTimeout)
		defer cancel()
		status, err := client.Status(check)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(struct {
			Ready                      bool
			Profile                    string
			Waiting, Active, OwnActive int64
		}{true, profile.Profile, status.Waiting, status.Active, status.OwnActive})
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	releaseRoot, err := worker.LockWorkRoot(cfg.WorkRoot)
	if err != nil {
		return err
	}
	defer releaseRoot()
	if err = adapter.CleanupAbandoned(ctx); err != nil {
		return errors.New("media workspace recovery failed")
	}
	// The adapter tracks active workspaces, so the cleanup cannot reap live work.
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if adapter.CleanupStale(ctx, time.Now()) != nil {
					slog.Warn("media workspace cleanup will retry")
				}
			}
		}
	}()
	slog.Info("media worker ready", "profile", profile.Profile, "concurrency", cfg.Concurrency)
	loop := worker.Loop{Control: client, Executor: worker.NewExecutor(adapter, renderer, workerclient.NewTransfers(client), mcfg), Profile: profile, DrainTimeout: cfg.DrainTimeout, PollDelay: workerclient.PollDelay}
	return loop.Run(ctx)
}
