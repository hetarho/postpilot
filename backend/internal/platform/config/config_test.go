package config

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

func TestVoicePersonalizationDefaultsContainNoScheduler(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.VoicePersonalization
	if got.FewShotTargetCount != 2 || got.FewShotMax != 3 || got.FewShotExcerptTargetChars != 500 || got.FewShotExcerptMaxChars != 800 || got.EmbeddingSwitchPosts != 50 || got.DiffMaxRules != 3 || got.DiffMinPatternEdits != 2 || got.RuleActivationEvidence != 3 || got.RuleRetireAfter != 180*24*time.Hour || got.ValidationPostCount != 3 || got.EndingMaxConsecutive != 2 {
		t.Fatalf("voice personalization defaults = %+v", got)
	}
	typeOf := reflect.TypeOf(got)
	for i := 0; i < typeOf.NumField(); i++ {
		name := strings.ToLower(typeOf.Field(i).Name)
		if strings.Contains(name, "interval") || strings.Contains(name, "schedule") || strings.Contains(name, "sweep") {
			t.Fatalf("scheduled personalization config is forbidden: %s", typeOf.Field(i).Name)
		}
	}
}

// TestValidateOrigin guards the one config mistake that cannot be caught later: the
// credentialed CORS layer must never be handed an origin the browser will refuse.
func TestValidateOrigin(t *testing.T) {
	valid := []string{
		"http://localhost:2564",
		"https://postpilot.haeram.me",
		"https://postpilot.example.com:8443",
	}
	for _, origin := range valid {
		if err := validateOrigin(origin); err != nil {
			t.Errorf("validateOrigin(%q) = %v, want nil", origin, err)
		}
	}

	invalid := map[string]string{
		"empty":    "",
		"wildcard": "*",
		// rs/cors reads an embedded * as a pattern and reflects any matching origin,
		// so a sibling project on the same registered domain could make authenticated
		// calls with the user's session.
		"wildcard subdomain": "https://*.example.com",
		"wildcard scheme":    "*://postpilot.example.com",
		"wildcard suffix":    "https://postpilot.example.*",
		"no scheme":          "postpilot.example.com",
		"scheme only":        "https://",
		"with path":          "https://postpilot.example.com/app",
		"with query":         "https://postpilot.example.com?x=1",
		"with fragment":      "https://postpilot.example.com#top",
	}
	for name, origin := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := validateOrigin(origin); err == nil {
				t.Errorf("validateOrigin(%q) = nil, want an error", origin)
			}
		})
	}
}

