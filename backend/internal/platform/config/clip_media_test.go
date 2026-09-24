package config

import (
	"testing"
	"time"
)

func TestMediaStageEnvironmentOverrides(t *testing.T) {
	for _, key := range []string{"CLIP_MEDIA_LEASE_TTL", "CLIP_MEDIA_WAIT_TIMEOUT", "CLIP_MEDIA_STAGE_TIMEOUT", "CLIP_MEDIA_MAX_ATTEMPTS"} {
		for _, value := range []string{"0", "-1", "invalid"} {
			t.Run(key+value, func(t *testing.T) {
				t.Setenv(key, value)
				if err := loadMediaStageOverrides(&Config{}); err == nil {
					t.Fatal("invalid override accepted")
				}
			})
		}
	}
	t.Setenv("CLIP_MEDIA_LEASE_TTL", "45s")
	t.Setenv("CLIP_MEDIA_WAIT_TIMEOUT", "5m")
	t.Setenv("CLIP_MEDIA_STAGE_TIMEOUT", "1h")
	t.Setenv("CLIP_MEDIA_MAX_ATTEMPTS", "2")
	var cfg Config
	if err := loadMediaStageOverrides(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ClipMediaLeaseTTL != 45*time.Second || cfg.ClipMediaWaitTimeout != 5*time.Minute || cfg.ClipMediaStageTimeout != time.Hour || cfg.ClipMediaMaxAttempts != 2 {
		t.Fatal("overrides lost")
	}
}

// The product shape these paths feed is pinned in internal/clip (limits_test.go).
// What belongs here is only that the environment is parsed and validated.
func TestClipMediaEnvironmentIsValidated(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClipResvgPath != "/usr/local/bin/resvg" || len(cfg.ClipFontPaths) != 5 || cfg.ClipFontPaths["wantedsans"] != "/usr/share/postpilot-fonts/wantedsans/WantedSansVariable.ttf" || cfg.ClipFontPaths["nanummyeongjo-800"] != "/usr/share/postpilot-fonts/nanummyeongjo/NanumMyeongjo-ExtraBold.ttf" {
		t.Fatalf("%+v", cfg.ClipFontPaths)
	}
	for key, value := range map[string]string{"CLIP_WORK_ROOT": "/", "CLIP_FFMPEG_PATH": "ffmpeg", "CLIP_FFPROBE_PATH": "$BIN/ffprobe", "CLIP_RESVG_PATH": "resvg", "CLIP_FONT_PATH": "$FONT/file.ttf", "CLIP_FONT_JUA_PATH": "jua.ttf", "CLIP_WORK_STALE_AGE": "0s", "CLIP_MEDIA_TIMEOUT": "-1m"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestClipOverlayDirectoryConfiguration(t *testing.T) {
	for _, value := range []string{"", t.TempDir()} {
		t.Setenv("CLIP_OVERLAY_DIR", value)
		cfg, err := Load()
		if err != nil || cfg.ClipOverlayDir != value {
			t.Fatalf("overlay directory not propagated: %v", err)
		}
	}
	for _, value := range []string{"relative/presets", "/", "$PRESETS/assets", "/tmp/../presets"} {
		t.Setenv("CLIP_OVERLAY_DIR", value)
		if _, err := Load(); err == nil {
			t.Fatalf("invalid directory accepted: %q", value)
		}
	}
}
