package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

const workspacePrefix = "postpilot-clip-"
const rootMarker = ".postpilot-clip-root"

var workspaceName = regexp.MustCompile(`^postpilot-clip-[0-9a-f]{32}$`)

func unsafeRoot(root string) bool {
	home, _ := os.UserHomeDir()
	realHome, _ := filepath.EvalSymlinks(home)
	return root == home || root == realHome || filepath.Dir(root) == string(filepath.Separator) || slices.Contains([]string{"/var/tmp", "/private/tmp", "/private/var", "/private/var/tmp"}, root)
}

type Adapter struct {
	cfg       clip.MediaConfig
	runner    Runner
	mu        sync.Mutex
	active    map[string]bool
	rootInfo  os.FileInfo
	process   chan struct{}
	diskCheck func(string, int64) error
}

var _ clip.Media = (*Adapter)(nil)

func New(cfg clip.MediaConfig, runner Runner) (*Adapter, error) {
	if cfg.WorkRoot == "" || strings.ContainsAny(cfg.WorkRoot, "$\x00") || !filepath.IsAbs(cfg.WorkRoot) || filepath.Clean(cfg.WorkRoot) != cfg.WorkRoot || unsafeRoot(cfg.WorkRoot) {
		return nil, errors.New("clip work root must be a dedicated absolute directory")
	}
	home, _ := os.UserHomeDir()
	if home != "" && filepath.Clean(home) == cfg.WorkRoot {
		return nil, errors.New("clip work root must not be a home directory")
	}
	if cfg.StaleAge <= 0 || cfg.OperationTimeout <= 0 || cfg.WaitDelay <= 0 || cfg.ChunkDurationMS <= 0 || cfg.ChunkDurationMS > 60000 || cfg.LongEdge <= 0 || cfg.LongEdge > 720 || cfg.FPS != 15 || cfg.Threads != 1 || cfg.StdoutLimit <= 0 || cfg.StderrLimit <= 0 || cfg.MaxStreams <= 0 || cfg.MaxDimension <= 0 || cfg.FFmpegPath == "" || cfg.FFprobePath == "" {
		return nil, errors.New("invalid clip media configuration")
	}
	if cfg.AnalysisMaxBytes <= 0 || cfg.AnalysisMaxBytes > 8<<20 || cfg.PreparedMaxBytes < cfg.AnalysisMaxBytes || cfg.PreparedMaxBytes > 512<<20 || cfg.WorkspaceMaxBytes < cfg.PreparedMaxBytes || cfg.WorkspaceMaxBytes > 8<<30 || cfg.VideoMaxRate <= 0 || cfg.VideoBufferSize <= 0 || cfg.RetryMaxRate <= 0 || cfg.RetryMaxRate >= cfg.VideoMaxRate || cfg.RetryBufferSize <= 0 || cfg.DiskCheckInterval <= 0 {
		return nil, errors.New("invalid clip resource limits")
	}
	if err := os.MkdirAll(cfg.WorkRoot, 0700); err != nil {
		return nil, fmt.Errorf("create clip work root: %w", err)
	}
	// Resolve the parent once (macOS /tmp is a symlink), but never accept a root
	// which is itself a symlink. Every operation revalidates this actual root.
	info, err := os.Lstat(cfg.WorkRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("clip work root is not a real directory")
	}
	root, err := filepath.EvalSymlinks(cfg.WorkRoot)
	if err != nil {
		return nil, err
	}
	if unsafeRoot(root) {
		return nil, errors.New("clip work root resolves to a protected directory")
	}
	cfg.WorkRoot = root
	marker := filepath.Join(root, rootMarker)
	if _, err := os.Lstat(marker); os.IsNotExist(err) {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		if len(entries) != 0 {
			return nil, errors.New("clip work root is not an empty dedicated directory")
		}
		file, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		if _, err := file.WriteString("postpilot-clip-work-v1\n"); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	}
	markerInfo, err := os.Lstat(marker)
	if err != nil || !markerInfo.Mode().IsRegular() || markerInfo.Size() != int64(len("postpilot-clip-work-v1\n")) {
		return nil, errors.New("clip work root ownership marker is invalid")
	}
	markerData, err := os.ReadFile(marker)
	if err != nil || string(markerData) != "postpilot-clip-work-v1\n" {
		return nil, errors.New("clip work root ownership marker is invalid")
	}
	if err := os.Chmod(root, 0700); err != nil {
		return nil, err
	}
	if runner == nil {
		runner = ExecRunner{StdoutLimit: cfg.StdoutLimit, StderrLimit: cfg.StderrLimit, WaitDelay: cfg.WaitDelay}
	}
	return &Adapter{cfg: cfg, runner: runner, active: map[string]bool{}, rootInfo: info, process: make(chan struct{}, 1), diskCheck: availableDisk}, nil
}
func (a *Adapter) validRoot() error {
	info, err := os.Lstat(a.cfg.WorkRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, a.rootInfo) {
		return errors.New("clip work root changed")
	}
	real, err := filepath.EvalSymlinks(a.cfg.WorkRoot)
	if err != nil || real != a.cfg.WorkRoot {
		return errors.New("clip work root changed")
	}
	return nil
}
func (a *Adapter) validWorkspace(ws clip.MediaWorkspace, requireActive bool) error {
	if err := a.validRoot(); err != nil {
		return err
	}
	if filepath.Dir(ws.Path) != a.cfg.WorkRoot || !workspaceName.MatchString(filepath.Base(ws.Path)) {
		return errors.New("invalid clip workspace")
	}
	info, err := os.Lstat(ws.Path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid clip workspace")
	}
	if requireActive {
		a.mu.Lock()
		active := a.active[ws.Path]
		a.mu.Unlock()
		if !active {
			return errors.New("inactive clip workspace")
		}
	}
	return nil
}
func (a *Adapter) sourcePath(ws clip.MediaWorkspace, path string) error {
	if err := a.validWorkspace(ws, true); err != nil {
		return err
	}
	if filepath.Dir(path) != ws.Path || filepath.Clean(path) != path {
		return clip.ErrInvalidMedia
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > a.cfg.Sources.MaxFileBytes {
		return clip.ErrInvalidMedia
	}
	return nil
}
func (a *Adapter) WithWorkspace(ctx context.Context, jobID string, fn func(clip.MediaWorkspace) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(jobID) == "" || fn == nil {
		return errors.New("clip workspace requires a job and callback")
	}
	if err := a.validRoot(); err != nil {
		return err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return err
	}
	ws := clip.MediaWorkspace{Path: filepath.Join(a.cfg.WorkRoot, workspacePrefix+hex.EncodeToString(id[:]))}
	if err := os.Mkdir(ws.Path, 0700); err != nil {
		return err
	}
	a.mu.Lock()
	a.active[ws.Path] = true
	a.mu.Unlock()
	ws.CheckCapacity = func(additional int64) error { return a.capacity(ws, additional) }
	defer func() {
		cleanup := a.removeWorkspace(ws)
		a.mu.Lock()
		delete(a.active, ws.Path)
		a.mu.Unlock()
		err = errors.Join(err, cleanup)
	}()
	return fn(ws)
}
func (a *Adapter) removeWorkspace(ws clip.MediaWorkspace) error {
	if err := a.validWorkspace(ws, false); err != nil {
		return err
	}
	return os.RemoveAll(ws.Path) // validated direct child only, never the root or an unresolved path
}
func (a *Adapter) CleanupStale(ctx context.Context, now time.Time) error {
	if err := a.validRoot(); err != nil {
		return err
	}
	entries, err := os.ReadDir(a.cfg.WorkRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || !workspaceName.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(a.cfg.WorkRoot, entry.Name())
		a.mu.Lock()
		active := a.active[path]
		a.mu.Unlock()
		if active {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if now.Sub(info.ModTime()) < a.cfg.StaleAge {
			continue
		}
		if err := a.removeWorkspace(clip.MediaWorkspace{Path: path}); err != nil {
			return err
		}
	}
	return nil
}
