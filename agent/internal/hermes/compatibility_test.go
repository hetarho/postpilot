package hermes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatibilityRequiresCapabilitiesRatherThanExactVersion(t *testing.T) {
	manifest := []byte(`{"checked_at":"2026-08-31","latest_checked_release":"0.20.6","required_commands":["doctor"],"required_plugin_hooks":["pre_tool_call"],"allowed_browser_hosts":["blog.naver.com"]}`)
	parsed, err := ReadCompatibility(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.LatestCheckedRelease != "0.20.6" {
		t.Fatalf("release = %q", parsed.LatestCheckedRelease)
	}
	if _, err := ReadCompatibility([]byte(`{"latest_checked_release":"0.20.6"}`)); err == nil {
		t.Fatal("incomplete capability manifest accepted")
	}
}

func TestInstallProfileCopiesAndEnablesLocalPlugin(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HERMES_HOME", root)
	commandLog := filepath.Join(root, "commands.log")
	t.Setenv("HERMES_TEST_COMMAND_LOG", commandLog)
	binary := filepath.Join(root, "hermes")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$HERMES_TEST_COMMAND_LOG\"\nif [ \"$1 $2\" = \"profile show\" ]; then exit 1; fi\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "publisher-source")
	if err := os.MkdirAll(filepath.Join(source, "skills", "publisher"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "plugin.yaml"), []byte("name: postpilot-publisher\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills", "publisher", "SKILL.md"), []byte("publisher"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := "postpilot-abc123"
	target := filepath.Join(root, "profiles", profile, "plugins", publisherPluginName)
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "stale.py"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := InstallProfile(context.Background(), binary, profile, source); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "publisher", "SKILL.md")); err != nil {
		t.Fatalf("copied skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "stale.py")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale plugin file was not replaced: %v", err)
	}
	commands, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	want := "-p " + profile + " plugins enable postpilot-publisher --no-allow-tool-override"
	if !strings.Contains(string(commands), want) {
		t.Fatalf("commands = %q, want %q", commands, want)
	}
	browserConfig := "-p " + profile + " config set browser.backend off"
	if !strings.Contains(string(commands), browserConfig) {
		t.Fatalf("commands = %q, want %q", commands, browserConfig)
	}
	if strings.Contains(string(commands), "plugins install") {
		t.Fatalf("local plugin was passed to Git-only installer: %q", commands)
	}
}
