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
	r := ClipRender(cfg)
	if r.FadeMS != 200 || r.FPS != 30 || r.MinDurationMS != 15000 || r.MaxDurationMS != 90000 || r.ResvgPath != "/usr/local/bin/resvg" || r.FontPath != "/usr/share/postpilot-fonts/pretendard/PretendardVariable.ttf" {
		t.Fatalf("%+v", r)
	}
	if m.ChunkDurationMS != 60000 || m.LongEdge != 720 || m.FPS != 15 || m.Threads != 1 || m.AudioBitrate != 64000 || m.DurationToleranceMS != 1000 || m.Sources.MaxCount != 20 || m.Sources.MaxDurationMS != 1800000 || m.OperationTimeout != 15*time.Minute {
		t.Fatalf("%+v", m)
	}
	if WorkerConcurrency != 1 || m.AnalysisMaxBytes != 8<<20 || m.PreparedMaxBytes != 512<<20 || m.WorkspaceMaxBytes != 8<<30 || m.VideoMaxRate != 900000 || m.VideoBufferSize != 1800000 || m.RetryMaxRate != 650000 || m.RetryBufferSize != 1300000 || m.DiskCheckInterval != 100*time.Millisecond {
		t.Fatalf("unbounded shared media configuration: %+v", m)
	}
	for key, value := range map[string]string{"CLIP_WORK_ROOT": "/", "CLIP_FFMPEG_PATH": "ffmpeg", "CLIP_FFPROBE_PATH": "$BIN/ffprobe", "CLIP_RESVG_PATH": "resvg", "CLIP_FONT_PATH": "$FONT/file.ttf", "CLIP_WORK_STALE_AGE": "0s", "CLIP_MEDIA_TIMEOUT": "-1m"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
