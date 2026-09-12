package config

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// FadeMS is the design system's transition constant (CDS-36), not a setting:
// the renderer's constructor refuses a configuration that disagrees with it.
func ClipRender(cfg *Config) clip.RenderConfig {
	return clip.RenderConfig{Composition: ClipCompositionLimits(), OverlayBatchSize: 8, ResvgPath: cfg.ClipResvgPath, FontPath: cfg.ClipFontPath, DisplayFontPath: cfg.ClipDisplayFontPath, OverlayDir: cfg.ClipOverlayDir, MaxCuts: 100, MaxCopyRunes: 500, FadeMS: design.Transition.FadeMS, FPS: 30, CRF: 20, AudioRate: 48000, AudioBitrate: 192000, MinDurationMS: ClipMinDurationMS, MaxDurationMS: ClipMaxDurationMS}
}
