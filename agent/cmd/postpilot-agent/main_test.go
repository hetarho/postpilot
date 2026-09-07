package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/agent/internal/browser"
	"github.com/postpilot/agent/internal/config"
	"github.com/postpilot/agent/internal/credentials"
	"github.com/postpilot/agent/internal/naver"
	"github.com/postpilot/agent/internal/publishing"
	"github.com/postpilot/agent/internal/singleton"
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

// PUBLISH-23: startup removes abandoned job directories only after acquiring the owner-only
// lock. A second daemon that loses the race must leave the running one's payloads alone.
func TestASecondDaemonLeavesTheRunningOnesJobPayloadsUntouched(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "jobs")
	abandoned := filepath.Join(jobs, "connection-1", "job-abcdef")
	if err := os.MkdirAll(abandoned, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(abandoned, "0000.jpg")
	if err := os.WriteFile(payload, []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	held, err := singleton.Acquire(filepath.Join(root, "run.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Jobs: jobs, Logs: filepath.Join(root, "logs")}
	err = runAgents(paths, func(config.Connection) (publishing.Publisher, error) { return nil, nil })
	if !errors.Is(err, singleton.ErrAlreadyRunning) {
		t.Fatalf("second daemon error = %v, want the singleton refusal", err)
	}
	if _, statErr := os.Stat(payload); statErr != nil {
		t.Fatalf("the losing daemon deleted a live job payload: %v", statErr)
	}
}

// The packaging scripts are the reproducible half of PUBLISH-29 and the one place the
// uninstall promise is actually kept, so both are asserted as content rather than trusted.
func TestPackagingScriptsInstallReproduciblyAndUninstallKeepsCredentials(t *testing.T) {
	install, err := os.ReadFile(filepath.Join("..", "..", "packaging", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"set -eu", "go build -trimpath", "launchctl bootout", "postpilot-agent\" install"} {
		if !strings.Contains(string(install), required) {
			t.Fatalf("install.sh lacks %q", required)
		}
	}
	uninstall, err := os.ReadFile(filepath.Join("..", "..", "packaging", "uninstall.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(uninstall), `"$BIN" uninstall`) {
		t.Fatal("uninstall.sh does not remove the LaunchAgent through the reviewed subcommand")
	}
	// Nothing here may delete a browser profile, the config or a Keychain credential: each
	// needs its own explicit confirmation, which a script cannot give on the user's behalf.
	destructive := regexp.MustCompile(`(?i)\brm\s+-[a-z]*r|security\s+delete-generic-password|defaults\s+delete|browser-profiles|config\.json`)
	if match := destructive.FindString(string(uninstall)); match != "" {
		t.Fatalf("uninstall.sh removes user data without a separate confirmation: %q", match)
	}
	for _, promised := range []string{"browser profiles", "Keychain"} {
		if !strings.Contains(string(uninstall), promised) {
			t.Fatalf("uninstall.sh does not tell the user %q was kept", promised)
		}
	}
}

type fakePublisher string

func (fakePublisher) Run(context.Context, string, publishing.Reporter) (publishing.Result, error) {
	return publishing.Result{}, nil
}

// PUBLISH-23: two accounts paired on one Mac each get their own Keychain credential, API
// client, dedicated browser profile and job directory, and share only the permit that
// serializes publication. A credential one account cannot read must not stop the other.
func TestTwoPairedAccountsGetIsolatedCredentialsPublishersAndJobDirectories(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	account := func(id string) config.Connection {
		return config.Connection{ID: id, Label: id, APIURL: "https://api.example.com", AgentID: "agent-" + id,
			KeychainAccount: "key-" + id, BrowserBinary: "/browser", BrowserLabel: "Chrome",
			PlatformAccountID: "blog-" + id, ProfileDir: "/profiles/" + id, LeaseTTLSeconds: 45, Armed: true}
	}
	first, second, unreadable := account("alice"), account("bob"), account("carol")
	keychain := diagnosticKeychain{"key-alice": "token-alice", "key-bob": "token-bob"}
	askedFor := map[string]string{}
	permit := make(chan struct{}, 1)
	deps := daemonDeps{
		paths: paths, keychain: keychain, permit: permit,
		logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		newPublisher: func(connection config.Connection) (publishing.Publisher, error) {
			askedFor[connection.ID] = connection.ProfileDir
			return fakePublisher(connection.ID), nil
		},
	}

	supervisors := map[string]publishing.Supervisor{}
	for _, connection := range []config.Connection{first, second} {
		supervisor, err := buildSupervisor(context.Background(), connection, deps)
		if err != nil {
			t.Fatalf("%s: %v", connection.ID, err)
		}
		supervisors[connection.ID] = supervisor
	}
	alice, bob := supervisors["alice"], supervisors["bob"]
	aliceExecutor, aliceOK := alice.Executor.(publishing.Executor)
	bobExecutor, bobOK := bob.Executor.(publishing.Executor)
	if !aliceOK || !bobOK {
		t.Fatalf("executors = %T %T", alice.Executor, bob.Executor)
	}
	if aliceExecutor.ConnectionID != "alice" || bobExecutor.ConnectionID != "bob" {
		t.Fatalf("connection ids = %q %q", aliceExecutor.ConnectionID, bobExecutor.ConnectionID)
	}
	if aliceExecutor.Publisher == bobExecutor.Publisher || alice.Client == bob.Client {
		t.Fatal("the two accounts share a publisher or an API client")
	}
	if askedFor["alice"] != first.ProfileDir || askedFor["bob"] != second.ProfileDir {
		t.Fatalf("dedicated browser profiles crossed accounts: %v", askedFor)
	}
	if alice.Permit != bob.Permit {
		t.Fatal("the accounts do not share the permit that serializes publication")
	}
	if _, err := buildSupervisor(context.Background(), unreadable, deps); err == nil {
		t.Fatal("an account with no Keychain credential was started")
	}
	if _, started := askedFor["carol"]; started {
		t.Fatal("a publisher was built for an account whose credential was unavailable")
	}
}

// launchd restarts the daemon after a crash (KeepAlive/SuccessfulExit false), and that
// restart is what clears payloads a killed run left behind (PUBLISH-23).
func TestARestartClearsPayloadsLeftBehindByAKilledRun(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "jobs")
	abandoned := filepath.Join(jobs, "connection-1", "job-abcdef")
	if err := os.MkdirAll(abandoned, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(abandoned, "0000.jpg"), []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Jobs: jobs, Logs: filepath.Join(root, "logs")}
	err := runAgentsWith(paths, func(config.Connection) (publishing.Publisher, error) { return fakePublisher("unused"), nil }, diagnosticKeychain{})
	if err == nil || !strings.Contains(err.Error(), "no armed publishing connection") {
		t.Fatalf("restart error = %v", err)
	}
	if _, statErr := os.Stat(abandoned); !os.IsNotExist(statErr) {
		t.Fatalf("the restart kept an abandoned job directory: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(jobs, "connection-1")); statErr != nil {
		t.Fatalf("the restart removed the stable connection directory: %v", statErr)
	}
}
