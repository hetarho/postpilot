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
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := provision.Run(context.Background(), os.Args[1:], defaultVoiceBootstrap); err != nil {
		slog.Error("adduser failed", "err", err)
		os.Exit(1)
	}
}

// defaultVoiceBootstrap mirrors cmd/api: the account's `기본 말투` must exist before the
// account can create a post, and rerunning repairs an account left without one.
func defaultVoiceBootstrap(ctx context.Context, handle *db.DB, userID string) error {
	directory := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	_, _, err := directory.EnsureDefaultVoice(ctx, userID, voice.LanguageKorean)
	return err
}
