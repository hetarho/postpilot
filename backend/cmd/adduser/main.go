// Command adduser creates a postpilot account from a host shell.
//
// It is a thin wrapper: the deployed image dispatches the same code through
// `api adduser <id>` (see cmd/api), so the image stays one binary. This entry point
// exists for `go run ./cmd/adduser <id>` during development, where there is no image.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/postpilot/backend/internal/auth/provision"
	"github.com/postpilot/backend/internal/platform/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// The environment is read here rather than inside the command (ARCH-6).
	cfg, err := config.Load()
	if err != nil {
		slog.Error("adduser failed", "err", err)
		os.Exit(1)
	}
	settings := provision.Settings{DBPath: cfg.DBPath, SessionTTL: cfg.SessionTTL}
	if err := provision.Run(context.Background(), settings, os.Args[1:]); err != nil {
		slog.Error("adduser failed", "err", err)
		os.Exit(1)
	}
}
