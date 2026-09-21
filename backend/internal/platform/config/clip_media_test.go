package config

import (
	"testing"
)

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
