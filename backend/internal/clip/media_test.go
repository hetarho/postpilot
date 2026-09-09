package clip_test

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
	"testing"
	"time"
)

func TestValidateProbedSources(t *testing.T) {
	cfg := config.ClipMedia(&config.Config{ClipSourceBatchTTL: 6 * time.Hour, PresignPutTTL: 10 * time.Minute})
	source := clip.ProbedSource{Metadata: clip.SourceMetadata{Filename: "source.MOV", ContentType: "video/quicktime", Fingerprint: "one", Bytes: 1024, DurationMS: 61000, Width: 1080, Height: 1920}, Info: clip.MediaInfo{DurationMS: 61000, Width: 1080, Height: 1920}}
	if n, err := clip.ValidateProbedSources(cfg, []clip.ProbedSource{source}); err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	other := source
	other.Metadata.Fingerprint = "two"
	other.Metadata.DurationMS = 120000
	other.Info.DurationMS = 120000
	if n, err := clip.ValidateProbedSources(cfg, []clip.ProbedSource{source, other}); err != nil || n != 4 {
		t.Fatalf("sum ceil chunk count: %d %v", n, err)
	}
	for name, change := range map[string]func(*clip.ProbedSource){
		"duration lie": func(v *clip.ProbedSource) { v.Metadata.DurationMS -= 1001 }, "rotation lie": func(v *clip.ProbedSource) { v.Metadata.Width, v.Metadata.Height = 1920, 1080 }, "empty": func(v *clip.ProbedSource) { v.Info.DurationMS = 0 }, "container": func(v *clip.ProbedSource) { v.Metadata.Filename = "a.avi" }, "mime": func(v *clip.ProbedSource) { v.Metadata.ContentType = "video/mp4" }, "bytes": func(v *clip.ProbedSource) { v.Metadata.Bytes = cfg.Sources.MaxFileBytes + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			v := source
			change(&v)
			if _, err := clip.ValidateProbedSources(cfg, []clip.ProbedSource{v}); err == nil {
				t.Fatal("accepted invalid source")
			}
		})
	}
	if _, err := clip.ValidateProbedSources(cfg, nil); err == nil {
		t.Fatal("accepted empty batch")
	}
	batch := make([]clip.ProbedSource, 21)
	if _, err := clip.ValidateProbedSources(cfg, batch); err == nil {
		t.Fatal("accepted 21 sources")
	}
	first, second := source, source
	first.Metadata.Fingerprint = "a"
	second.Metadata.Fingerprint = "b"
	first.Info.DurationMS = 1800000
	first.Metadata.DurationMS = 1800000
	if _, err := clip.ValidateProbedSources(cfg, []clip.ProbedSource{first, second}); err == nil {
		t.Fatal("accepted >30 minutes")
	}
	first = source
	first.Metadata.DurationMS -= 1000
	if _, err := clip.ValidateProbedSources(cfg, []clip.ProbedSource{first}); err != nil {
		t.Fatal("one-second tolerance rejected")
	}
}
