package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/postpilot/agent/internal/browser"
	"github.com/postpilot/agent/internal/config"
	"github.com/postpilot/agent/internal/credentials"
	"github.com/postpilot/agent/internal/launchd"
	"github.com/postpilot/agent/internal/naver"
	"github.com/postpilot/agent/internal/postpilot"
	"github.com/postpilot/agent/internal/publishing"
	"github.com/postpilot/agent/internal/retirement"
	"github.com/postpilot/agent/internal/singleton"
	"github.com/postpilot/agent/internal/workdir"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "postpilot-agent:", err)
		os.Exit(1)
	}
}

func run() error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	command := "retire"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "retire", "uninstall":
		return runRetirement(paths, os.Args[2:], os.Stdout)
	case "setup", "run", "install", "diagnostics":
		return fmt.Errorf("automatic publishing has been retired; run `postpilot-agent retire` to inspect local cleanup, then add `--apply` to perform it")
	default:
		return fmt.Errorf("unknown command %q (retire)", command)
	}
}

func runRetirement(paths config.Paths, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("retire", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "stop the companion and remove owned local state")
	deleteProfiles := flags.Bool("delete-profiles", false, "also remove the exact owned browser profile paths shown by inspection")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("retire options: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("retire takes no positional arguments")
	}
	plist, err := launchd.PlistPath()
	if err != nil {
		return err
	}
	runner := retirement.Runner{
		Paths:       paths,
		BinaryPath:  filepath.Join(paths.Root, "bin", "postpilot-agent"),
		PlistPath:   plist,
		ReceiptPath: retirement.DefaultReceiptPath(paths),
		Credentials: credentials.Keychain{},
		Stop:        launchd.StopAndVerify,
	}
	result, runErr := runner.Run(context.Background(), retirement.Options{Apply: *apply, DeleteProfiles: *deleteProfiles})
	fmt.Fprintf(output, "Postpilot automatic publishing retirement: %s\n", result.Receipt.Status)
	fmt.Fprintf(output, "Stored credential accounts: %d; owned local paths: %d\n", len(result.Receipt.CredentialAccounts), len(result.Receipt.OwnedPaths))
	for _, profile := range result.Receipt.ProfilePaths {
		action := "preserve"
		if *deleteProfiles {
			action = "delete"
		}
		fmt.Fprintf(output, "Browser profile (%s): %s\n", action, profile.Path)
	}
	for _, warning := range result.Receipt.ProfileWarnings {
		fmt.Fprintf(output, "Browser profile preserved (ownership unresolved): %s\n", warning)
	}
	for _, failure := range result.Receipt.Failures {
		fmt.Fprintf(output, "Unresolved inventory or cleanup item: %s\n", failure)
	}
	if !*apply {
		fmt.Fprintln(output, "Inspection only; nothing was changed. Re-run with `retire --apply` to stop and remove the companion.")
		if len(result.Receipt.ProfilePaths) > 0 {
			fmt.Fprintln(output, "Profiles remain by default. Use `retire --apply --delete-profiles` only after reviewing the exact paths above.")
		}
	} else {
		fmt.Fprintf(output, "Local shutdown receipt: %s\n", result.ReceiptPath)
	}
	return runErr
}

type publisherFactory func(config.Connection) (publishing.Publisher, error)

// naverPort is the driver surface one job needs: the commit-fence port plus the close that
// releases its CDP connection.
type naverPort interface {
	naver.CommitPort
	Close() error
}

// publisherBoundaries are the three edges the real publisher is built from — the browser,
// Naver's own account resolution, and the driver release this build carries. They are fields
// so the wiring can be exercised without launching a browser (PUB-29).
type publisherBoundaries struct {
	openEditor func(binary, profileDir string) (*browser.Session, error)
	identity   func(ctx context.Context, cdpURL string) (browser.NaverIdentity, error)
	bindPort   func(ctx context.Context, cdpURL string) (naverPort, error)
	signature  func() (string, error)
}

func defaultPublisherBoundaries() publisherBoundaries {
	return publisherBoundaries{
		openEditor: browser.OpenEditor,
		identity:   browser.ObserveNaverIdentity,
		bindPort: func(ctx context.Context, cdpURL string) (naverPort, error) {
			return naver.NewCDPPort(ctx, cdpURL)
		},
		signature: func() (string, error) {
			manifest, err := naver.Manifest()
			if err != nil {
				return "", err
			}
			return manifest.SignatureID, nil
		},
	}
}

