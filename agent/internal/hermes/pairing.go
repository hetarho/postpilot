package hermes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const pairingPrompt = "Use the Postpilot Naver pairing skill to open the signed-in account's own Naver Blog writing editor. Do not type, upload, or publish anything."

// PrepareNaverEditor lets Hermes drive the visible, account-specific Naver UI.
// Its prose is discarded; the caller independently verifies the resulting live
// CDP page before accepting any identity or category metadata.
func PrepareNaverEditor(ctx context.Context, binary, profile, cdpURL string) error {
	if profile == "" || cdpURL == "" {
		return errors.New("incomplete Hermes pairing context")
	}
	if binary == "" {
		binary = "hermes"
	}
	command := exec.CommandContext(
		ctx, binary, "-p", profile, "chat", "--oneshot", "--quiet", "--ignore-rules",
		"--skills", "postpilot-publisher:postpilot-naver-pairing",
		"--toolsets", "postpilot-publisher,browser", "--max-turns", "30",
		"--query", pairingPrompt,
	)
	command.Env = append(os.Environ(), "POSTPILOT_MODE=pairing", "BROWSER_CDP_URL="+cdpURL)
	// Pairing completion is proven from the bound browser page, never model text.
	// Discard both streams so page/account prose cannot become product authority.
	if err := command.Run(); err != nil {
		return fmt.Errorf("Hermes Naver pairing exited: %w", err)
	}
	return nil
}
