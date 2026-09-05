package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultAPIURL = "https://api.postpilot.haeram.me"
	// PollInterval is repeated as publishing.Supervisor.Run's fallback default; change
	// both together. The server absorbs the resulting call rate with its own heartbeat
	// throttle (PUBLISH_AGENT_HEARTBEAT_INTERVAL), so lowering this multiplies requests
	// without multiplying writes.
	PollInterval = 5 * time.Second
	Heartbeat    = 10 * time.Second
	JobTimeout   = 15 * time.Minute
)

type Connection struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	APIURL          string `json:"api_url"`
	AgentID         string `json:"agent_id"`
	KeychainAccount string `json:"keychain_account"`
	BrowserBinary   string `json:"browser_binary"`
	BrowserLabel    string `json:"browser_label"`
	ProfileDir      string `json:"profile_dir"`
	WorkDir         string `json:"work_dir,omitempty"`
	// DraftID is present while enrollment/profile sync is incomplete. It is removed with
	// the local draft only when activation succeeds; older armed connections decode empty.
	DraftID         string `json:"draft_id,omitempty"`
	LeaseTTLSeconds int64  `json:"lease_ttl_seconds"`
	Armed           bool   `json:"armed"`
}

// ConnectionDraft is Mac-owned browser state before the server has issued an agent id or
// token. Its random identity, profile and work paths survive every device-code replacement.
type ConnectionDraft struct {
	ID            string    `json:"id"`
	Label         string    `json:"label"`
	APIURL        string    `json:"api_url"`
	BrowserBinary string    `json:"browser_binary"`
	BrowserLabel  string    `json:"browser_label"`
	ProfileDir    string    `json:"profile_dir"`
	WorkDir       string    `json:"work_dir"`
	CreatedAt     time.Time `json:"created_at"`
}

type File struct {
	Connections []Connection      `json:"connections"`
	Drafts      []ConnectionDraft `json:"drafts,omitempty"`
}

type Paths struct {
	Root       string
	ConfigFile string
	Profiles   string
	Jobs       string
	Logs       string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	root := filepath.Join(home, "Library", "Application Support", "Postpilot Agent")
	return Paths{Root: root, ConfigFile: filepath.Join(root, "config.json"), Profiles: filepath.Join(root, "browser-profiles"), Jobs: filepath.Join(root, "jobs"), Logs: filepath.Join(root, "logs")}, nil
}

