package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

func loadClipMedia(cfg *Config) error {
	cfg.ClipWorkRoot = getenv("CLIP_WORK_ROOT", "/tmp/postpilot-clip-work")
	cfg.ClipFFmpegPath = getenv("CLIP_FFMPEG_PATH", "/usr/local/bin/ffmpeg")
	cfg.ClipFFprobePath = getenv("CLIP_FFPROBE_PATH", "/usr/local/bin/ffprobe")
	cfg.ClipResvgPath = getenv("CLIP_RESVG_PATH", "/usr/local/bin/resvg")
	cfg.ClipFontPath = getenv("CLIP_FONT_PATH", "/usr/share/postpilot-fonts/pretendard/PretendardVariable.ttf")
	for key, value := range map[string]string{"CLIP_WORK_ROOT": cfg.ClipWorkRoot, "CLIP_FFMPEG_PATH": cfg.ClipFFmpegPath, "CLIP_FFPROBE_PATH": cfg.ClipFFprobePath, "CLIP_RESVG_PATH": cfg.ClipResvgPath, "CLIP_FONT_PATH": cfg.ClipFontPath} {
		if strings.ContainsAny(value, "$\x00") || !filepath.IsAbs(value) || filepath.Clean(value) != value || filepath.Dir(value) == "/" {
			return fmt.Errorf("%s must be a dedicated absolute path", key)
		}
	}
	var err error
	cfg.ClipWorkStaleAge, err = positiveDuration("CLIP_WORK_STALE_AGE", "6h")
	if err != nil {
		return err
	}
	cfg.ClipMediaTimeout, err = positiveDuration("CLIP_MEDIA_TIMEOUT", "15m")
	return err
}