func TestLoadRejectsWildcardOrigin(t *testing.T) {
	for _, origin := range []string{"*", "https://*.example.com"} {
		t.Run(origin, func(t *testing.T) {
			t.Setenv("CORS_ORIGIN", origin)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted CORS_ORIGIN=%q", origin)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("CORS_ORIGIN", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("OBSERVE_BATCH_SIZE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPath != "data/postpilot.db" {
		t.Errorf("DBPath = %q, want data/postpilot.db", cfg.DBPath)
	}
	// 30 days, on both ends: this value becomes sessions.expires_at AND the cookie's
	// Max-Age, so a change here silently changes the login lifetime (PRD F-1).
	if got := cfg.SessionTTL.Hours(); got != 720 {
		t.Errorf("SessionTTL = %v hours, want 720", got)
	}
	if cfg.ObserveBatchSize != 4 {
		t.Errorf("ObserveBatchSize = %d, want 4", cfg.ObserveBatchSize)
	}
	if cfg.LLMMaxTokensDefault != 8192 || llmMaxTokensDefault != 8192 {
		t.Errorf("LLMMaxTokensDefault = %d / %d, want 8192", cfg.LLMMaxTokensDefault, llmMaxTokensDefault)
	}
	if cfg.LLMReasoning.Observe != llm.ReasoningLow || cfg.LLMReasoning.Write != llm.ReasoningLow ||
		cfg.LLMReasoning.Analyze != llm.ReasoningUnset {
		t.Errorf("LLMReasoning = %+v", cfg.LLMReasoning)
	}
}

func TestLoadObserveBatchSize(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	t.Setenv("OBSERVE_BATCH_SIZE", "7")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ObserveBatchSize != 7 {
		t.Fatalf("ObserveBatchSize = %d, want 7", cfg.ObserveBatchSize)
	}
	for _, bad := range []string{"nope", "0", "-1"} {
		t.Setenv("OBSERVE_BATCH_SIZE", bad)
		if _, err := Load(); err == nil {
			t.Errorf("OBSERVE_BATCH_SIZE=%q was accepted", bad)
		}
	}
}

// TestRequireObjectStorage is job 03 A10: the server must refuse to start without a
// bucket, naming what is missing.
func TestRequireObjectStorage(t *testing.T) {
	full := func() *Config {
		return &Config{
			R2Endpoint:        "http://localhost:9000",
			R2PublicEndpoint:  "http://localhost:9000",
			R2AccessKeyID:     "key",
			R2SecretAccessKey: "secret",
			R2Bucket:          "postpilot",
		}
	}

	if err := full().RequireObjectStorage(); err != nil {
		t.Fatalf("a complete config was rejected: %v", err)
	}

	missing := map[string]func(*Config){
		"R2_ENDPOINT":          func(c *Config) { c.R2Endpoint = "" },
		"R2_ACCESS_KEY_ID":     func(c *Config) { c.R2AccessKeyID = "" },
		"R2_SECRET_ACCESS_KEY": func(c *Config) { c.R2SecretAccessKey = "" },
		"R2_BUCKET":            func(c *Config) { c.R2Bucket = "" },
	}
	for name, clear := range missing {
		t.Run(name, func(t *testing.T) {
			cfg := full()
			clear(cfg)
			err := cfg.RequireObjectStorage()
			if err == nil {
				t.Fatalf("a config missing %s was accepted", name)
			}
			// The message has to name the variable, or an operator is left guessing.
			if !strings.Contains(err.Error(), name) {
				t.Errorf("message = %q, want it to name %s", err, name)
			}
		})
	}

	for _, bad := range []string{"localhost:9000", "ftp://x", "not a url", ""} {
		cfg := full()
		cfg.R2Endpoint = bad
		if err := cfg.RequireObjectStorage(); err == nil {
			t.Errorf("R2_ENDPOINT=%q was accepted", bad)
		}
	}
}

// Creating an account must not need object storage: on a fresh box that happens before
// the operator has made a bucket.
func TestLoadDoesNotRequireObjectStorage(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	t.Setenv("R2_ENDPOINT", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("R2_BUCKET", "")

	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestLoadOrphanSweepInterval(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")

	t.Setenv("ORPHAN_SWEEP_INTERVAL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OrphanSweepInterval != 24*time.Hour {
		t.Errorf("default = %v, want 24h", cfg.OrphanSweepInterval)
	}

	// A zero or negative interval would make time.NewTicker panic at boot.
	for _, bad := range []string{"nonsense", "0s", "-1h"} {
		t.Setenv("ORPHAN_SWEEP_INTERVAL", bad)
		if _, err := Load(); err == nil {
			t.Errorf("ORPHAN_SWEEP_INTERVAL=%q was accepted", bad)
		}
	}
}

func TestLoadExperimentRetentionAndSweepIntervals(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	t.Setenv("EXPERIMENT_CONTENT_RETENTION", "48h")
	t.Setenv("EXPERIMENT_SWEEP_INTERVAL", "30m")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ExperimentContentRetention != 48*time.Hour || cfg.ExperimentSweepInterval != 30*time.Minute {
		t.Fatalf("experiment durations = %v / %v", cfg.ExperimentContentRetention, cfg.ExperimentSweepInterval)
	}
	for _, name := range []string{"EXPERIMENT_CONTENT_RETENTION", "EXPERIMENT_SWEEP_INTERVAL"} {
		for _, bad := range []string{"invalid", "0s", "-1h"} {
			t.Run(name+"="+bad, func(t *testing.T) {
				t.Setenv("EXPERIMENT_CONTENT_RETENTION", "48h")
				t.Setenv("EXPERIMENT_SWEEP_INTERVAL", "30m")
				t.Setenv(name, bad)
				if _, err := Load(); err == nil {
					t.Fatalf("%s=%q was accepted", name, bad)
				}
			})
		}
	}
}

func TestLoadPublishingDefaultsAndValidation(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	for _, name := range []string{
		"PUBLISH_PAIRING_TTL", "PUBLISH_MAX_PENDING_PAIRINGS", "PUBLISH_LEASE_TTL",
		"PUBLISH_ASSET_URL_TTL", "PUBLISH_ORPHAN_SWEEP_INTERVAL", "PUBLISH_ORPHAN_MIN_AGE",
		"PUBLISH_AGENT_HEARTBEAT_INTERVAL",
	} {
		t.Setenv(name, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublishPairingTTL != 10*time.Minute || cfg.PublishMaxPendingPairings != 8 ||
		cfg.PublishLeaseTTL != 45*time.Second || cfg.PublishAssetURLTTL != 10*time.Minute ||
		cfg.PublishOrphanSweepInterval != 24*time.Hour || cfg.PublishOrphanMinAge != time.Hour ||
		cfg.PublishAgentHeartbeatInterval != 15*time.Second {
		t.Fatalf("publishing defaults = %+v", cfg)
	}
	// The frontend hides an agent after PUBLISH_AGENT_STALE_MS = 30s, and a refresh only
	// lands on the poll that follows the elapsed interval, so the budget is the heartbeat
	// plus the agent's 5s poll — not the heartbeat alone. No gate spans the three seams
	// those values live in, so the relationship is asserted here.
	const staleWindow, agentPoll = 30 * time.Second, 5 * time.Second
	if cfg.PublishAgentHeartbeatInterval+agentPoll >= staleWindow {
		t.Fatalf("heartbeat %s plus a %s poll does not fit the %s staleness window",
			cfg.PublishAgentHeartbeatInterval, agentPoll, staleWindow)
	}

	for _, name := range []string{
		"PUBLISH_PAIRING_TTL", "PUBLISH_LEASE_TTL", "PUBLISH_ASSET_URL_TTL",
		"PUBLISH_ORPHAN_SWEEP_INTERVAL", "PUBLISH_ORPHAN_MIN_AGE",
		"PUBLISH_AGENT_HEARTBEAT_INTERVAL",
	} {
		for _, bad := range []string{"invalid", "0s", "-1s"} {
			t.Run(name+"="+bad, func(t *testing.T) {
				t.Setenv(name, bad)
				if _, err := Load(); err == nil || !strings.Contains(err.Error(), name) {
					t.Fatalf("%s=%q error = %v", name, bad, err)
				}
			})
		}
	}
	for _, bad := range []string{"invalid", "0", "-1"} {
		t.Run("PUBLISH_MAX_PENDING_PAIRINGS="+bad, func(t *testing.T) {
			t.Setenv("PUBLISH_MAX_PENDING_PAIRINGS", bad)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "PUBLISH_MAX_PENDING_PAIRINGS") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadPurposeLimitDefaultsAndValidation(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	names := []string{"PURPOSE_NAME_MAX_CHARS", "PURPOSE_DESCRIPTION_MAX_CHARS", "PURPOSE_INSTRUCTIONS_MAX_CHARS"}
	for _, name := range names {
		t.Setenv(name, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PurposeNameMaxChars != 40 || cfg.PurposeDescriptionMaxChars != 200 || cfg.PurposeInstructionsMaxChars != 2000 {
		t.Fatalf("purpose limit defaults = %+v", cfg)
	}

	for _, name := range names {
		for _, bad := range []string{"invalid", "0", "-1"} {
			t.Run(name+"="+bad, func(t *testing.T) {
				t.Setenv(name, bad)
				if _, err := Load(); err == nil || !strings.Contains(err.Error(), name) {
					t.Fatalf("%s=%q error = %v", name, bad, err)
				}
			})
		}
	}
}
