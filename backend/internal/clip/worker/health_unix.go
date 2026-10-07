//go:build unix

package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HealthSocketPath stays container-local and short enough for Unix sockets even
// when the configured identity workspace has a long pathname.
func HealthSocketPath(root string) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join("/tmp", "postpilot-health-"+hex.EncodeToString(sum[:]), "h.sock")
}

// StartHealth serves readiness inside the current validated worker process.
// No external listener or additional media process is admitted.
func StartHealth(ctx context.Context, root string, status func(context.Context, *http.Request) ([]byte, error)) (func(), error) {
	if err := CheckWorkRootActive(root); err != nil {
		return nil, err
	}
	owner, err := readRootOwner(root)
	if err != nil || owner.PID != os.Getpid() {
		return nil, errors.New("health listener requires the worker root owner")
	}
	path := HealthSocketPath(root)
	directory := filepath.Dir(path)
	if err = os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return nil, errors.New("invalid worker health directory")
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(false)
	if err = os.Chmod(path, 0600); err != nil {
		listener.Close()
		os.Remove(path)
		return nil, err
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unavailable := func() {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"Ready":false}`, http.StatusServiceUnavailable)
		}
		if r.Method != http.MethodGet || r.URL.Path != "/health" || r.ContentLength > 0 || ctx.Err() != nil {
			unavailable()
			return
		}
		current, err := readRootOwner(root)
		if err != nil || current != owner || CheckWorkRootActive(root) != nil {
			unavailable()
			return
		}
		check, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		raw, err := status(check, r)
		if err != nil || len(raw) == 0 || len(raw) > 4096 || ctx.Err() != nil {
			unavailable()
			return
		}
		current, err = readRootOwner(root)
		if err != nil || current != owner || CheckWorkRootActive(root) != nil {
			unavailable()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 11 * time.Second, MaxHeaderBytes: 4096}
	done := make(chan struct{})
	var once sync.Once
	close := func() {
		once.Do(func() {
			_ = server.Close()
			current, err := readRootOwner(root)
			if err == nil && current == owner {
				_ = os.Remove(path)
				_ = os.Remove(directory)
			}
			close(done)
		})
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "worker health listener stopped")
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			close()
		case <-done:
		}
	}()
	return close, nil
}
