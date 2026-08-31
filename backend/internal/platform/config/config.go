// Package config loads process configuration from the environment. It is the only
// place that reads os.Getenv — everything else takes a *Config.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/postpilot/backend/internal/llm"
)

// sessionTTL is how long a session stays valid after login. Fixed at 30 days by the
// PRD (F-1), so it is a constant rather than an env var — an operator who changes it
// per environment changes a product rule, not a deployment detail.
const sessionTTL = 720 * time.Hour

// Presigned URL lifetimes. Short on purpose: a leaked URL is a leaked object for as
// long as it lives, and both are re-minted on demand (PUT at upload time, GET on every
// GetPost), so nothing breaks by expiring.
const (
	presignPutTTL = 10 * time.Minute
	presignGetTTL = 10 * time.Minute
)

// orphanMinAge is the grace period before the sweep may delete an object that has no
// database row. It is not zero because an upload in flight has an object and no row
// yet — deleting it would race the user's own PUT.
const orphanMinAge = time.Hour

// maxImageBytes is the largest object accepted as a photo.
//
// The browser converts to ~200 KB before uploading, so this is not a limit anyone hits
// by accident. It exists because a presigned PUT is a URL an authenticated client can
// use however it likes, and the browser's own cap cannot be trusted on the server side.
const maxImageBytes int64 = 10 << 20 // 10 MiB

// llmStageTimeout bounds one provider call. Generation takes 30 s – 2 min and a long
// draft can take longer on a slow model (PRD §6.6); the API is a long-lived container,
// so the bound exists to fail a hung call, not to fit a platform limit.
const llmStageTimeout = 5 * time.Minute

// llmMaxTokensDefault is the completion cap sent when a caller does not set one. Large
// enough for a full blog draft; a caller with a smaller output (an observation, a style
// analysis) may pass its own.
const llmMaxTokensDefault = 8192

// The queue is deliberately single-consumer at this scale: SQLite has one writer and
// parallel provider calls would only make rate limits and ordering less predictable.
const WorkerConcurrency = 1

// ExperimentCandidateConcurrency is the fixed pair width: one comparison has exactly
// two candidates and both may call providers concurrently.
const ExperimentCandidateConcurrency = 2

// VoicePersonalizationConfig contains product thresholds for progressive learning.
// They are deliberately code-owned: changing one changes product semantics, while no
// interval exists because personalization never runs on a clock.
type VoicePersonalizationConfig struct {
	FewShotTargetCount        int
	FewShotMax                int
	FewShotExcerptTargetChars int
	FewShotExcerptMaxChars    int
	EmbeddingSwitchPosts      int
	DiffMaxRules              int
	DiffMinPatternEdits       int
	RuleActivationEvidence    int
	RuleRetireAfter           time.Duration
	ValidationPostCount       int
	EndingMaxConsecutive      int
}

// LLMReasoningPolicy is code-owned because changing a stage's reasoning strength
// changes generation behavior rather than deployment topology. A model-level registry
// override still wins; Unset means the provider request stays untouched.
type LLMReasoningPolicy struct {
	Observe llm.ReasoningEffort
	Write   llm.ReasoningEffort
	Analyze llm.ReasoningEffort
}

func defaultLLMReasoningPolicy() LLMReasoningPolicy {
	return LLMReasoningPolicy{
		Observe: llm.ReasoningLow,
		Write:   llm.ReasoningLow,
		Analyze: llm.ReasoningUnset,
	}
}

func defaultVoicePersonalizationConfig() VoicePersonalizationConfig {
	return VoicePersonalizationConfig{
		FewShotTargetCount: 2, FewShotMax: 3,
		FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800,
		EmbeddingSwitchPosts: 50, DiffMaxRules: 3, DiffMinPatternEdits: 2,
		RuleActivationEvidence: 3, RuleRetireAfter: 180 * 24 * time.Hour,
		ValidationPostCount: 3, EndingMaxConsecutive: 2,
	}
}

