package media

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

type Command struct {
	Binary, Dir string
	Args        []string
	// FFmpeg prints a filter's own report — loudnorm's measurement JSON among
	// them — on stderr and nowhere else, so a caller that needs one asks for
	// stderr instead of stdout. The bound and the failure text are unchanged.
	CaptureStderr bool
}
type Runner interface {
	Run(context.Context, Command) ([]byte, error)
}
type ExecRunner struct {
	StdoutLimit, StderrLimit int
	WaitDelay                time.Duration
}
type boundedBuffer struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := max(0, b.limit-len(b.data))
	if n > remaining {
		b.exceeded = true
	}
	b.data = append(b.data, p[:min(n, remaining)]...)
	return n, nil
}
func (r ExecRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	if r.StdoutLimit <= 0 || r.StderrLimit <= 0 || r.WaitDelay <= 0 {
		return nil, errors.New("invalid media runner limits")
	}
	cmd := exec.CommandContext(ctx, command.Binary, command.Args...)
	cmd.Dir = command.Dir
	cmd.WaitDelay = r.WaitDelay
	out := &boundedBuffer{limit: r.StdoutLimit}
	stderr := &boundedBuffer{limit: r.StderrLimit}
	cmd.Stdout, cmd.Stderr = out, stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w; stderr=%q", filepath.Base(command.Binary), err, stderr.data)
	}
	if out.exceeded {
		return nil, errors.New("media command output exceeded its bound")
	}
	if command.CaptureStderr {
		// A filter report arrives at the END of the log, so a truncated stderr
		// is a missing report, not a shorter one.
		if stderr.exceeded {
			return nil, errors.New("media command log exceeded its bound")
		}
		return stderr.data, nil
	}
	return out.data, nil
}
