package config

import (
	"github.com/postpilot/backend/internal/clip"
	"time"
)

func ClipGeneration(cfg *Config) clip.GenerationConfig {
	return clip.GenerationConfig{Preview: clip.PreviewConfig{MaxAssets: 8, MaxAssetBytes: 512 * 1024, MaxResponseBytes: 4 * 1024 * 1024, Timeout: 5 * time.Second}, Render: ClipRender(cfg), Media: ClipMedia(cfg), Analysis: ClipAI(cfg).Analysis, ReadTTL: cfg.PresignGetTTL, CleanupTimeout: 30 * time.Second, OrphanMinAge: cfg.OrphanMinAge, QuoteTTL: cfg.ClipQuoteTTL}
}
