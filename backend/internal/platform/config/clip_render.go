package config

import "github.com/postpilot/backend/internal/clip"

func ClipRender(cfg *Config) clip.RenderConfig {
	return clip.RenderConfig{ResvgPath: cfg.ClipResvgPath, FontPath: cfg.ClipFontPath, MaxCuts: 100, MaxCopyRunes: 500, FadeMS: 200, FPS: 30, CRF: 20, AudioRate: 48000, AudioBitrate: 192000, MinDurationMS: ClipMinDurationMS, MaxDurationMS: ClipMaxDurationMS}
}
