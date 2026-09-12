package media

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// Only code-owned labels cross the worker's logging boundary. Keep the original
// cause for errors.Is/As and local diagnosis, never expose stderr or arguments.
type commandFailure struct {
	error
	operation, class string
	elapsed          time.Duration
}

func (e *commandFailure) Unwrap() error             { return e.error }
func (e *commandFailure) MediaOperation() string    { return e.operation }
func (e *commandFailure) MediaFailureClass() string { return e.class }
func (e *commandFailure) MediaElapsedMS() int64     { return e.elapsed.Milliseconds() }

func commandDiagnostic(err error, c Command, elapsed time.Duration) error {
	if err == nil {
		return nil
	}
	class := "command_failed"
	var exited *exec.ExitError
	switch {
	case errors.Is(err, clip.ErrWorkspaceLimit):
		class = "workspace_limit"
	case errors.Is(err, clip.ErrAnalysisTooLarge):
		class = "output_limit"
	case errors.Is(err, context.DeadlineExceeded):
		class = "timeout"
	case errors.Is(err, context.Canceled):
		class = "canceled"
	case errors.As(err, &exited):
		class = "process_exit"
		if exited.ExitCode() < 0 {
			class = "process_signal"
		}
	}
	return &commandFailure{error: err, operation: commandOperation(c), class: class, elapsed: elapsed}
}

func commandOperation(c Command) string {
	switch filepath.Base(c.Binary) {
	case "resvg":
		return "typeset"
	case "ffprobe":
		return "probe"
	case "ffmpeg":
		if c.CaptureStderr {
			return "loudness"
		}
		if len(c.Args) > 0 {
			output := filepath.Base(c.Args[len(c.Args)-1])
			switch {
			case output == "clip-result.mp4":
				return "encode_final"
			case output == "compose-audio.wav":
				return "compose_audio"
			case strings.HasPrefix(output, "audio-correction-"):
				return "correct_audio"
			case strings.HasPrefix(output, "render-cut-"):
				return "render_cut"
			case strings.HasPrefix(output, "compose-"):
				return "compose_video"
			case strings.HasPrefix(output, "sample-"):
				return "sample"
			case strings.HasPrefix(output, "proxy-"):
				return "prepare"
			}
		}
		return "decode"
	}
	return "unknown"
}