// WorkerPollInterval is the fallback for a missed in-process wake signal.
const WorkerPollInterval = time.Second

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

	// R2Endpoint is the S3-compatible endpoint the API itself calls (HEAD, DELETE, LIST).
	R2Endpoint string
	// R2PublicEndpoint is the endpoint presigned URLs are minted against — the one the
	// BROWSER will call. It is separate because a signature covers the Host header, so a
	// URL signed for a name only the API can resolve is rejected when the browser sends
	// it. In production both are the same R2 endpoint and this is left unset; in local
	// dev the API reaches MinIO at `minio:9000` while the browser reaches `localhost:9000`.
	R2PublicEndpoint  string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string

	// PresignPutTTL bounds an upload URL; it is also how long the uploads row is valid.
	PresignPutTTL time.Duration
	// PresignGetTTL bounds a view URL. The frontend never persists one.
	PresignGetTTL time.Duration

	// OrphanSweepInterval is how often unconfirmed uploads and stray objects are cleaned
	// up. The PRD leaves the cadence undecided (§9.5); daily is the provisional default.
	OrphanSweepInterval time.Duration
	// OrphanMinAge is the grace period before an object with no row may be deleted.
	OrphanMinAge time.Duration
	// MaxImageBytes is the largest object recorded as a photo.
	MaxImageBytes int64

	// ProvidersConfig is the path of providers.yaml — the model registry (PRD §6.4). The
	// file names the env vars the API keys are read from; it never holds a key itself.
	ProvidersConfig string
	// LLMStageTimeout bounds one provider call.
	LLMStageTimeout time.Duration
	// LLMMaxTokensDefault is the completion cap when a caller sets none.
	LLMMaxTokensDefault int
	// LLMReasoning supplies stage defaults; model registry overrides take precedence.
	LLMReasoning LLMReasoningPolicy
	// ObserveBatchSize is the number of photos sent to one observation call.
	ObserveBatchSize int
	// ExperimentContentRetention starts after a verdict/dismissal and bounds how long
	// private inputs and candidate output remain stored.
	ExperimentContentRetention time.Duration
	// ExperimentSweepInterval is how often terminal experiment content is purged.
	ExperimentSweepInterval time.Duration
	// VoicePersonalization is injected into the voice and generation contexts. It has
	// no scheduler or sweep interval: all evaluation is request-time and user-initiated.
	VoicePersonalization VoicePersonalizationConfig

	// Publishing owns a separate durable queue because a browser-side external commit
	// has lease and ambiguity semantics that generation jobs do not have (plan 12).
	PublishPairingTTL          time.Duration
	PublishMaxPendingPairings  int
	PublishLeaseTTL            time.Duration
	PublishAssetURLTTL         time.Duration
	PublishOrphanSweepInterval time.Duration
	PublishOrphanMinAge        time.Duration
	// PublishAgentHeartbeatInterval is the minimum spacing between last_seen_at writes
	// for one agent; calls arriving sooner are served without touching the writer. The
	// refresh only happens on a poll that lands after the interval has elapsed, so the
	// worst observed staleness is this interval plus the agent's poll (5s), and that sum
	// must stay under the frontend's PUBLISH_AGENT_STALE_MS (30s) or a live agent renders
	// as offline. The three values sit in three different seams, so nothing but this note
	// connects them.
	PublishAgentHeartbeatInterval time.Duration

	// Purpose brief field ceilings, counted in Unicode scalar values like the voice
	// sample minimum. They are env-owned rather than constants because a brief that is
	// too short to be useful is a per-account editorial judgement, not a product rule.
	PurposeNameMaxChars         int
	PurposeDescriptionMaxChars  int
	PurposeInstructionsMaxChars int
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

		R2Endpoint:        os.Getenv("R2_ENDPOINT"),
		R2AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:          os.Getenv("R2_BUCKET"),

		PresignPutTTL: presignPutTTL,
		PresignGetTTL: presignGetTTL,
		OrphanMinAge:  orphanMinAge,
		MaxImageBytes: maxImageBytes,

		// Relative to the working directory, like DB_PATH: `backend/config/providers.yaml`
		// for a host run and `/app/config/providers.yaml` in the dev container. The
		// production image sets an absolute path (Dockerfile).
		ProvidersConfig:      getenv("PROVIDERS_CONFIG", "config/providers.yaml"),
		LLMStageTimeout:      llmStageTimeout,
		LLMMaxTokensDefault:  llmMaxTokensDefault,
		LLMReasoning:         defaultLLMReasoningPolicy(),
		VoicePersonalization: defaultVoicePersonalizationConfig(),
	}
	cfg.R2PublicEndpoint = getenv("R2_PUBLIC_ENDPOINT", cfg.R2Endpoint)

	if err := validateOrigin(cfg.CORSOrigin); err != nil {
		return nil, fmt.Errorf("CORS_ORIGIN: %w", err)
	}

	sweep, err := time.ParseDuration(getenv("ORPHAN_SWEEP_INTERVAL", "24h"))
	if err != nil {
		return nil, fmt.Errorf("ORPHAN_SWEEP_INTERVAL: %w", err)
	}
	if sweep <= 0 {
		return nil, fmt.Errorf("ORPHAN_SWEEP_INTERVAL: must be positive, got %s", sweep)
	}
	cfg.OrphanSweepInterval = sweep

	batchSize, err := strconv.Atoi(getenv("OBSERVE_BATCH_SIZE", "4"))
	if err != nil || batchSize <= 0 {
		return nil, fmt.Errorf("OBSERVE_BATCH_SIZE: must be a positive integer")
	}
	cfg.ObserveBatchSize = batchSize

	retention, err := positiveDuration("EXPERIMENT_CONTENT_RETENTION", "720h")
	if err != nil {
		return nil, err
	}
	cfg.ExperimentContentRetention = retention
	experimentSweep, err := positiveDuration("EXPERIMENT_SWEEP_INTERVAL", "24h")
	if err != nil {
		return nil, err
	}
	cfg.ExperimentSweepInterval = experimentSweep

	pairingTTL, err := positiveDuration("PUBLISH_PAIRING_TTL", "10m")
	if err != nil {
		return nil, err
	}
	cfg.PublishPairingTTL = pairingTTL
	maxPairings, err := positiveInt("PUBLISH_MAX_PENDING_PAIRINGS", "8")
	if err != nil {
		return nil, err
	}
	cfg.PublishMaxPendingPairings = maxPairings
	leaseTTL, err := positiveDuration("PUBLISH_LEASE_TTL", "45s")
	if err != nil {
		return nil, err
	}
	cfg.PublishLeaseTTL = leaseTTL
	assetURLTTL, err := positiveDuration("PUBLISH_ASSET_URL_TTL", "10m")
	if err != nil {
		return nil, err
	}
	cfg.PublishAssetURLTTL = assetURLTTL
	publishSweep, err := positiveDuration("PUBLISH_ORPHAN_SWEEP_INTERVAL", "24h")
	if err != nil {
		return nil, err
	}
	cfg.PublishOrphanSweepInterval = publishSweep
	publishMinAge, err := positiveDuration("PUBLISH_ORPHAN_MIN_AGE", "1h")
	if err != nil {
		return nil, err
	}
	cfg.PublishOrphanMinAge = publishMinAge
	agentHeartbeat, err := positiveDuration("PUBLISH_AGENT_HEARTBEAT_INTERVAL", "15s")
	if err != nil {
		return nil, err
	}
	cfg.PublishAgentHeartbeatInterval = agentHeartbeat

	purposeName, err := positiveInt("PURPOSE_NAME_MAX_CHARS", "40")
	if err != nil {
		return nil, err
	}
	cfg.PurposeNameMaxChars = purposeName
	purposeDescription, err := positiveInt("PURPOSE_DESCRIPTION_MAX_CHARS", "200")
	if err != nil {
		return nil, err
	}
	cfg.PurposeDescriptionMaxChars = purposeDescription
	purposeInstructions, err := positiveInt("PURPOSE_INSTRUCTIONS_MAX_CHARS", "2000")
	if err != nil {
		return nil, err
	}
	cfg.PurposeInstructionsMaxChars = purposeInstructions

	return cfg, nil
}

