package hermes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureProfileInferenceImportsSharedNousAndInheritsModel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HERMES_HOME", root)
	commandLog := filepath.Join(root, "commands.log")
	authMarker := filepath.Join(root, "auth-imported")
	t.Setenv("HERMES_TEST_COMMAND_LOG", commandLog)
	t.Setenv("HERMES_TEST_AUTH_MARKER", authMarker)
	for _, name := range []string{"postpilot-target", "postpilot-source"} {
		if err := os.MkdirAll(filepath.Join(root, "profiles", name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "shared"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "shared", "nous_auth.json"), []byte("shared"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "hermes")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$HERMES_TEST_COMMAND_LOG"
case "$*" in
  "-p postpilot-target config get model.provider") printf '%s\n' auto ;;
  "-p default config get model.provider") printf '%s\n' auto ;;
  "-p postpilot-source config get model.provider") printf '%s\n' nous ;;
  "-p postpilot-source config get model.default") printf '%s\n' upstage/solar-pro4:free ;;
  "-p postpilot-target auth status nous")
    if [ -f "$HERMES_TEST_AUTH_MARKER" ]; then printf '%s\n' 'nous: logged in'; else printf '%s\n' 'nous: logged out'; fi ;;
  "-p postpilot-target auth add nous --type oauth --no-browser --timeout 10")
    read answer
    [ "$answer" = y ] || exit 2
    : > "$HERMES_TEST_AUTH_MARKER" ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := EnsureProfileInference(context.Background(), binary, "postpilot-target"); err != nil {
		t.Fatal(err)
	}
	commands, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"auth add nous --type oauth --no-browser --timeout 10",
		"config set model.provider nous",
		"config set model.default upstage/solar-pro4:free",
		"config set model.base_url " + nousInferenceBaseURL,
	} {
		if !strings.Contains(string(commands), want) {
			t.Fatalf("commands = %q, want %q", commands, want)
		}
	}
}

func TestEnsureProfileInferenceExplainsFirstPortalSetup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HERMES_HOME", root)
	if err := os.MkdirAll(filepath.Join(root, "profiles", "postpilot-target"), 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "hermes")
	if err := os.WriteFile(binary, []byte(`#!/bin/sh
case "$*" in
  *"config get model.provider") printf '%s\n' auto ;;
esac
`), 0o700); err != nil {
		t.Fatal(err)
	}

	err := EnsureProfileInference(context.Background(), binary, "postpilot-target")
	if err == nil || !strings.Contains(err.Error(), "hermes -p postpilot-target setup --portal") {
		t.Fatalf("error = %v", err)
	}
}
