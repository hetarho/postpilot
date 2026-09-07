package launchd

import (
	"bytes"
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

// PUBLISH-29 asks for a reproducible install, so the same binary and log directory must
// produce byte-identical plists: an install that rewrites the file differently each time
// cannot be diffed against what is actually loaded.
func TestWritePlistIsByteIdenticalForTheSameInputs(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.plist")
	second := filepath.Join(dir, "second.plist")
	for _, path := range []string{first, second} {
		if err := writePlist(path, "/opt/postpilot/postpilot-agent", "/opt/postpilot/logs"); err != nil {
			t.Fatal(err)
		}
	}
	left, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left, right) {
		t.Fatalf("plist is not reproducible:\n%s\n%s", left, right)
	}
}

// PUBLISH-29: uninstall never removes a browser profile or a Keychain credential without a
// separate explicit confirmation, so the Go half must touch nothing but its own plist.
func TestUninstallRemovesOnlyItsOwnPlist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Never unload the real agent: on the owner's own Mac this test would otherwise stop
	// the daemon that is actually publishing.
	var unloaded []string
	original := launchctlBootout
	launchctlBootout = func(target string) { unloaded = append(unloaded, target) }
	t.Cleanup(func() { launchctlBootout = original })
	agents := filepath.Join(home, "Library", "LaunchAgents")
	support := filepath.Join(home, "Library", "Application Support", "Postpilot Agent")
	profile := filepath.Join(support, "profiles", "connection-1")
	for _, dir := range []string{agents, profile} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	kept := map[string]string{
		filepath.Join(support, "config.json"):        `{"connections":[]}`,
		filepath.Join(profile, "Cookies"):            "naver session",
		filepath.Join(agents, "com.other.app.plist"): "someone else's agent",
	}
	for path, contents := range kept {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	plist, err := PlistPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writePlist(plist, "/opt/postpilot/postpilot-agent", filepath.Join(support, "logs")); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Fatalf("plist survived uninstall: %v", err)
	}
	for path, contents := range kept {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != contents {
			t.Fatalf("uninstall disturbed %s: %q %v", path, got, err)
		}
	}
	if len(unloaded) != 1 || unloaded[0] != domainTarget() {
		t.Fatalf("unloaded = %v, want exactly the Postpilot agent's own domain target", unloaded)
	}
	// Removing an agent that was never installed is how a partial install is cleaned up,
	// so a second uninstall must stay silent rather than fail.
	if err := Uninstall(); err != nil {
		t.Fatalf("second uninstall = %v", err)
	}
}