func positiveDuration(name, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(getenv(name, fallback))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s: must be positive, got %s", name, value)
	}
	return value, nil
}

func positiveInt(name, fallback string) (int, error) {
	value, err := strconv.Atoi(getenv(name, fallback))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s: must be a positive integer", name)
	}
	return value, nil
}

// RequireObjectStorage validates the R2 block.
//
// It is a separate step rather than part of Load because not every entry point needs
// object storage: `api adduser` creates an account, and on a fresh VPS that happens
// before the operator has set up a bucket. Only the server calls this — and it does so
// before the listener starts, so a missing value still stops the deploy at the health
// gate rather than surfacing as a failed upload the first time a user picks a photo.
func (c *Config) RequireObjectStorage() error {
	for name, value := range map[string]string{
		"R2_ENDPOINT":          c.R2Endpoint,
		"R2_ACCESS_KEY_ID":     c.R2AccessKeyID,
		"R2_SECRET_ACCESS_KEY": c.R2SecretAccessKey,
		"R2_BUCKET":            c.R2Bucket,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required (object storage for photos)", name)
		}
	}
	for name, value := range map[string]string{
		"R2_ENDPOINT":        c.R2Endpoint,
		"R2_PUBLIC_ENDPOINT": c.R2PublicEndpoint,
	} {
		if err := validateEndpoint(value); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
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

// validateEndpoint catches an object-storage URL that would only fail later, inside a
// presigned URL the browser cannot use and cannot explain.
func validateEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must be an http(s) URL, got %q", endpoint)
	}
	if u.Host == "" {
		return fmt.Errorf("must include a host, got %q", endpoint)
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
