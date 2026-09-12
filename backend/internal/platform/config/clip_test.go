package config

import (
	"testing"
	"time"
)

func TestClipSourceOperationalDefaultsAndOverrides(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:2564")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClipSourceBatchTTL != 6*time.Hour || cfg.ClipSourceSweepInterval != 10*time.Minute {
		t.Fatal(cfg.ClipSourceBatchTTL, cfg.ClipSourceSweepInterval)
	}
	if cfg.ClipQuoteTTL != 5*time.Minute || ClipGeneration(cfg).QuoteTTL != cfg.ClipQuoteTTL {
		t.Fatal("quote TTL not wired", cfg.ClipQuoteTTL)
	}
	for _, name := range []string{"CLIP_SOURCE_BATCH_TTL", "CLIP_SOURCE_SWEEP_INTERVAL", "CLIP_QUOTE_TTL"} {
		t.Run(name, func(t *testing.T) {
			for _, value := range []string{"0s", "-1h", "invalid"} {
				t.Setenv(name, value)
				if _, err := Load(); err == nil {
					t.Fatal(name, value)
				}
			}
		})
	}
	t.Setenv("CLIP_SOURCE_BATCH_TTL", "1m")
	if _, err := Load(); err == nil {
		t.Fatal("batch cannot expire before its URL")
	}
}

func TestClipConfirmedRetentionDoesNotFollowIncompleteUploadTTL(t *testing.T) {
	for _, incomplete := range []time.Duration{time.Hour, 6 * time.Hour, 48 * time.Hour} {
		limits := ClipSourceLimits(incomplete, 10*time.Minute)
		if limits.RetentionTTL != 24*time.Hour || limits.PlaybackTTL != 5*time.Minute || limits.BatchTTL != incomplete {
			t.Fatal("original policy inherited the upload bound", limits)
		}
	}
}
