package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const devToolsActivePort = "DevToolsActivePort"

var endpointTimeout = 10 * time.Second

type Installation struct {
	Label  string
	Binary string
}

// Session is a verified loopback CDP connection for one dedicated browser
// profile. Close terminates the browser only when this process launched it;
// an already-open setup/login browser remains under the user's control.
type Session struct {
	CDPURL  string
	command *exec.Cmd
	done    chan error
	once    sync.Once
}

var candidates = []Installation{
	{Label: "Google Chrome", Binary: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
	{Label: "Chromium", Binary: "/Applications/Chromium.app/Contents/MacOS/Chromium"},
	{Label: "Microsoft Edge", Binary: "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"},
	{Label: "Brave Browser", Binary: "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"},
}

func Discover() []Installation {
	found := make([]Installation, 0, len(candidates))
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate.Binary); err == nil && !info.IsDir() {
			found = append(found, candidate)
		}
	}
	return found
}

func Supported(binary string) (Installation, bool) {
	for _, installation := range Discover() {
		if installation.Binary == binary {
			return installation, true
		}
	}
	return Installation{}, false
}

func PrepareProfile(root, connectionID string) (string, error) {
	if connectionID == "" {
		return "", errors.New("empty connection id")
	}
	dir := filepath.Join(root, connectionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func OpenLogin(binary, profileDir string) error {
	_, err := Start(binary, profileDir, "https://nid.naver.com/nidlogin.login")
	return err
}

// Start reuses a live CDP endpoint recorded by the dedicated profile or starts
// the selected Chromium-family browser with an ephemeral loopback debugging
// port. Chrome 136+ requires a non-default --user-data-dir for remote debugging.
func Start(binary, profileDir, initialURL string) (*Session, error) {
	if _, err := os.Stat(binary); err != nil {
		return nil, err
	}
	if existing, err := Connect(profileDir); err == nil {
		if initialURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), endpointTimeout)
			defer cancel()
			if err := navigateSinglePage(ctx, existing.CDPURL, initialURL); err != nil {
				return nil, fmt.Errorf("navigate existing dedicated browser: %w", err)
			}
		}
		return existing, nil
	}
	if err := os.Remove(filepath.Join(profileDir, devToolsActivePort)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if initialURL == "" {
		initialURL = "about:blank"
	}
	command := exec.Command(
		binary,
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=0",
		"--user-data-dir="+profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		initialURL,
	)
	if err := command.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	deadline := time.Now().Add(endpointTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if session, err := Connect(profileDir); err == nil {
			session.command = command
			session.done = done
			return session, nil
		} else {
			lastErr = err
		}
		select {
		case err := <-done:
			if err == nil {
				err = errors.New("browser exited before exposing CDP")
			}
			return nil, err
		case <-time.After(100 * time.Millisecond):
		}
	}
	_ = command.Process.Kill()
	<-done
	return nil, fmt.Errorf("browser CDP did not become ready: %w", lastErr)
}

// Connect verifies the profile's DevToolsActivePort file against Chrome's
// version endpoint and returns the exact browser WebSocket URL Hermes expects.
func Connect(profileDir string) (*Session, error) {
	data, err := os.ReadFile(filepath.Join(profileDir, devToolsActivePort))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return nil, errors.New("invalid DevToolsActivePort")
	}
	expectedBrowserPath := strings.TrimSpace(lines[1])
	if !strings.HasPrefix(expectedBrowserPath, "/devtools/browser/") {
		return nil, errors.New("invalid DevToolsActivePort browser path")
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("invalid DevToolsActivePort port")
	}
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port) + "/json/version"
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("browser CDP returned %d", response.StatusCode)
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(response.Body).Decode(&version); err != nil {
		return nil, err
	}
	if err := validateCDPURL(version.WebSocketDebuggerURL, port, expectedBrowserPath); err != nil {
		return nil, err
	}
	return &Session{CDPURL: version.WebSocketDebuggerURL}, nil
}

func validateCDPURL(value string, expectedPort int, expectedBrowserPath string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("browser returned an invalid CDP WebSocket URL")
	}
	host := parsed.Hostname()
	address := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (address == nil || !address.IsLoopback()) {
		return errors.New("browser returned a non-loopback CDP WebSocket URL")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port != expectedPort {
		return errors.New("browser returned a mismatched CDP WebSocket port")
	}
	if parsed.EscapedPath() != expectedBrowserPath {
		return errors.New("browser returned a mismatched DevTools browser id")
	}
	return nil
}

func (s *Session) Close() error {
	if s == nil || s.command == nil {
		return nil
	}
	var closeErr error
	s.once.Do(func() {
		_ = s.command.Process.Signal(os.Interrupt)
		select {
		case closeErr = <-s.done:
		case <-time.After(3 * time.Second):
			if err := s.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				closeErr = err
			}
			<-s.done
		}
	})
	return closeErr
}
