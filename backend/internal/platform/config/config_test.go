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
	t.Setenv("LLM_MAX_TOKENS_DEFAULT", "")

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
	// A8: the cap is deployment-resolvable, with 8192 as its default.
	if cfg.LLMMaxTokensDefault != 8192 {
		t.Errorf("LLMMaxTokensDefault = %d, want 8192", cfg.LLMMaxTokensDefault)
	}
	if cfg.LLMReasoning.Observe != llm.ReasoningLow || cfg.LLMReasoning.Write != llm.ReasoningLow {
		t.Errorf("LLMReasoning = %+v", cfg.LLMReasoning)
	}
	// A7: the observation budget is derived from the batch size, not from the writer's.
	if got, want := cfg.LLMCompletionBudget.Observation(), 4*observeBudgetPerPhoto; got != want {
		t.Errorf("observation budget = %d, want %d for a batch of 4", got, want)
	}
	// Job 23 raised the effective write budget to 8,192 to stop write-stage truncation. A
	// post that requests no length must still be sent exactly that.
	if got := cfg.LLMCompletionBudget.Write(nil); got != 8192 {
		t.Errorf("no-target write budget = %d, want the configured fallback 8192", got)
	}
}

// The observation budget follows OBSERVE_BATCH_SIZE, because that is how many structured
// entries one call returns — a fixed number truncates the moment an operator raises the batch.
func TestObservationBudgetFollowsTheBatchSize(t *testing.T) {
	t.Setenv("LLM_MAX_TOKENS_DEFAULT", "")
	t.Setenv("OBSERVE_BATCH_SIZE", "12")
	big, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBSERVE_BATCH_SIZE", "1")
	small, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if big.LLMCompletionBudget.Observation() <= small.LLMCompletionBudget.Observation() {
		t.Errorf("a batch of 12 got %d, not more than a batch of 1's %d",
			big.LLMCompletionBudget.Observation(), small.LLMCompletionBudget.Observation())
	}
	// A batch of one is still floored to a usable entry.
	if small.LLMCompletionBudget.Observation() != observeBudgetFloor {
		t.Errorf("a batch of 1 got %d, want the floor %d",
			small.LLMCompletionBudget.Observation(), observeBudgetFloor)
	}
}

