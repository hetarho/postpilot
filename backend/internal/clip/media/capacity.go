package media

import (
	"context"
	"errors"
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

func (a *Adapter) capacity(ws clip.MediaWorkspace, additional int64) error {
	if err := a.validWorkspace(ws, true); err != nil {
		return err
	}
	if additional < 0 || additional > a.cfg.WorkspaceMaxBytes {
		return clip.ErrWorkspaceLimit
	}
	entries, err := os.ReadDir(ws.Path)
	if err != nil {
		return clip.ErrWorkspaceLimit
	}
	var total, prepared int64
	for _, entry := range entries {
		info, err := entry.Info()
		if os.IsNotExist(err) { // a successfully consumed proxy may just be released
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > a.cfg.WorkspaceMaxBytes-total {
			return clip.ErrWorkspaceLimit
		}
		total += info.Size()
		if strings.HasPrefix(entry.Name(), "proxy-") {
			if info.Size() > a.cfg.PreparedMaxBytes-prepared {
				return clip.ErrWorkspaceLimit
			}
			prepared += info.Size()
		}
	}
	if additional > a.cfg.WorkspaceMaxBytes-total {
		return clip.ErrWorkspaceLimit
	}
	return a.diskCheck(ws.Path, additional+diskHeadroom)
}

func availableDisk(path string, required int64) error {
	var stat unix.Statfs_t
	if required < 0 || unix.Statfs(path, &stat) != nil || stat.Bsize <= 0 {
		return clip.ErrWorkspaceLimit
	}
	// Divide rather than multiply the filesystem's unsigned block count.
	needed := uint64(required)
	block := uint64(stat.Bsize)
	if uint64(stat.Bavail) < (needed+block-1)/block {
		return clip.ErrWorkspaceLimit
	}
	return nil
}

func (a *Adapter) run(ctx context.Context, ws clip.MediaWorkspace, binary string, args ...string) ([]byte, error) {
	return a.runBounded(ctx, ws, binary, "", 0, nil, args...)
}

// Only subprocess execution is serialized: caption measurement can open a nested
// workspace during planning, so a workspace-wide semaphore would deadlock.
func (a *Adapter) runBounded(ctx context.Context, ws clip.MediaWorkspace, binary, output string, limit int64, limitError error, args ...string) ([]byte, error) {
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
				return clip.ErrWorkspaceLimit
			}
			info, err := os.Lstat(output)
			if err != nil && !os.IsNotExist(err) {
				return clip.ErrWorkspaceLimit
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
	data, err := a.runner.Run(ctx, Command{Binary: binary, Dir: ws.Path, Args: args})
	cancel()
	monitorErr := <-monitored
	return data, errors.Join(monitorErr, check(), err)
}
