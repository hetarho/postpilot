package hermes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const publisherPluginName = "postpilot-publisher"

// Compatibility is deliberately capability-based. latest_checked_release is
// evidence for packaging, never an authority that prevents a newer compatible release.
type Compatibility struct {
	CheckedAt            string   `json:"checked_at"`
	LatestCheckedRelease string   `json:"latest_checked_release"`
	RequiredCommands     []string `json:"required_commands"`
	RequiredPluginHooks  []string `json:"required_plugin_hooks"`
	AllowedBrowserHosts  []string `json:"allowed_browser_hosts"`
}

func Probe(ctx context.Context, binary, profile, pluginDir, cdpURL string) (string, error) {
	if cdpURL == "" {
		return "", errors.New("browser CDP URL is required for the Hermes capability probe")
	}
	if binary == "" {
		binary = "hermes"
	}
	versionCommand := exec.CommandContext(ctx, binary, "--version")
	versionCommand.Env = append(os.Environ(), "BROWSER_CDP_URL="+cdpURL)
	versionOut, err := versionCommand.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("hermes --version: %w", err)
	}
	version := strings.TrimSpace(string(versionOut))
	if !regexp.MustCompile(`\d+\.\d+\.\d+`).MatchString(version) {
		return "", errors.New("Hermes version output is not recognized")
	}
	doctorArgs := []string{"doctor"}
	if profile != "" {
		doctorArgs = []string{"-p", profile, "doctor"}
	}
	doctorCommand := exec.CommandContext(ctx, binary, doctorArgs...)
	doctorCommand.Env = append(os.Environ(), "BROWSER_CDP_URL="+cdpURL)
	if output, err := doctorCommand.CombinedOutput(); err != nil {
		return "", fmt.Errorf("hermes doctor: %s", redact(string(output)))
	}
	pluginCommand := exec.CommandContext(ctx, binary, "plugins", "doctor", pluginDir, "--ci")
	pluginCommand.Env = append(os.Environ(), "BROWSER_CDP_URL="+cdpURL)
	if output, err := pluginCommand.CombinedOutput(); err != nil {
		return "", fmt.Errorf("publisher plugin probe: %s", redact(string(output)))
	}
	return version, nil
}

func InstallProfile(ctx context.Context, binary, profile, pluginDir string) error {
	if output, err := exec.CommandContext(ctx, binary, "profile", "show", profile).CombinedOutput(); err != nil {
		if output, err = exec.CommandContext(ctx, binary, "profile", "create", profile, "--no-alias", "--no-skills").CombinedOutput(); err != nil {
			return fmt.Errorf("create Hermes profile: %s", redact(string(output)))
		}
	}
	profileDir, err := hermesProfileDir(profile)
	if err != nil {
		return fmt.Errorf("resolve Hermes profile: %w", err)
	}
	if err := replaceLocalPlugin(pluginDir, filepath.Join(profileDir, "plugins", publisherPluginName)); err != nil {
		return fmt.Errorf("install publisher plugin: %w", err)
	}
	// Hermes 0.20 defaults to the Browser Use CLI backend. That mode hides the
	// guarded browser_* toolset even when BROWSER_CDP_URL points at Postpilot's
	// dedicated Chrome. The built-in backend attaches those tools to that exact
	// CDP target, which is the integration the publisher and pairing skills use.
	if output, err := exec.CommandContext(ctx, binary, "-p", profile, "config", "set", "browser.backend", "off").CombinedOutput(); err != nil {
		return fmt.Errorf("configure Hermes browser backend: %s", redact(string(output)))
	}
	// Hermes 0.20+ reserves `plugins install` for Git sources. Local plugins
	// are discovered from HERMES_HOME/plugins and then explicitly enabled.
	if output, err := exec.CommandContext(ctx, binary, "-p", profile, "plugins", "enable", publisherPluginName, "--no-allow-tool-override").CombinedOutput(); err != nil {
		return fmt.Errorf("enable publisher plugin: %s", redact(string(output)))
	}
	return nil
}

func hermesProfileDir(profile string) (string, error) {
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`).MatchString(profile) {
		return "", errors.New("invalid Hermes profile name")
	}
	root := strings.TrimSpace(os.Getenv("HERMES_HOME"))
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".hermes")
	} else if filepath.Base(filepath.Dir(filepath.Clean(root))) == "profiles" {
		// Match Hermes' own profile resolution when HERMES_HOME already points
		// at a named profile instead of the installation root.
		root = filepath.Dir(filepath.Dir(filepath.Clean(root)))
	}
	return filepath.Join(root, "profiles", profile), nil
}

func replaceLocalPlugin(source, target string) error {
	manifest := filepath.Join(source, "plugin.yaml")
	if info, err := os.Stat(manifest); err != nil || info.IsDir() {
		return errors.New("publisher plugin manifest is unavailable")
	}
	pluginsDir := filepath.Dir(target)
	if err := os.MkdirAll(pluginsDir, 0o700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(pluginsDir, ".postpilot-plugin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	stagedPlugin := filepath.Join(stage, publisherPluginName)
	if err := copyPluginTree(source, stagedPlugin); err != nil {
		return err
	}
	backup := filepath.Join(stage, "previous")
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stagedPlugin, target); err != nil {
		if _, backupErr := os.Stat(backup); backupErr == nil {
			_ = os.Rename(backup, target)
		}
		return err
	}
	return nil
}

func copyPluginTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("publisher plugin contains unsupported symlink: %s", relative)
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return closeErr
	})
}

func ReadCompatibility(data []byte) (Compatibility, error) {
	var compatibility Compatibility
	if err := json.Unmarshal(data, &compatibility); err != nil {
		return Compatibility{}, err
	}
	if len(compatibility.RequiredCommands) == 0 || len(compatibility.RequiredPluginHooks) == 0 || len(compatibility.AllowedBrowserHosts) == 0 {
		return Compatibility{}, errors.New("incomplete Hermes compatibility manifest")
	}
	return compatibility, nil
}

func redact(value string) string {
	value = regexp.MustCompile(`(?i)(token|secret|password|cookie)\s*[:=]\s*\S+`).ReplaceAllString(value, "$1=[redacted]")
	if len(value) > 1000 {
		value = value[:1000]
	}
	return strings.TrimSpace(value)
}
