package hermes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/postpilot/agent/internal/browser"
)

const promptPrefix = "Publish the prepared Postpilot Naver job referenced by this local handle: "

type Result struct {
	Status       string `json:"status"`
	PublishedURL string `json:"published_url"`
	FailureKind  string `json:"failure_kind"`
	Detail       string `json:"detail"`
}

type Runner struct {
	Binary         string
	Profile        string
	BrowserBinary  string
	BrowserProfile string
	MaxTurns       int
}

func (r Runner) Run(ctx context.Context, jobHandle, jobDir, callbackURL, callbackToken string) (Result, error) {
	if jobHandle == "" || jobDir == "" || callbackURL == "" || callbackToken == "" {
		return Result{}, errors.New("incomplete Hermes run context")
	}
	session, err := browser.Start(r.BrowserBinary, r.BrowserProfile, "")
	if err != nil {
		return Result{}, fmt.Errorf("start dedicated browser: %w", err)
	}
	defer session.Close()
	binary := r.Binary
	if binary == "" {
		binary = "hermes"
	}
	turns := r.MaxTurns
	if turns <= 0 {
		turns = 60
	}
	command := exec.CommandContext(ctx, binary, "-p", r.Profile, "chat", "--oneshot", "--quiet", "--ignore-rules", "--skills", "postpilot-publisher:postpilot-naver-publisher", "--toolsets", "postpilot-publisher,browser", "--max-turns", strconv.Itoa(turns), "--query", promptPrefix+jobHandle)
	command.Dir = jobDir
	command.Env = append(os.Environ(),
		"POSTPILOT_JOB_HANDLE="+jobHandle,
		"POSTPILOT_JOB_DIR="+jobDir,
		"POSTPILOT_CALLBACK_URL="+callbackURL,
		"POSTPILOT_CALLBACK_TOKEN="+callbackToken,
	)
	command.Env = append(command.Env, "BROWSER_CDP_URL="+session.CDPURL)
	// Model prose is never terminal publication evidence. postpilot_finish records
	// the validated result through the authenticated loopback callback; stdout and
	// stderr are deliberately discarded by leaving them unset on the command.
	if err := command.Run(); err != nil {
		return Result{}, fmt.Errorf("Hermes publisher exited: %w", err)
	}
	return Result{}, nil
}
