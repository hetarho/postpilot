package hermes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const nousInferenceBaseURL = "https://inference-api.nousresearch.com/v1"

var hermesModelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}/[A-Za-z0-9][A-Za-z0-9._:-]{0,191}$`)

// EnsureProfileInference makes a blank Postpilot profile reuse an existing
// local Nous Portal login through Hermes' official shared-credential import.
// It never reads or copies OAuth tokens itself. A first-ever Hermes login still
// requires the user-facing `setup --portal` flow.
func EnsureProfileInference(ctx context.Context, binary, profile string) error {
	if binary == "" {
		binary = "hermes"
	}
	provider, err := profileConfigValue(ctx, binary, profile, "model.provider")
	if err != nil {
		return fmt.Errorf("read Hermes inference provider: %w", err)
	}
	if provider != "" && provider != "auto" && provider != "nous" {
		// Non-Nous providers may use profile-specific API keys or OAuth stores.
		// Preserve an explicitly configured provider instead of rewriting it.
		return nil
	}
	if provider == "nous" {
		if err := ensureSharedNousAuth(ctx, binary, profile); err != nil {
			return err
		}
		return nil
	}

	model, found, err := findNousModelSeed(ctx, binary, profile)
	if err != nil {
		return err
	}
	if !found {
		return inferenceSetupRequired(profile)
	}
	if err := ensureSharedNousAuth(ctx, binary, profile); err != nil {
		return err
	}
	settings := []struct{ key, value string }{
		{key: "model.default", value: model},
		{key: "model.base_url", value: nousInferenceBaseURL},
		// Activate the provider last so a failed earlier write never leaves a
		// half-configured Nous runtime selected.
		{key: "model.provider", value: "nous"},
	}
	for _, setting := range settings {
		key, value := setting.key, setting.value
		command := exec.CommandContext(ctx, binary, "-p", profile, "config", "set", key, value)
		if output, commandErr := command.CombinedOutput(); commandErr != nil {
			return fmt.Errorf("configure Hermes inference %s: %s", key, redact(string(output)))
		}
	}
	return nil
}

func ensureSharedNousAuth(ctx context.Context, binary, profile string) error {
	if nousLoggedIn(ctx, binary, profile) {
		return nil
	}
	available, err := sharedNousAuthAvailable(profile)
	if err != nil || !available {
		return inferenceSetupRequired(profile)
	}
	command := exec.CommandContext(
		ctx, binary, "-p", profile, "auth", "add", "nous", "--type", "oauth",
		"--no-browser", "--timeout", "10",
	)
	// Hermes prompts before importing its owner-only shared Portal credential.
	// This input accepts only that official local import; --no-browser prevents
	// this background setup path from starting a new OAuth login.
	command.Stdin = strings.NewReader("y\n")
	if output, commandErr := command.CombinedOutput(); commandErr != nil {
		return fmt.Errorf("import shared Hermes Portal login: %s", redact(string(output)))
	}
	if !nousLoggedIn(ctx, binary, profile) {
		return inferenceSetupRequired(profile)
	}
	return nil
}

func nousLoggedIn(ctx context.Context, binary, profile string) bool {
	command := exec.CommandContext(ctx, binary, "-p", profile, "auth", "status", "nous")
	output, err := command.CombinedOutput()
	return err == nil && strings.Contains(strings.ToLower(string(output)), "nous: logged in")
}

func sharedNousAuthAvailable(profile string) (bool, error) {
	profileDir, err := hermesProfileDir(profile)
	if err != nil {
		return false, err
	}
	sharedDir := strings.TrimSpace(os.Getenv("HERMES_SHARED_AUTH_DIR"))
	if sharedDir == "" {
		sharedDir = filepath.Join(filepath.Dir(filepath.Dir(profileDir)), "shared")
	}
	info, err := os.Stat(filepath.Join(sharedDir, "nous_auth.json"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return false, errors.New("shared Hermes Portal login is not an owner-only regular file")
	}
	return true, nil
}

func findNousModelSeed(ctx context.Context, binary, targetProfile string) (string, bool, error) {
	profileDir, err := hermesProfileDir(targetProfile)
	if err != nil {
		return "", false, err
	}
	candidates := []string{"default"}
	entries, err := os.ReadDir(filepath.Dir(profileDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("list Hermes profiles: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && name != targetProfile && strings.HasPrefix(name, "postpilot-") {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates[1:])
	for _, candidate := range candidates {
		provider, providerErr := profileConfigValue(ctx, binary, candidate, "model.provider")
		if providerErr != nil || provider != "nous" {
			continue
		}
		model, modelErr := profileConfigValue(ctx, binary, candidate, "model.default")
		if modelErr == nil && hermesModelID.MatchString(model) {
			return model, true, nil
		}
	}
	return "", false, nil
}

func profileConfigValue(ctx context.Context, binary, profile, key string) (string, error) {
	command := exec.CommandContext(ctx, binary, "-p", profile, "config", "get", key)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", redact(string(output)))
	}
	value := strings.TrimSpace(string(output))
	if strings.ContainsAny(value, "\r\n") || len(value) > 256 {
		return "", errors.New("Hermes returned an invalid config value")
	}
	return value, nil
}

func inferenceSetupRequired(profile string) error {
	return fmt.Errorf(
		"Hermes 모델 인증이 필요합니다. 터미널에서 `hermes -p %s setup --portal`을 한 번 실행한 뒤 다시 시도하세요",
		profile,
	)
}
