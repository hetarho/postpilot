package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestValidateConnectionTrustAndLease(t *testing.T) {
	valid := Connection{ID: "one", APIURL: "https://api.example.com", AgentID: "agent", KeychainAccount: "key", BrowserBinary: "/browser", ProfileDir: "/profile", LeaseTTLSeconds: 45}
	if err := ValidateConnection(valid); err != nil {
		t.Fatalf("valid connection: %v", err)
	}
	plain := valid
	plain.APIURL = "http://api.example.com"
	if err := ValidateConnection(plain); err == nil {
		t.Fatal("public plain HTTP was accepted")
	}
	loopback := valid
	loopback.APIURL = "http://127.0.0.1:8080"
	if err := ValidateConnection(loopback); err != nil {
		t.Fatalf("loopback development URL: %v", err)
	}
	short := valid
	short.LeaseTTLSeconds = 20
	if err := ValidateConnection(short); err == nil {
		t.Fatal("unsafe heartbeat/lease ratio was accepted")
	}
}

func TestValidateLeaseTTLRejectsServerValueThatCannotSustainHeartbeat(t *testing.T) {
	if err := ValidateLeaseTTL(20); err == nil {
		t.Fatal("lease equal to two heartbeats was accepted")
	}
	if err := ValidateLeaseTTL(21); err != nil {
		t.Fatalf("safe lease was rejected: %v", err)
	}
}

func TestValidateFileRejectsDraftIdentityAndPathConflicts(t *testing.T) {
	first := ConnectionDraft{ID: "00112233445566778899aabbccddeeff", Label: "one", APIURL: "https://api.example.com", BrowserBinary: "/browser", BrowserLabel: "Chrome", ProfileDir: "/profiles/one", WorkDir: "/jobs/one", CreatedAt: time.Now()}
	second := first
	second.ID = "ffeeddccbbaa99887766554433221100"

	for name, cfg := range map[string]File{
		"duplicate id":        {Drafts: []ConnectionDraft{first, first}},
		"profile conflict":    {Drafts: []ConnectionDraft{first, func() ConnectionDraft { value := second; value.ProfileDir = first.ProfileDir; return value }()}},
		"work conflict":       {Drafts: []ConnectionDraft{first, func() ConnectionDraft { value := second; value.WorkDir = first.WorkDir; return value }()}},
		"missing bound draft": {Connections: []Connection{{ID: "connection", DraftID: first.ID, ProfileDir: first.ProfileDir}}},
		"malformed draft id":  {Drafts: []ConnectionDraft{func() ConnectionDraft { value := first; value.ID = "../escape"; return value }()}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFile(cfg); err == nil {
				t.Fatalf("invalid config was accepted: %+v", cfg)
			}
		})
	}
}

func TestExistingArmedConnectionKeepsItsLegacyProfilePath(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}
	legacy := Connection{ID: "legacy-connection", Label: "기존 연결", ProfileDir: "/previous/custom/profile", Armed: true}
	if err := Save(paths, File{Connections: []Connection{legacy}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Connections) != 1 || loaded.Connections[0].ProfileDir != legacy.ProfileDir || len(loaded.Drafts) != 0 {
		t.Fatalf("legacy connection was migrated or attached: %+v", loaded)
	}
}
