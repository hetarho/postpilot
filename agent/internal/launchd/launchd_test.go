package launchd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePlistProducesAnOwnerOnlyReproducibleUserAgent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Label+".plist")
	binary := filepath.Join(dir, "Postpilot & Agent", "postpilot-agent")
	logs := filepath.Join(dir, "Logs & Diagnostics")
	if err := writePlist(path, binary, logs); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{Label, "<string>run</string>", "<key>RunAtLoad</key><true/>", "<key>SuccessfulExit</key><false/>", "Postpilot &amp; Agent", "Logs &amp; Diagnostics"} {
		if !strings.Contains(text, required) {
			t.Fatalf("plist lacks %q: %s", required, text)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("plist mode=%v", info.Mode().Perm())
	}
}

func TestInstallRejectsRelativeFilesystemTargetsBeforeLaunchctl(t *testing.T) {
	if err := Install("relative/agent", "/tmp/logs"); err == nil {
		t.Fatal("relative binary was accepted")
	}
	if err := Install("/tmp/agent", "relative/logs"); err == nil {
		t.Fatal("relative log directory was accepted")
	}
}