// A revision re-emits the WHOLE PostContent, so its budget has to fit what already exists —
// a long post with no requested length would otherwise get the floor and truncate every time.
func TestRevisionBudgetFitsTheContentItReEmits(t *testing.T) {
	t.Setenv("LLM_MAX_TOKENS_DEFAULT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	budget := cfg.LLMCompletionBudget

	if got := budget.Revise(0, nil); got != budget.Write(nil) {
		t.Errorf("an empty revision = %d, want the writer's floor %d", got, budget.Write(nil))
	}
	long := budget.Revise(6000, nil)
	if long <= budget.Write(nil) {
		t.Errorf("a 6,000-character post to re-emit got %d, no more than the floor %d", long, budget.Write(nil))
	}
	// The larger of the two wins, in both directions.
	if got := budget.Revise(6000, intPointer(500)); got != long {
		t.Errorf("a short target shrank a long revision to %d, want %d", got, long)
	}
	if got := budget.Revise(100, intPointer(6000)); got != long {
		t.Errorf("a long target on short content gave %d, want %d", got, long)
	}
	if got := budget.Revise(10_000_000, nil); got != budget.Ceiling {
		t.Errorf("an enormous post = %d, want the ceiling %d", got, budget.Ceiling)
	}
}

// A6: the writing budget follows the post's requested length, floored so a post with no
// target is never squeezed and capped so a mistyped target cannot ask for an unbounded
// completion. A unit test over the derivation, not a provider call.
func TestWritingBudgetFollowsTheRequestedLength(t *testing.T) {
	t.Setenv("LLM_MAX_TOKENS_DEFAULT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	budget := cfg.LLMCompletionBudget

	if got := budget.Write(nil); got != 8192 {
		t.Errorf("no target = %d, want the configured fallback 8192", got)
	}
	if got := budget.Write(intPointer(500)); got != 8192 {
		t.Errorf("a short target = %d, want the fallback floor 8192", got)
	}
	// A6: a longer requested draft is sent a larger budget. The comparison is made where the
	// derivation is above the floor, which is what "a longer draft raises the ceiling instead
	// of meeting the same wall" means.
	short, long := budget.Write(intPointer(3000)), budget.Write(intPointer(6000))
	if long <= short {
		t.Errorf("a longer requested draft got %d, not more than the shorter one's %d", long, short)
	}
	if short <= budget.Write(nil) {
		t.Errorf("a 3,000-character target got %d, no more than the no-target floor", short)
	}
	if got := budget.Write(intPointer(1_000_000)); got != budget.Ceiling {
		t.Errorf("an absurd target = %d, want the ceiling %d", got, budget.Ceiling)
	}
	// The observation stage is independent of the writer's, which is the whole point of the
	// split: a stage given headroom it does not need hands a reasoning model room to fill.
	if budget.Observation() >= budget.Write(nil) {
		t.Errorf("observation %d is not smaller than the writer's floor %d", budget.Observation(), budget.Write(nil))
	}
}

// A8: an invalid cap is refused at boot, the way OBSERVE_BATCH_SIZE is.
func TestInvalidCompletionCapIsRefused(t *testing.T) {
	for _, value := range []string{"0", "-1", "lots"} {
		t.Setenv("LLM_MAX_TOKENS_DEFAULT", value)
		if _, err := Load(); err == nil {
			t.Errorf("LLM_MAX_TOKENS_DEFAULT=%q was accepted", value)
		}
	}
	t.Setenv("LLM_MAX_TOKENS_DEFAULT", "16384")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// The resolved value is the registry's fallback AND the writer's floor, and the ceiling is
	// a bounded multiple of it — so one env value moves the whole policy.
	if cfg.LLMMaxTokensDefault != 16384 || cfg.LLMCompletionBudget.Write(nil) != 16384 {
		t.Errorf("a resolved cap did not reach the budget: %d / %d", cfg.LLMMaxTokensDefault, cfg.LLMCompletionBudget.Write(nil))
	}
	if cfg.LLMCompletionBudget.Ceiling != 16384*writeBudgetCeilingFactor {
		t.Errorf("ceiling = %d, want %d", cfg.LLMCompletionBudget.Ceiling, 16384*writeBudgetCeilingFactor)
	}
}

func intPointer(value int) *int { return &value }

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

func TestLoadTemplateLimitDefaultsAndValidation(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	names := []string{
		"TEMPLATE_NAME_MAX_CHARS", "TEMPLATE_DESCRIPTION_MAX_CHARS", "TEMPLATE_BODY_MAX_CHARS",
		"TEMPLATE_MAX_PER_ACCOUNT", "TEMPLATE_MAX_REPEAT_EXPANSION",
	}
	for _, name := range names {
		t.Setenv(name, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TemplateNameMaxChars != 40 || cfg.TemplateDescriptionMaxChars != 200 || cfg.TemplateBodyMaxChars != 4000 ||
		cfg.TemplateMaxPerAccount != 50 || cfg.TemplateMaxRepeatExpansion != 40 {
		t.Fatalf("template limit defaults = %+v", cfg)
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

func TestLoadGuidelineLimitDefaultsAndValidation(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	names := []string{"GUIDELINE_TEXT_MAX_CHARS", "GUIDELINE_MAX_PER_ACCOUNT"}
	for _, name := range names {
		t.Setenv(name, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GuidelineTextMaxChars != 300 || cfg.GuidelineMaxPerAccount != 100 {
		t.Fatalf("guideline limit defaults = %+v", cfg)
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