// newPublisher gives one paired connection its own deterministic publisher. Nothing is
// launched here: a browser is opened per JOB, because the commit port's activation latch
// (PUB-15) may never be carried from one run to the next, and PUB-18 wants a browser this
// daemon launched closed again after the run it was launched for.
func newPublisher(connection config.Connection) (publishing.Publisher, error) {
	if err := config.ValidateConnection(connection); err != nil {
		return nil, err
	}
	if _, ok := browser.Supported(connection.BrowserBinary); !ok {
		return nil, fmt.Errorf("connection %s: browser %q is not a supported Chromium family binary", connection.ID, connection.BrowserBinary)
	}
	return connectionPublisher{connection: connection, boundaries: defaultPublisherBoundaries()}, nil
}

// connectionPublisher turns one job directory into one publication attempt for one paired
// account. Every failure it can see itself becomes a typed terminal result; everything it
// cannot is left to the deterministic publisher, which is the only thing that classifies the
// editor (PUB-16).
type connectionPublisher struct {
	connection config.Connection
	boundaries publisherBoundaries
}

var _ publishing.Publisher = connectionPublisher{}

func terminalFailure(kind naver.FailureKind) publishing.Result {
	return publishing.Result{Status: "failed", FailureKind: string(kind)}
}

func (c connectionPublisher) Run(ctx context.Context, dir string, reporter publishing.Reporter) (publishing.Result, error) {
	// The driver release is checked against what this connection's own probe recorded, before
	// a browser is touched: PUB-19 keeps a connection unready until a fresh probe agrees with
	// the release that is actually running, and until then a job fails closed rather than
	// driving an editor this build has not verified.
	signature, err := c.boundaries.signature()
	if err != nil || signature == "" || signature != c.connection.CompatibilitySignature {
		return terminalFailure(naver.FailureEditorChanged), nil
	}
	session, err := c.boundaries.openEditor(c.connection.BrowserBinary, c.connection.ProfileDir)
	if err != nil {
		return terminalFailure(naver.FailureBrowserLost), nil
	}
	// Close releases only a browser THIS process launched; a setup or login browser the owner
	// left open is reused and never killed (PUB-18).
	defer func() { _ = session.Close() }()

	// This is also the navigation: Naver's signed-in session resolves the id-less writer
	// entry to the account's own blog, and this reads the id it chose. A profile whose login
	// has expired cannot reach its own writer, which is exactly what this failure means.
	identity, err := c.boundaries.identity(ctx, session.CDPURL)
	if err != nil {
		return terminalFailure(naver.FailureLoginExpired), nil
	}
	if identity.BlogID != c.connection.PlatformAccountID {
		return terminalFailure(naver.FailureAccountMismatch), nil
	}

	port, err := c.boundaries.bindPort(ctx, session.CDPURL)
	if err != nil {
		return terminalFailure(naver.FailureBrowserLost), nil
	}
	defer func() { _ = port.Close() }()
	return naver.Publisher{Port: port}.Run(ctx, dir, reporter)
}

const connectionReloadInterval = 2 * time.Second

// daemonDeps are the adapters one paired account's supervisor is built from. They are
// grouped so the wiring can be exercised with fakes instead of only through a live daemon.
type daemonDeps struct {
	paths        config.Paths
	keychain     credentials.Store
	newPublisher publisherFactory
	permit       chan struct{}
	logger       *slog.Logger
}

// buildSupervisor gives one paired account its own Keychain credential, API client,
// publisher and job directory. Two accounts on the same Mac share only the cross-account
// execution permit, so neither can see the other's jobs or publish through the other's
// Naver identity (PUBLISH-23).
func buildSupervisor(ctx context.Context, connection config.Connection, deps daemonDeps) (publishing.Supervisor, error) {
	if err := config.ValidateConnection(connection); err != nil {
		return publishing.Supervisor{}, fmt.Errorf("connection is not armed: %w", err)
	}
	token, err := deps.keychain.Get(ctx, connection.KeychainAccount)
	if err != nil {
		return publishing.Supervisor{}, errors.New("Keychain credential unavailable")
	}
	client := postpilot.New(connection.APIURL, token)
	publisher, err := deps.newPublisher(connection)
	if err != nil {
		return publishing.Supervisor{}, fmt.Errorf("deterministic publisher unavailable: %w", err)
	}
	logger := deps.logger.With("connection_id", connection.ID)
	executor := publishing.Executor{API: client, Publisher: publisher, JobsRoot: deps.paths.Jobs, ConnectionID: connection.ID, HeartbeatEvery: config.Heartbeat, Timeout: config.JobTimeout, Logger: logger}
	return publishing.Supervisor{Client: client, Executor: executor, PollInterval: config.PollInterval, Logger: logger, Permit: deps.permit}, nil
}

