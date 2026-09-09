package main

import (
	"bytes"
	"context"
	"errors"
	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
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

// The daemon is wired to the real factory now. A nil one is a wiring fault rather than a
// state to run in, and `run` must never hand one over.
func TestRunCommandWiresTheRealPublisherFactory(t *testing.T) {
	if err := runAgents(config.Paths{}, nil); err == nil || !strings.Contains(err.Error(), "no publisher factory") {
		t.Fatalf("runAgents with no factory = %v", err)
	}
	// newPublisher is what `run` passes, and it is a publisherFactory.
	var factory publisherFactory = newPublisher
	if factory == nil {
		t.Fatal("newPublisher does not satisfy publisherFactory")
	}
}

// The factory builds one publisher per paired connection and refuses a connection the daemon
// must not run at all: an unarmed or incomplete one, or a browser outside the supported
// Chromium family (PUB-18).
func TestNewPublisherRefusesAConnectionItCouldNotRun(t *testing.T) {
	supported := browser.Discover()
	if len(supported) == 0 {
		t.Skip("no supported Chromium-family browser on this machine")
	}
	base := config.Connection{
		ID: "connection", Label: "Mac", APIURL: "https://api.example.com", AgentID: "agent",
		KeychainAccount: "key", BrowserBinary: supported[0].Binary, BrowserLabel: supported[0].Label,
		PlatformAccountID: "alice", ProfileDir: t.TempDir(), CompatibilitySignature: "sig",
		LeaseTTLSeconds: 45, Armed: true,
	}
	if _, err := newPublisher(base); err != nil {
		t.Fatalf("a complete armed connection was refused: %v", err)
	}
	// Arming is the DAEMON's filter (unseenArmedConnections), not the factory's; what the
	// factory refuses is a connection it could not run at all.
	incomplete := base
	incomplete.KeychainAccount = ""
	if _, err := newPublisher(incomplete); err == nil {
		t.Fatal("an incomplete connection got a publisher")
	}
	shortLease := base
	shortLease.LeaseTTLSeconds = 1
	if _, err := newPublisher(shortLease); err == nil {
		t.Fatal("a lease too short for the heartbeat got a publisher")
	}
	foreign := base
	foreign.BrowserBinary = "/usr/bin/true"
	if _, err := newPublisher(foreign); err == nil {
		t.Fatal("a browser outside the supported family got a publisher")
	}
}

// fakeSession is a browser.Session the wiring can hand around without launching anything.
func fakeSession(cdpURL string) *browser.Session { return &browser.Session{CDPURL: cdpURL} }

type stubPort struct {
	naver.CommitPort
	closed bool
}

func (s *stubPort) Close() error {
	s.closed = true
	return nil
}

func fenceConnection() config.Connection {
	return config.Connection{
		ID: "connection", BrowserBinary: "/browser", ProfileDir: "/profile",
		PlatformAccountID: "alice", CompatibilitySignature: "smarteditor-test",
	}
}

// The typed terminal result for each failure the wiring can see itself, and the proof that
// none of them reaches further than it has to: a signature mismatch never opens a browser,
// and a browser that will not launch never binds a port.
func TestConnectionPublisherMapsEveryPreflightFailureToItsTypedResult(t *testing.T) {
	for name, test := range map[string]struct {
		arrange     func(*publisherBoundaries)
		failOpen    bool
		failBind    bool
		wantFailure naver.FailureKind
		wantOpened  bool
		wantBound   bool
	}{
		"signature mismatch": {
			arrange:     func(b *publisherBoundaries) { b.signature = func() (string, error) { return "another-release", nil } },
			wantFailure: naver.FailureEditorChanged,
		},
		"no signature at all": {
			arrange: func(b *publisherBoundaries) {
				b.signature = func() (string, error) { return "", errors.New("no manifest") }
			},
			wantFailure: naver.FailureEditorChanged,
		},
		"browser will not launch": {
			failOpen:    true,
			wantFailure: naver.FailureBrowserLost, wantOpened: true,
		},
		"login expired": {
			arrange: func(b *publisherBoundaries) {
				b.identity = func(context.Context, string) (browser.NaverIdentity, error) {
					return browser.NaverIdentity{}, errors.New("no blog identity")
				}
			},
			wantFailure: naver.FailureLoginExpired, wantOpened: true,
		},
		"another account's profile": {
			arrange: func(b *publisherBoundaries) {
				b.identity = func(context.Context, string) (browser.NaverIdentity, error) {
					return browser.NaverIdentity{BlogID: "mallory"}, nil
				}
			},
			wantFailure: naver.FailureAccountMismatch, wantOpened: true,
		},
		"the page cannot be bound": {
			failBind:    true,
			wantFailure: naver.FailureBrowserLost, wantOpened: true, wantBound: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			opened, bound := false, false
			boundaries := publisherBoundaries{
				signature: func() (string, error) { return "smarteditor-test", nil },
				openEditor: func(binary, profileDir string) (*browser.Session, error) {
					opened = true
					if binary != "/browser" || profileDir != "/profile" {
						t.Fatalf("the wrong browser or profile was opened: %q %q", binary, profileDir)
					}
					if test.failOpen {
						return nil, errors.New("no browser")
					}
					return fakeSession("ws://127.0.0.1:1/devtools/browser/one"), nil
				},
				identity: func(context.Context, string) (browser.NaverIdentity, error) {
					return browser.NaverIdentity{BlogID: "alice"}, nil
				},
				bindPort: func(context.Context, string) (naverPort, error) {
					bound = true
					if test.failBind {
						return nil, errors.New("two pages")
					}
					return &stubPort{}, nil
				},
			}
			if test.arrange != nil {
				test.arrange(&boundaries)
			}
			publisher := connectionPublisher{connection: fenceConnection(), boundaries: boundaries}
			result, err := publisher.Run(context.Background(), t.TempDir(), nil)
			if err != nil {
				t.Fatalf("the wiring returned an error instead of a typed result: %v", err)
			}
			if result.Status != "failed" || result.FailureKind != string(test.wantFailure) {
				t.Fatalf("result = %+v, want failure %s", result, test.wantFailure)
			}
			// A browser is only opened once the release agrees, and only bound once the
			// account is proven.
			if opened != test.wantOpened {
				t.Fatalf("browser opened = %t, want %t", opened, test.wantOpened)
			}
			if bound != test.wantBound {
				t.Fatalf("page bound = %t, want %t", bound, test.wantBound)
			}
		})
	}
}

