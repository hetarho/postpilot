package config

import (
	"github.com/postpilot/backend/internal/clip"
	"time"
)

func ClipGeneration(cfg *Config) clip.GenerationConfig {
	return clip.GenerationConfig{Render: ClipRender(cfg), Media: ClipMedia(cfg), Analysis: ClipAI(cfg).Analysis, ReadTTL: cfg.PresignGetTTL, CleanupTimeout: 30 * time.Second, OrphanMinAge: cfg.OrphanMinAge, QuoteTTL: cfg.ClipQuoteTTL}
}
