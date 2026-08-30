package config

import (
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
	PollInterval = 5 * time.Second
	Heartbeat    = 10 * time.Second
	JobTimeout   = 15 * time.Minute
	MaxTurns     = 60
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
	HermesBinary    string `json:"hermes_binary"`
	HermesProfile   string `json:"hermes_profile"`
	LeaseTTLSeconds int64  `json:"lease_ttl_seconds"`
	Armed           bool   `json:"armed"`
}

type File struct {
	Connections []Connection `json:"connections"`
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
	return cfg, nil
}

func Save(paths Paths, cfg File) error {
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
	if connection.ID == "" || connection.AgentID == "" || connection.KeychainAccount == "" || connection.BrowserBinary == "" || connection.ProfileDir == "" || connection.HermesBinary == "" || connection.HermesProfile == "" {
		return errors.New("connection is incomplete")
	}
	return ValidateLeaseTTL(connection.LeaseTTLSeconds)
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
