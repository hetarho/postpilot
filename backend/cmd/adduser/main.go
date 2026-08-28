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
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := provision.Run(context.Background(), os.Args[1:]); err != nil {
		slog.Error("adduser failed", "err", err)
		os.Exit(1)
	}
}
