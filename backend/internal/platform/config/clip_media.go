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
	// Every face CDS-17 names, each its own setting so an image may move one
	// file without the others. NanumMyeongjo ships the two weights CDS-17 asks
	// for as two files under one family.
	cfg.ClipFontPaths = map[string]string{
		"pretendard":        getenv("CLIP_FONT_PATH", "/usr/share/postpilot-fonts/pretendard/PretendardVariable.ttf"),
		"paperlogy":         getenv("CLIP_FONT_PAPERLOGY_PATH", "/usr/share/postpilot-fonts/paperlogy/Paperlogy-8ExtraBold.ttf"),
		"jua":               getenv("CLIP_FONT_JUA_PATH", "/usr/share/postpilot-fonts/jua/Jua-Regular.ttf"),
		"nanummyeongjo":     getenv("CLIP_FONT_NANUM_MYEONGJO_PATH", "/usr/share/postpilot-fonts/nanummyeongjo/NanumMyeongjo-Regular.ttf"),
		"nanummyeongjo-800": getenv("CLIP_FONT_NANUM_MYEONGJO_BOLD_PATH", "/usr/share/postpilot-fonts/nanummyeongjo/NanumMyeongjo-ExtraBold.ttf"),
	}
	cfg.ClipOverlayDir = getenv("CLIP_OVERLAY_DIR", "")
	if value := cfg.ClipOverlayDir; value != "" && (strings.ContainsAny(value, "$\x00") || !filepath.IsAbs(value) || filepath.Clean(value) != value || filepath.Dir(value) == "/") {
		return fmt.Errorf("CLIP_OVERLAY_DIR must be a dedicated absolute directory")
	}
	paths := map[string]string{"CLIP_WORK_ROOT": cfg.ClipWorkRoot, "CLIP_FFMPEG_PATH": cfg.ClipFFmpegPath, "CLIP_FFPROBE_PATH": cfg.ClipFFprobePath, "CLIP_RESVG_PATH": cfg.ClipResvgPath}
	for face, value := range cfg.ClipFontPaths {
		paths["CLIP_FONT_"+strings.ToUpper(face)] = value
	}
	for key, value := range paths {
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