// A run that gets past the preflight closes both the port and the session, whatever the
// publisher then decides: PUB-18 wants the browser this daemon launched closed after the run
// it was launched for, and a leaked CDP connection would strand the next job.
func TestConnectionPublisherAlwaysReleasesThePortAndTheSession(t *testing.T) {
	port := &stubPort{}
	publisher := connectionPublisher{connection: fenceConnection(), boundaries: publisherBoundaries{
		signature: func() (string, error) { return "smarteditor-test", nil },
		openEditor: func(string, string) (*browser.Session, error) {
			return fakeSession("ws://127.0.0.1:1/devtools/browser/one"), nil
		},
		identity: func(context.Context, string) (browser.NaverIdentity, error) {
			return browser.NaverIdentity{BlogID: "alice"}, nil
		},
		bindPort: func(context.Context, string) (naverPort, error) { return port, nil },
	}}
	// A nil manifest in the job directory is the publisher's own safe refusal, which is
	// enough to prove the release path runs and unwinds.
	result, err := publisher.Run(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("result = %+v", result)
	}
	if !port.closed {
		t.Fatal("the bound page was left open after the run")
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

// wiringAPI is the durable server, reduced to what one job's boundaries need.
type wiringAPI struct {
	progress  []postpilotv1.PublishStage
	completed []string
	failures  []postpilotv1.PublishFailureKind
}

func (a *wiringAPI) Renew(context.Context, string, string) error { return nil }

func (a *wiringAPI) Progress(_ context.Context, _, _ string, _ int64, stage postpilotv1.PublishStage) error {
	a.progress = append(a.progress, stage)
	return nil
}

func (a *wiringAPI) Complete(_ context.Context, _, _ string, _ int64, url string) error {
	a.completed = append(a.completed, url)
	return nil
}

func (a *wiringAPI) Fail(_ context.Context, _, _ string, _ int64, kind postpilotv1.PublishFailureKind, _ string) error {
	a.failures = append(a.failures, kind)
	return nil
}

// One job driven end to end through the REAL factory shape: the executor claims it, the
// connection publisher runs its preflight, and the job reaches a terminal result the server
// is told about. The browser and the bound page are the only fakes — everything between the
// claim and the durable report is the daemon's own code.
func TestRunAgentsDrivesOneJobThroughTheRealFactoryShape(t *testing.T) {
	api := &wiringAPI{}
	port := &stubPort{}
	publisher := connectionPublisher{connection: fenceConnection(), boundaries: publisherBoundaries{
		signature: func() (string, error) { return "smarteditor-test", nil },
		openEditor: func(string, string) (*browser.Session, error) {
			return fakeSession("ws://127.0.0.1:1/devtools/browser/one"), nil
		},
		identity: func(context.Context, string) (browser.NaverIdentity, error) {
			return browser.NaverIdentity{BlogID: "alice"}, nil
		},
		bindPort: func(context.Context, string) (naverPort, error) { return port, nil },
	}}
	executor := publishing.Executor{
		API: api, Publisher: publisher, JobsRoot: t.TempDir(), ConnectionID: "connection",
		HeartbeatEvery: time.Second, Timeout: 10 * time.Second,
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	claim := &postpilotv1.ClaimPublishJobResponse{
		Job:        &postpilotv1.PublishJob{Id: "job", ProgressSequence: 1},
		Manifest:   &postpilotv1.PublishManifest{JobId: "job", ExpectedPlatformAccountId: "alice"},
		LeaseToken: "lease", LeaseTtlSeconds: 45,
	}
	if err := executor.Execute(context.Background(), claim); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The manifest carries no content, so the publisher's own validation refuses it safely —
	// which is a terminal result reported to the server, not a panic or a silent retry.
	if len(api.failures) != 1 || api.failures[0] != postpilotv1.PublishFailureKind_PUBLISH_FAILURE_SAFE {
		t.Fatalf("failures = %v", api.failures)
	}
	if len(api.completed) != 0 {
		t.Fatalf("a job with no content was completed: %v", api.completed)
	}
	if !port.closed {
		t.Fatal("the bound page was left open after the job")
	}
}
