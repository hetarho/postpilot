// Package config loads process configuration from the environment. It is the only
// place that reads os.Getenv — everything else takes a *Config.
package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config is the fully-resolved process configuration.
type Config struct {
	// Port the HTTP server listens on.
	Port string
	// CORSOrigin is the single browser origin allowed to call the API. In production
	// this is the frontend origin for THIS environment; locally it is the Vite dev server.
	CORSOrigin string
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

	return &Config{
		Port:       getenv("PORT", "8080"),
		CORSOrigin: getenv("CORS_ORIGIN", "http://localhost:2564"),
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
