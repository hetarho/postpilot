package main

import (
	"strings"
	"testing"

	"github.com/postpilot/agent/internal/config"
)

func TestRunAgentsRefusesBeforeClaimWithoutDeterministicPublisher(t *testing.T) {
	err := runAgents(config.Paths{}, nil)
	if err == nil || !strings.Contains(err.Error(), "deterministic Naver publisher is not implemented") {
		t.Fatalf("runAgents error = %v", err)
	}
}

func TestInstallIsDisabledWithoutDeterministicPublisher(t *testing.T) {
	err := installUnavailable()
	if err == nil || !strings.Contains(err.Error(), "LaunchAgent installation is disabled") {
		t.Fatalf("install error = %v", err)
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
