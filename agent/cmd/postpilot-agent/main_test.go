package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/agent/internal/browser"
	"github.com/postpilot/agent/internal/config"
	"github.com/postpilot/agent/internal/credentials"
	"github.com/postpilot/agent/internal/naver"
)

func TestRunAgentsRefusesBeforeClaimWithoutDeterministicPublisher(t *testing.T) {
	err := runAgents(config.Paths{}, nil)
	if err == nil || !strings.Contains(err.Error(), "deterministic Naver publisher is not implemented") {
		t.Fatalf("runAgents error = %v", err)
	}
}

func TestUnseenArmedConnectionsFindsAccountAddedWhileDaemonRuns(t *testing.T) {
	cfg := config.File{Connections: []config.Connection{
		{ID: "already-running", Armed: true},
		{ID: "new-account", Armed: true},
		{ID: "setup-not-finished", Armed: false},
	}}
	got := unseenArmedConnections(cfg, map[string]struct{}{"already-running": {}})
	if len(got) != 1 || got[0].ID != "new-account" {
		t.Fatalf("new connections = %#v", got)
	}
}

func TestDaemonReloadReadsANewArmedConnectionWithoutRestart(t *testing.T) {
	if connectionReloadInterval != 2*time.Second {
		t.Fatalf("reload interval=%s", connectionReloadInterval)
	}
	root := t.TempDir()
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	initial := config.Connection{ID: "first", ProfileDir: "/legacy/first", Armed: true}
	if err := config.Save(paths, config.File{Connections: []config.Connection{initial}}); err != nil {
		t.Fatal(err)
	}
	launched := map[string]struct{}{"first": {}}
	if got, err := reloadArmedConnections(paths, launched); err != nil || len(got) != 0 {
		t.Fatalf("initial reload=%v err=%v", got, err)
	}
	second := config.Connection{ID: "second", ProfileDir: "/legacy/second", Armed: true}
	if err := config.Save(paths, config.File{Connections: []config.Connection{initial, second}}); err != nil {
		t.Fatal(err)
	}
	got, err := reloadArmedConnections(paths, launched)
	if err != nil || len(got) != 1 || got[0].ID != "second" {
		t.Fatalf("new connection reload=%v err=%v", got, err)
	}
}

type diagnosticKeychain map[string]string

var _ credentials.Store = diagnosticKeychain{}

func (store diagnosticKeychain) Put(_ context.Context, account, token string) error {
	store[account] = token
	return nil
}
func (store diagnosticKeychain) Get(_ context.Context, account string) (string, error) {
	if store[account] == "" {
		return "", errors.New("missing")
	}
	return store[account], nil
}
func (store diagnosticKeychain) Delete(_ context.Context, account string) error {
	delete(store, account)
	return nil
}

func TestDiagnosticsPrintsOnlySafeLabelsAndReviewedVersions(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	connection := config.Connection{ID: "private-connection-id", Label: "침실 Mac", APIURL: "https://api.example.com", AgentID: "private-agent-id", KeychainAccount: "key", BrowserBinary: "/private/browser", BrowserLabel: "Chrome", PlatformAccountID: "private-blog-id", ProfileDir: "/private/profile", LeaseTTLSeconds: 45, Armed: true}
	if err := config.Save(paths, config.File{Connections: []config.Connection{connection}}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := diagnosticsWith(paths, diagnosticKeychain{"key": "private-token"},
		func(binary, profile, initialURL string) (*browser.Session, error) {
			if binary != connection.BrowserBinary || profile != connection.ProfileDir || initialURL != "" {
				t.Fatal("diagnostics changed the dedicated browser selection")
			}
			return &browser.Session{CDPURL: "ws://127.0.0.1/private"}, nil
		},
		func(context.Context, string) (naver.Result, error) {
			return naver.Result{Identity: browser.NaverIdentity{BlogID: connection.PlatformAccountID}, BrowserVersion: "Chrome/152.0", SignatureID: "signed-v1"}, nil
		}, &output)
	if err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "침실 Mac: ready (Chrome/152.0, driver signed-v1)") {
		t.Fatalf("safe diagnostics missing: %q", got)
	}
	for _, secret := range []string{"private-token", "private-blog-id", "private-agent-id", "private-connection-id", "/private/"} {
		if strings.Contains(got, secret) {
			t.Fatalf("diagnostics exposed %q: %s", secret, got)
		}
	}
}