func runAgents(paths config.Paths, newPublisher publisherFactory) error {
	return runAgentsWith(paths, newPublisher, credentials.Keychain{})
}

func runAgentsWith(paths config.Paths, newPublisher publisherFactory, keychain credentials.Store) error {
	if newPublisher == nil {
		// The factory is injected so the daemon can be exercised with a fake. A nil one is
		// a wiring fault, not a state the daemon should try to run in.
		return errors.New("no publisher factory was wired into the daemon")
	}
	processLock, err := singleton.Acquire(filepath.Join(paths.Root, "run.lock"))
	if err != nil {
		return err
	}
	defer processLock.Close()
	if err := workdir.Cleanup(paths.Jobs); err != nil {
		return fmt.Errorf("clean abandoned publishing jobs: %w", err)
	}
	cfg, err := config.Load(paths)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	type stopped struct {
		connectionID string
		err          error
	}
	errCh := make(chan stopped)
	executionPermit := make(chan struct{}, 1)
	executionPermit <- struct{}{}
	launched := make(map[string]struct{})
	running := 0
	deps := daemonDeps{paths: paths, keychain: keychain, newPublisher: newPublisher, permit: executionPermit, logger: logger}
	startNew := func(loaded config.File) {
		for _, connection := range unseenArmedConnections(loaded, launched) {
			supervisor, err := buildSupervisor(ctx, connection, deps)
			if err != nil {
				// One account failing to start must never stop the other's supervisor.
				logger.Error("connection supervisor not started", "connection_id", connection.ID, "error", err)
				continue
			}
			launched[connection.ID] = struct{}{}
			running++
			go func(connectionID string) { errCh <- stopped{connectionID: connectionID, err: supervisor.Run(ctx)} }(connection.ID)
		}
	}
	startNew(cfg)
	if running == 0 {
		return errors.New("no armed publishing connection; run setup first")
	}
	reload := time.NewTicker(connectionReloadInterval)
	defer reload.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-reload.C:
			connections, err := reloadArmedConnections(paths, launched)
			if err != nil {
				logger.Warn("publishing connection reload failed", "error", err)
				continue
			}
			startNew(config.File{Connections: connections})
		case result := <-errCh:
			running--
			logger.Error("connection supervisor stopped", "connection_id", result.connectionID, "error", result.err)
		}
	}
}

func reloadArmedConnections(paths config.Paths, launched map[string]struct{}) ([]config.Connection, error) {
	loaded, err := config.Load(paths)
	if err != nil {
		return nil, err
	}
	return unseenArmedConnections(loaded, launched), nil
}

func unseenArmedConnections(cfg config.File, launched map[string]struct{}) []config.Connection {
	connections := make([]config.Connection, 0)
	for _, connection := range cfg.Connections {
		if !connection.Armed {
			continue
		}
		if _, exists := launched[connection.ID]; exists {
			continue
		}
		connections = append(connections, connection)
	}
	return connections
}

func diagnostics(paths config.Paths) error {
	return diagnosticsWith(paths, credentials.Keychain{}, browser.Start, naver.Probe, os.Stdout)
}

func diagnosticsWith(paths config.Paths, keychain credentials.Store, start func(string, string, string) (*browser.Session, error), probe func(context.Context, string) (naver.Result, error), output io.Writer) error {
	cfg, err := config.Load(paths)
	if err != nil {
		return err
	}
	for _, connection := range cfg.Connections {
		if err := config.ValidateConnection(connection); err != nil {
			return fmt.Errorf("connection %s: %w", connection.ID, err)
		}
		if _, err := keychain.Get(context.Background(), connection.KeychainAccount); err != nil {
			return fmt.Errorf("connection %s: Keychain credential unavailable", connection.ID)
		}
		browserSession, err := start(connection.BrowserBinary, connection.ProfileDir, "")
		if err != nil {
			return fmt.Errorf("connection %s: browser CDP unavailable: %w", connection.ID, err)
		}
		result, err := probe(context.Background(), browserSession.CDPURL)
		_ = browserSession.Close()
		if err != nil || result.Identity.BlogID != connection.PlatformAccountID {
			return fmt.Errorf("connection %s: Naver compatibility probe failed", connection.ID)
		}
		fmt.Fprintf(output, "%s: ready (%s, driver %s)\n", connection.Label, result.BrowserVersion, result.SignatureID)
	}
	return nil
}