func Ensure(paths Paths) error {
	for _, dir := range []string{paths.Root, paths.Profiles, paths.Jobs, paths.Logs} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func Load(paths Paths) (File, error) {
	data, err := os.ReadFile(paths.ConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, err
	}
	var cfg File
	if err := json.Unmarshal(data, &cfg); err != nil {
		return File{}, fmt.Errorf("decode config: %w", err)
	}
	if err := ValidateFile(cfg); err != nil {
		return File{}, fmt.Errorf("validate config: %w", err)
	}
	if err := validateOwnedDraftPaths(paths, cfg); err != nil {
		return File{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func Save(paths Paths, cfg File) error {
	if err := ValidateFile(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	if err := validateOwnedDraftPaths(paths, cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	if err := Ensure(paths); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := paths.ConfigFile + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, paths.ConfigFile)
}

func ValidateConnection(connection Connection) error {
	if err := ValidateAPIURL(connection.APIURL); err != nil {
		return err
	}
	if connection.ID == "" || connection.AgentID == "" || connection.KeychainAccount == "" || connection.BrowserBinary == "" || connection.ProfileDir == "" {
		return errors.New("connection is incomplete")
	}
	return ValidateLeaseTTL(connection.LeaseTTLSeconds)
}

func ValidateDraft(draft ConnectionDraft) error {
	if err := ValidateAPIURL(draft.APIURL); err != nil {
		return err
	}
	decodedID, idErr := hex.DecodeString(draft.ID)
	if idErr != nil || len(decodedID) != 16 || draft.ID != strings.ToLower(draft.ID) || strings.TrimSpace(draft.Label) == "" ||
		draft.BrowserBinary == "" || draft.BrowserLabel == "" || !filepath.IsAbs(draft.ProfileDir) ||
		!filepath.IsAbs(draft.WorkDir) || draft.CreatedAt.IsZero() {
		return errors.New("connection draft is incomplete")
	}
	return nil
}

func validateOwnedDraftPaths(paths Paths, cfg File) error {
	for _, draft := range cfg.Drafts {
		if filepath.Clean(draft.ProfileDir) != filepath.Join(paths.Profiles, draft.ID) ||
			filepath.Clean(draft.WorkDir) != filepath.Join(paths.Jobs, draft.ID) {
			return fmt.Errorf("draft %q has paths outside its owned directories", draft.ID)
		}
	}
	for _, connection := range cfg.Connections {
		if connection.WorkDir == "" {
			continue
		}
		if filepath.Clean(connection.WorkDir) != filepath.Join(paths.Jobs, connection.ID) ||
			filepath.Clean(connection.ProfileDir) != filepath.Join(paths.Profiles, connection.ID) {
			return fmt.Errorf("connection %q has paths outside its owned directories", connection.ID)
		}
	}
	return nil
}

// ValidateFile makes identity and path selection fail closed. A draft may share its paths
// only with the one pending connection explicitly bound by DraftID; nothing is inferred.
func ValidateFile(cfg File) error {
	ids := map[string]string{}
	profiles := map[string]string{}
	works := map[string]string{}
	drafts := map[string]ConnectionDraft{}
	for _, draft := range cfg.Drafts {
		if err := ValidateDraft(draft); err != nil {
			return fmt.Errorf("draft %q: %w", draft.ID, err)
		}
		if owner, exists := ids[draft.ID]; exists {
			return fmt.Errorf("duplicate local identity %q (%s and draft)", draft.ID, owner)
		}
		ids[draft.ID], drafts[draft.ID] = "draft", draft
		if owner, exists := profiles[draft.ProfileDir]; exists {
			return fmt.Errorf("profile path conflict between %s and draft %q", owner, draft.ID)
		}
		profiles[draft.ProfileDir] = "draft " + draft.ID
		if owner, exists := works[draft.WorkDir]; exists {
			return fmt.Errorf("work path conflict between %s and draft %q", owner, draft.ID)
		}
		works[draft.WorkDir] = "draft " + draft.ID
	}
	for _, connection := range cfg.Connections {
		if strings.TrimSpace(connection.ID) == "" {
			return errors.New("connection has no identity")
		}
		if owner, exists := ids[connection.ID]; exists && !(owner == "draft" && connection.DraftID == connection.ID) {
			return fmt.Errorf("duplicate local identity %q (%s and connection)", connection.ID, owner)
		}
		ids[connection.ID] = "connection"
		bound, isBound := drafts[connection.DraftID]
		if connection.DraftID != "" && !isBound {
			return fmt.Errorf("connection %q references missing draft %q", connection.ID, connection.DraftID)
		}
		if owner, exists := profiles[connection.ProfileDir]; exists && (!isBound || bound.ProfileDir != connection.ProfileDir) {
			return fmt.Errorf("profile path conflict between %s and connection %q", owner, connection.ID)
		}
		if connection.ProfileDir != "" {
			profiles[connection.ProfileDir] = "connection " + connection.ID
		}
		if connection.WorkDir != "" {
			if owner, exists := works[connection.WorkDir]; exists && (!isBound || bound.WorkDir != connection.WorkDir) {
				return fmt.Errorf("work path conflict between %s and connection %q", owner, connection.ID)
			}
			works[connection.WorkDir] = "connection " + connection.ID
		}
	}
	return nil
}

func ValidateLeaseTTL(seconds int64) error {
	if seconds <= 0 || Heartbeat*2 >= time.Duration(seconds)*time.Second {
		return errors.New("server lease TTL is too short for the configured heartbeat")
	}
	return nil
}

func ValidateAPIURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && isLoopback(u.Hostname()))) {
		return errors.New("API URL must use HTTPS (HTTP is allowed only for loopback development)")
	}
	return nil
}

func isLoopback(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
