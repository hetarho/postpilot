// Package config loads process configuration from the environment. It is the only
// place that reads os.Getenv — everything else takes a *Config.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// sessionTTL is how long a session stays valid after login. Fixed at 30 days by the
// PRD (F-1), so it is a constant rather than an env var — an operator who changes it
// per environment changes a product rule, not a deployment detail.
const sessionTTL = 720 * time.Hour

// Config is the fully-resolved process configuration.
type Config struct {
	// Port the HTTP server listens on.
	Port string
	// CORSOrigin is the single browser origin allowed to call the API. In production
	// this is the frontend origin for THIS environment; locally it is the Vite dev server.
	CORSOrigin string
	// DBPath is the SQLite file. Relative paths resolve against the process working
	// directory: `/` in the production image (volume `./data:/data`) and `/app` in the
	// dev container.
	DBPath string
	// SessionTTL is how long a session is valid after login.
	SessionTTL time.Duration
}

// Load reads the environment, falling back to a repo-root .env when present so a
// bare `go run ./cmd/api` behaves like the compose service. A missing .env is not an
// error (containers get real env vars); only a malformed one is.
func Load() (*Config, error) {
	// Ignore "file not found" — godotenv returns it as a plain *PathError and there is
	// nothing to load in a container.
	if _, err := os.Stat("../.env"); err == nil {
		if err := godotenv.Load("../.env"); err != nil {
			return nil, err
		}
	}

	cfg := &Config{
		Port:       getenv("PORT", "8080"),
		CORSOrigin: getenv("CORS_ORIGIN", "http://localhost:2564"),
		DBPath:     getenv("DB_PATH", "data/postpilot.db"),
		SessionTTL: sessionTTL,
	}

	if err := validateOrigin(cfg.CORSOrigin); err != nil {
		return nil, fmt.Errorf("CORS_ORIGIN: %w", err)
	}

	return cfg, nil
}

// validateOrigin rejects anything the credentialed CORS layer cannot safely serve.
//
// Two different failures are caught here, and both are invisible until a browser tries
// them in production:
//   - A bare `*` is refused by the browser when combined with `Allow-Credentials`.
//   - Any embedded `*` (`https://*.example.com`) is a PATTERN to rs/cors, which then
//     reflects whatever subdomain asks — handing a sibling project on the same
//     registered domain the ability to make authenticated calls with the user's
//     session. That is the same leak the cookie avoids by carrying no Domain
//     attribute, so the two must agree.
func validateOrigin(origin string) error {
	if origin == "" {
		return fmt.Errorf("required (the exact browser origin, e.g. https://postpilot.example.com)")
	}
	if strings.Contains(origin, "*") {
		return fmt.Errorf("must be one exact origin — a wildcard cannot be combined with credentialed CORS, got %q", origin)
	}

	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("not a valid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("must be an absolute origin with scheme and host, got %q", origin)
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("must be scheme://host[:port] only, got %q", origin)
	}

	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
