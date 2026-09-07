package main

import (
	"context"
	"errors"
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
	"github.com/postpilot/agent/internal/setup"
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
	if err := config.Ensure(paths); err != nil {
		return err
	}
	command := "setup"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "setup":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return (setup.Server{Paths: paths, Keychain: credentials.Keychain{}, ProbePublisher: naver.Probe}).Run(ctx)
	case "run":
		return runAgents(paths, nil)
	case "install":
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		if err := launchd.Install(binary, paths.Logs); err != nil {
			return err
		}
		fmt.Println("Postpilot publishing LaunchAgent installed. Browser profiles and Keychain credentials remain account-isolated.")
		return nil
	case "uninstall":
		if err := launchd.Uninstall(); err != nil {
			return err
		}
		fmt.Println("LaunchAgent removed. Browser profiles and Keychain credentials were kept; remove them separately only if intended.")
		return nil
	case "diagnostics":
		return diagnostics(paths)
	default:
		return fmt.Errorf("unknown command %q (setup|run|install|uninstall|diagnostics)", command)
	}
}

type publisherFactory func(config.Connection) (publishing.Publisher, error)

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
		return errors.New("deterministic Naver publisher is not implemented; finish Job 25 before starting the LaunchAgent")
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
