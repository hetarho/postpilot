package main

import (
	"context"
	"errors"
	"fmt"
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
		return (setup.Server{Paths: paths, Keychain: credentials.Keychain{}}).Run(ctx)
	case "run":
		return runAgents(paths, nil)
	case "install":
		return installUnavailable()
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

func installUnavailable() error {
	return errors.New("deterministic Naver publisher is not implemented; LaunchAgent installation is disabled until Job 25 passes its release gates")
}

type publisherFactory func(config.Connection) (publishing.Publisher, error)

func runAgents(paths config.Paths, newPublisher publisherFactory) error {
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
	keychain := credentials.Keychain{}
	type stopped struct {
		connectionID string
		err          error
	}
	errCh := make(chan stopped)
	executionPermit := make(chan struct{}, 1)
	executionPermit <- struct{}{}
	launched := make(map[string]struct{})
	running := 0
	startNew := func(loaded config.File) {
		for _, connection := range unseenArmedConnections(loaded, launched) {
			if err := config.ValidateConnection(connection); err != nil {
				logger.Error("connection is not armed", "connection_id", connection.ID, "error", err)
				continue
			}
			token, err := keychain.Get(ctx, connection.KeychainAccount)
			if err != nil {
				logger.Error("Keychain credential unavailable", "connection_id", connection.ID)
				continue
			}
			client := postpilot.New(connection.APIURL, token)
			publisher, err := newPublisher(connection)
			if err != nil {
				logger.Error("deterministic publisher unavailable", "connection_id", connection.ID, "error", err)
				continue
			}
			executor := publishing.Executor{API: client, Publisher: publisher, JobsRoot: paths.Jobs, ConnectionID: connection.ID, HeartbeatEvery: config.Heartbeat, Timeout: config.JobTimeout, Logger: logger.With("connection_id", connection.ID)}
			supervisor := publishing.Supervisor{Client: client, Executor: executor, PollInterval: config.PollInterval, Logger: logger.With("connection_id", connection.ID), Permit: executionPermit}
			launched[connection.ID] = struct{}{}
			running++
			go func(connectionID string) { errCh <- stopped{connectionID: connectionID, err: supervisor.Run(ctx)} }(connection.ID)
		}
	}
	startNew(cfg)
	if running == 0 {
		return errors.New("no armed publishing connection; run setup first")
	}
	reload := time.NewTicker(2 * time.Second)
	defer reload.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-reload.C:
			loaded, err := config.Load(paths)
			if err != nil {
				logger.Warn("publishing connection reload failed", "error", err)
				continue
			}
			startNew(loaded)
		case result := <-errCh:
			running--
			logger.Error("connection supervisor stopped", "connection_id", result.connectionID, "error", result.err)
		}
	}
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
	cfg, err := config.Load(paths)
	if err != nil {
		return err
	}
	keychain := credentials.Keychain{}
	for _, connection := range cfg.Connections {
		if err := config.ValidateConnection(connection); err != nil {
			return fmt.Errorf("connection %s: %w", connection.ID, err)
		}
		if _, err := keychain.Get(context.Background(), connection.KeychainAccount); err != nil {
			return fmt.Errorf("connection %s: Keychain credential unavailable", connection.ID)
		}
		if _, err := os.Stat(connection.BrowserBinary); err != nil {
			return fmt.Errorf("connection %s: browser unavailable", connection.ID)
		}
		browserSession, err := browser.Start(connection.BrowserBinary, connection.ProfileDir, "")
		if err != nil {
			return fmt.Errorf("connection %s: browser CDP unavailable: %w", connection.ID, err)
		}
		_ = browserSession.Close()
		fmt.Printf("%s: transport ready (token and browser present)\n", connection.Label)
	}
	return errors.New("deterministic Naver publisher compatibility probe is not implemented; Job 25 remains unarmed")
}
