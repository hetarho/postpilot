package media

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"golang.org/x/sys/unix"
)

// Leave enough space to close a muxer and remove its incomplete output. Checks
// precede writes and continue while a child is running, not just after encoding.
const diskHeadroom int64 = 8 << 20

// A refusal names WHICH ceiling stopped the job and the numbers that decided
// it. Three different limits raise the one error, and without these a failed
// render says only that something was full. Sizes are code-owned integers, so
// they cross the worker's logging boundary where a name or a path may not.
type workspaceLimit struct {
	error
	check                   string
	total, additional, free int64
}

func (e *workspaceLimit) Unwrap() error              { return e.error }
func (e *workspaceLimit) MediaLimitCheck() string    { return e.check }
func (e *workspaceLimit) MediaWorkspaceBytes() int64 { return e.total }
func (e *workspaceLimit) MediaRequestedBytes() int64 { return e.additional }

// MediaFreeBytes is negative when the filesystem was never reached, which is
// itself the diagnosis: the refusal came from the budget, not from the disk.
func (e *workspaceLimit) MediaFreeBytes() int64 { return e.free }

func limited(check string, total, additional, free int64) error {
	return &workspaceLimit{error: clip.ErrWorkspaceLimit, check: check, total: total, additional: additional, free: free}
}

func (a *Adapter) capacity(ws clip.MediaWorkspace, additional int64) error {
	if err := a.validWorkspace(ws, true); err != nil {
		return err
	}
	if additional < 0 || additional > a.cfg.WorkspaceMaxBytes {
		return limited("request", 0, additional, -1)
	}
	entries, err := os.ReadDir(ws.Path)
	if err != nil {
		return limited("listing", 0, additional, -1)
	}
	var total, prepared int64
	for _, entry := range entries {
		info, err := entry.Info()
		if os.IsNotExist(err) { // a successfully consumed proxy may just be released
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > a.cfg.WorkspaceMaxBytes-total {
			return limited("budget", total, additional, -1)
		}
		total += info.Size()
		if strings.HasPrefix(entry.Name(), "proxy-") {
			if info.Size() > a.cfg.PreparedMaxBytes-prepared {
				return limited("prepared", prepared, additional, -1)
			}
			prepared += info.Size()
		}
	}
	if additional > a.cfg.WorkspaceMaxBytes-total {
		return limited("budget", total, additional, -1)
	}
	return workspaceTotal(a.diskCheck(ws.Path, additional+diskHeadroom), total, additional)
}

// The disk check knows what the filesystem has left and nothing about the
// directory; the caller knows the opposite. Both numbers belong on the one
// error that leaves the check, so they are joined here.
func workspaceTotal(err error, total, additional int64) error {
	if err == nil {
		return nil
	}
	var limit *workspaceLimit
	if errors.As(err, &limit) {
		return &workspaceLimit{error: err, check: limit.check, total: total, additional: additional, free: limit.free}
	}
	if errors.Is(err, clip.ErrWorkspaceLimit) {
		return &workspaceLimit{error: err, check: "disk", total: total, additional: additional, free: -1}
	}
	return err
}

func availableDisk(path string, required int64) error {
	var stat unix.Statfs_t
	if required < 0 || unix.Statfs(path, &stat) != nil || stat.Bsize <= 0 {
		return limited("disk", 0, required, -1)
	}
	// Divide rather than multiply the filesystem's unsigned block count.
	needed := uint64(required)
	block := uint64(stat.Bsize)
	if uint64(stat.Bavail) < (needed+block-1)/block {
		return limited("disk", 0, required, reportedFree(uint64(stat.Bavail), block))
	}
	return nil
}

// Report what is free WITHOUT the multiplication the check above avoids: a
// filesystem large enough to overflow the product reports the largest size the
// log can carry, which is true enough for a number that only says "not this".
func reportedFree(blocks, block uint64) int64 {
	if blocks > uint64(math.MaxInt64)/block {
		return math.MaxInt64
	}
	return int64(blocks * block)
}

func (a *Adapter) run(ctx context.Context, ws clip.MediaWorkspace, binary string, args ...string) ([]byte, error) {
	return a.runBounded(ctx, ws, binary, "", 0, nil, args...)
}

// runLog runs a command that writes nothing but its own log: the loudness
// measurement passes, whose report FFmpeg prints only on stderr.
func (a *Adapter) runLog(ctx context.Context, ws clip.MediaWorkspace, binary string, args ...string) ([]byte, error) {
	return a.runCommand(ctx, ws, Command{Binary: binary, Dir: ws.Path, Args: args, CaptureStderr: true}, "", 0, nil)
}

// runBoundedLog is runBounded for a command whose own log is a result: the
// analysis-copy pass carries the source's verification output beside the copy.
func (a *Adapter) runBoundedLog(ctx context.Context, ws clip.MediaWorkspace, binary, output string, limit int64, limitError error, args ...string) ([]byte, error) {
	return a.runCommand(ctx, ws, Command{Binary: binary, Dir: ws.Path, Args: args, CaptureStderr: true}, output, limit, limitError)
}

// Only subprocess execution is serialized: caption measurement can open a nested
// workspace during planning, so a workspace-wide semaphore would deadlock.
func (a *Adapter) runBounded(ctx context.Context, ws clip.MediaWorkspace, binary, output string, limit int64, limitError error, args ...string) ([]byte, error) {
	return a.runCommand(ctx, ws, Command{Binary: binary, Dir: ws.Path, Args: args}, output, limit, limitError)
}
func (a *Adapter) runCommand(ctx context.Context, ws clip.MediaWorkspace, command Command, output string, limit int64, limitError error) (data []byte, err error) {
	// Report against the caller's context: the timed one below is cancelled by
	// the time the record is written, and a Value read must still find the sink.
	started, caller := time.Now(), ctx
	defer func() {
		elapsed := time.Since(started)
		err = commandDiagnostic(err, command, elapsed)
		clip.ReportMediaOperation(caller, commandOperation(command), commandOutcome(err), elapsed)
	}()
	ctx, cancel := context.WithTimeout(ctx, a.cfg.OperationTimeout)
	defer cancel()
	select {
	case a.process <- struct{}{}:
		defer func() { <-a.process }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	check := func() error {
		if err := a.capacity(ws, 0); err != nil {
			return err
		}
		if output != "" {
			if filepath.Dir(output) != ws.Path || limit <= 0 || limitError == nil {
				return limited("output", 0, limit, -1)
			}
			info, err := os.Lstat(output)
			if err != nil && !os.IsNotExist(err) {
				return limited("output", 0, limit, -1)
			}
			if err == nil && (!info.Mode().IsRegular() || info.Size() > limit) {
				return limitError
			}
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, err
	}
	done, monitored := make(chan struct{}), make(chan error, 1)
	go func() {
		ticker := time.NewTicker(a.cfg.DiskCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				monitored <- nil
				return
			case <-ctx.Done():
				monitored <- nil
				return
			case <-ticker.C:
				if err := check(); err != nil {
					cancel()
					monitored <- err
					return
				}
			}
		}
	}()
	// Always stop the monitor, including a custom runner panic. The outer
	// workspace defer then owns cleanup after the child has stopped.
	defer close(done)
	data, err = a.runner.Run(ctx, command)
	cancel()
	monitorErr := <-monitored
	return data, errors.Join(monitorErr, check(), err)
}
