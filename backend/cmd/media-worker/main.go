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
	"path/filepath"
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
	if len(os.Args) > 2 || (command != "run" && command != "health" && command != "status" && command != "manifest") {
		return errors.New("usage: media-worker [run|health|status|manifest]")
	}
	load := config.LoadWorkerConfig
	if command == "manifest" {
		load = config.LoadWorkerRuntime
	}
	cfg, err := load()
	if err != nil {
		return err
	}
	if cfg.Accel == "nvenc" {
		return errors.New("nvenc profile is not approved; use MEDIA_ACCEL=cpu or auto")
	}
	runningRoot := worker.WorkRoot(cfg.WorkRoot, cfg.ID)
	cfg.WorkRoot = runningRoot
	if command != "run" {
		// A command namespace cannot alias another deployment worker identity.
		// Startup recovery only collects direct media workspaces, not .checks.
		cfg.WorkRoot = filepath.Join(runningRoot, ".checks", command)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	mcfg := clip.DefaultMediaConfig(environment(cfg))
	adapter, err := media.New(mcfg, nil)
	if err != nil {
		return fmt.Errorf("worker media settings: %w", err)
	}
	renderer, err := media.NewRenderer(adapter, clip.DefaultRenderConfig(environment(cfg)))
	if err != nil {
		return fmt.Errorf("worker renderer settings: %w", err)
	}
	if command == "run" {
		releaseRoot, err := worker.LockWorkRoot(cfg.WorkRoot)
		if err != nil {
			return err
		}
		defer releaseRoot()
		if err = adapter.CleanupAbandoned(ctx); err != nil {
			return errors.New("media workspace recovery failed")
		}
	}
	profile, err := renderer.RuntimeProfile(ctx, cfg.Accel)
	if err != nil {
		return err
	}
	if command == "manifest" {
		raw, err := json.Marshal(profile)
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	profile.WorkerID = cfg.ID
	if command == "health" {
		if err := worker.CheckWorkRootActive(runningRoot); err != nil {
			return err
		}
	}
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
