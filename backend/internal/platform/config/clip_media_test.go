package config

import (
	"testing"
	"time"
)

func TestClipMediaConfiguration(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	m := ClipMedia(cfg)
	if m.ChunkDurationMS != 60000 || m.LongEdge != 720 || m.FPS != 30 || m.DurationToleranceMS != 1000 || m.Sources.MaxCount != 20 || m.Sources.MaxDurationMS != 1800000 || m.OperationTimeout != 15*time.Minute {
		t.Fatalf("%+v", m)
	}
	for key, value := range map[string]string{"CLIP_WORK_ROOT": "/", "CLIP_FFMPEG_PATH": "ffmpeg", "CLIP_FFPROBE_PATH": "$BIN/ffprobe", "CLIP_WORK_STALE_AGE": "0s", "CLIP_MEDIA_TIMEOUT": "-1m"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
