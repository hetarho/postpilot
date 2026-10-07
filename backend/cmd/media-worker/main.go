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
	"github.com/postpilot/backend/internal/clip/diagnostic"
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
	if command == "gpu-probe" || command == "benchmark" {
		cfg, err := config.LoadWorkerRuntime()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return diagnostic.Run(ctx, command, os.Args[2:], environment(cfg))
	}
	if len(os.Args) > 2 || (command != "run" && command != "health" && command != "status" && command != "manifest") {
		return errors.New("usage: media-worker [run|health|status|manifest] | gpu-probe --output NEW_DIR | benchmark --manifest FILE --output NEW_DIR [--cpu-only]")
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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if command == "health" || command == "status" && worker.CheckWorkRootActive(runningRoot) == nil {
		if err := worker.CheckWorkRootActive(runningRoot); err != nil {
			return err
		}
		binding, err := profileBinding(cfg)
		if err != nil {
			return err
		}
		profile, err := worker.ActiveProfile(runningRoot, binding)
		if err == nil {
			return reportStatus(ctx, cfg, profile)
		}
		if command == "health" {
			return err
		}
		// Status also supports an offline candidate. Preserve its full runtime
		// validation when no current compatible owner snapshot can be reused.
	}
	if command != "run" {
		// A command namespace cannot alias another deployment worker identity.
		// Startup recovery only collects direct media workspaces, not .checks.
		cfg.WorkRoot = filepath.Join(runningRoot, ".checks", command)
	}
	mcfg := clip.DefaultMediaConfig(environment(cfg))
	if cfg.Role == clip.AnalysisVerificationRole {
		mcfg, err = media.AnalysisVerificationConfig(mcfg)
		if err != nil {
			return err
		}
	}
	adapter, err := media.New(mcfg, nil)
	if err != nil {
		return fmt.Errorf("worker media settings: %w", err)
	}
	var validatedBinding worker.ProfileBinding
	if command == "run" {
		releaseRoot, err := worker.LockWorkRoot(cfg.WorkRoot)
		if err != nil {
			return err
		}
		defer releaseRoot()
		if err = adapter.CleanupAbandoned(ctx); err != nil {
			return errors.New("media workspace recovery failed")
		}
		validatedBinding, err = profileBinding(cfg)
		if err != nil {
			return err
		}
	}
	var renderer *media.Rendering
	if cfg.Role == clip.NativeWorkerRole {
		renderer, err = media.NewRenderer(adapter, clip.DefaultRenderConfig(environment(cfg)))
		if err != nil {
			return fmt.Errorf("worker renderer settings: %w", err)
		}
	}
	var profile clip.MediaWorkerProfile
	if cfg.Role == clip.AnalysisVerificationRole {
		profile, err = adapter.AnalysisVerificationProfile(ctx)
	} else {
		profile, err = renderer.RuntimeProfile(ctx, cfg.Accel)
	}
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
	client := workerclient.New(cfg.APIURL, cfg.ID, cfg.Token)
	if command != "run" {
		return reportStatus(ctx, cfg, profile)
	}
	if err = publishValidatedProfile(cfg, validatedBinding, profile); err != nil {
		return err
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
	var execution worker.Execution = worker.NewExecutor(adapter, renderer, workerclient.NewTransfers(client), mcfg)
	if cfg.Role == clip.AnalysisVerificationRole {
		execution = worker.NewAnalysisVerifier(adapter, workerclient.NewTransfers(client), mcfg)
	}
	loop := worker.Loop{Control: client, Executor: execution, Profile: profile, DrainTimeout: cfg.DrainTimeout, PollDelay: workerclient.PollDelay}
	return loop.Run(ctx)
}
