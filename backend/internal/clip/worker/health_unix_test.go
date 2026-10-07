//go:build unix

package worker

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func unixHealthRequest(ctx context.Context, path string) (*http.Response, error) {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://worker/health", nil)
	if err != nil {
		return nil, err
	}
	return (&http.Client{Transport: transport, Timeout: time.Second}).Do(request)
}

func TestHealthSocketRequiresOwnerAndStopsWithDrain(t *testing.T) {
	root := t.TempDir()
	callback := func(context.Context, *http.Request) ([]byte, error) {
		return []byte(`{"Ready":true,"Profile":"cpu","Waiting":2,"Active":1,"OwnActive":1}`), nil
	}
	if stop, err := StartHealth(t.Context(), root, callback); err == nil {
		stop()
		t.Fatal("stopped root published health")
	}
	release, err := LockWorkRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	stop, err := StartHealth(ctx, root, callback)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	path := HealthSocketPath(root)
	socket, err := os.Stat(path)
	if err != nil || socket.Mode().Perm() != 0600 {
		t.Fatal("health socket is not private", err)
	}
	directory, err := os.Stat(filepath.Dir(path))
	if err != nil || directory.Mode().Perm() != 0700 {
		t.Fatal("health directory is not private", err)
	}
	response, err := unixHealthRequest(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || !strings.Contains(string(raw), `"Ready":true`) {
		t.Fatal("active worker health rejected", err, string(raw))
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err = os.Stat(path); os.IsNotExist(err) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("drain kept health socket", err)
	}
	if response, err = unixHealthRequest(t.Context(), path); err == nil {
		response.Body.Close()
		t.Fatal("draining worker reported ready")
	}
}

func TestHealthSocketRejectsSuccessorAndUnavailableStatus(t *testing.T) {
	for _, mode := range []string{"api-unavailable", "owner-changed", "too-large"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			release, err := LockWorkRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			stop, err := StartHealth(t.Context(), root, func(context.Context, *http.Request) ([]byte, error) {
				switch mode {
				case "api-unavailable":
					return nil, errors.New("private API unavailable")
				case "owner-changed":
					release()
					next, err := LockWorkRoot(root)
					if err != nil {
						return nil, err
					}
					t.Cleanup(next)
				case "too-large":
					return []byte(strings.Repeat("x", 4097)), nil
				}
				return []byte(`{"Ready":true}`), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			response, err := unixHealthRequest(t.Context(), HealthSocketPath(root))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 503 {
				t.Fatal("unsafe health response", response.StatusCode)
			}
		})
	}
}

func TestHealthSocketNamespaceAcceptsLongWorkRoots(t *testing.T) {
	root := filepath.Join(t.TempDir(), strings.Repeat("long-work-root", 12))
	a, b := HealthSocketPath(root), HealthSocketPath(root+"-other")
	if len(a) >= 104 || a == b {
		t.Fatal("unsafe health socket namespace", a, b)
	}
}
