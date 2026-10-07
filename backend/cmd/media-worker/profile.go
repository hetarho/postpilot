package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/worker"
	"github.com/postpilot/backend/internal/clip/workerclient"
	"github.com/postpilot/backend/internal/platform/config"
)

func profileBinding(cfg config.WorkerConfig) (worker.ProfileBinding, error) {
	// Credentials are supplied afresh to the private API, never cached or hashed.
	cfg.Token = ""
	raw, err := json.Marshal(cfg)
	if err != nil {
		return worker.ProfileBinding{}, err
	}
	configuration := sha256.Sum256(raw)
	executable, err := os.Executable()
	if err != nil {
		return worker.ProfileBinding{}, err
	}
	paths := []string{executable, cfg.FFmpegPath, cfg.FFprobePath}
	if cfg.Role == clip.NativeWorkerRole {
		paths = append(paths, cfg.ResvgPath)
		for _, path := range cfg.FontPaths {
			paths = append(paths, path)
		}
		if cfg.OverlayDir != "" {
			paths = append(paths, cfg.OverlayDir)
		}
	}
	identity, err := worker.RuntimeIdentity(paths)
	if err != nil {
		return worker.ProfileBinding{}, fmt.Errorf("worker runtime identity: %w", err)
	}
	return worker.ProfileBinding{WorkerID: cfg.ID, Configuration: hex.EncodeToString(configuration[:]), RuntimeIdentity: identity}, nil
}

func publishValidatedProfile(cfg config.WorkerConfig, before worker.ProfileBinding, profile clip.MediaWorkerProfile) error {
	after, err := profileBinding(cfg)
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("worker runtime changed during capability validation")
	}
	return worker.PublishProfile(cfg.WorkRoot, after, profile)
}

func reportStatus(ctx context.Context, cfg config.WorkerConfig, profile clip.MediaWorkerProfile) error {
	raw, err := statusJSON(ctx, cfg, profile)
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

func statusJSON(ctx context.Context, cfg config.WorkerConfig, profile clip.MediaWorkerProfile) ([]byte, error) {
	check, cancel := context.WithTimeout(ctx, clip.MediaUnaryTimeout)
	defer cancel()
	status, err := workerclient.New(cfg.APIURL, cfg.ID, cfg.Token).StatusForProfile(check, profile)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(struct {
		Ready                      bool
		Profile                    string
		Waiting, Active, OwnActive int64
	}{true, profile.Profile, status.Waiting, status.Active, status.OwnActive})
	return raw, err
}
