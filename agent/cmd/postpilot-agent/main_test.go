package main

import (
	"testing"

	"github.com/postpilot/agent/internal/config"
)

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
