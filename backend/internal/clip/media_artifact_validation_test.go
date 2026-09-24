package clip_test

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
)

func artifactManifest() (clip.MediaTask, clip.MediaResult) {
	meta := clip.SourceMetadata{Filename: "original.mp4", ContentType: "video/mp4", Fingerprint: strings.Repeat("a", 64), Bytes: 100, DurationMS: 61000, Width: 1920, Height: 1080}
	task := clip.MediaTask{Version: clip.MediaContractVersion, Sources: []clip.MediaTaskSource{{ID: "source", SourceMetadata: meta}}}
	result := clip.MediaResult{Version: clip.MediaContractVersion, Sources: []clip.MediaVerifiedSource{{ID: "source", Fingerprint: meta.Fingerprint, Info: clip.MediaInfo{Width: 1920, Height: 1080, DurationMS: 61000}}}}
	for i, duration := range []int{60000, 1000} {
		result.Outputs = append(result.Outputs, clip.MediaOutput{Slot: clip.MediaAnalysisSlot("source", i), SourceID: "source", Index: i, OffsetMS: i * 60000, DurationMS: duration, Bytes: 100, ContentType: "video/mp4", Digest: strings.Repeat("b", 64), Info: clip.MediaInfo{Width: 720, Height: 406, DurationMS: duration}})
	}
	return task, result
}
func TestMediaArtifactCoverageAndLimits(t *testing.T) {
	cfg := clip.DefaultMediaConfig(clip.Environment{})
	task, result := artifactManifest()
	if err := clip.ValidateMediaTask(clip.MediaPrepare, task, cfg); err != nil {
		t.Fatal(err)
	}
	if err := clip.ValidateMediaResult(clip.MediaPrepare, task, result, cfg); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*clip.MediaResult){
		"missing copy":      func(r *clip.MediaResult) { r.Outputs = r.Outputs[:1] },
		"foreign source":    func(r *clip.MediaResult) { r.Sources[0].ID = "foreign" },
		"wrong fingerprint": func(r *clip.MediaResult) { r.Sources[0].Fingerprint = "foreign" },
		"reordered copies":  func(r *clip.MediaResult) { r.Outputs[0], r.Outputs[1] = r.Outputs[1], r.Outputs[0] },
		"duplicate ordinal": func(r *clip.MediaResult) { r.Outputs[1] = r.Outputs[0] },
		"offset":            func(r *clip.MediaResult) { r.Outputs[1].OffsetMS++ },
		"duration":          func(r *clip.MediaResult) { r.Outputs[1].DurationMS++ },
		"oversized copy":    func(r *clip.MediaResult) { r.Outputs[0].Bytes = cfg.AnalysisMaxBytes + 1 },
		"large dimension":   func(r *clip.MediaResult) { r.Outputs[0].Info.Width = 721 },
		"etag as digest":    func(r *clip.MediaResult) { r.Outputs[0].Digest = strings.Repeat("a", 32) },
		"wrong mime":        func(r *clip.MediaResult) { r.Outputs[0].ContentType = "video/quicktime" },
		"source dimensions": func(r *clip.MediaResult) { r.Sources[0].Info.Width++ },
	} {
		t.Run(name, func(t *testing.T) {
			task, result := artifactManifest()
			mutate(&result)
			if err := clip.ValidateMediaResult(clip.MediaPrepare, task, result, cfg); err == nil {
				t.Fatal("invalid artifact manifest accepted")
			}
		})
	}
	cfg.PreparedMaxBytes = 199
	if err := clip.ValidateMediaResult(clip.MediaPrepare, task, result, cfg); err == nil {
		t.Fatal("aggregate overflow accepted")
	}
	cfg = clip.DefaultMediaConfig(clip.Environment{})
	task.Sources[0].ReusedChunks = []int{0}
	result.Outputs = result.Outputs[1:]
	if err := clip.ValidateMediaResult(clip.MediaPrepare, task, result, cfg); err != nil {
		t.Fatal("missing-observation-only manifest refused", err)
	}
	result.Outputs = nil
	if err := clip.ValidateMediaResult(clip.MediaPrepare, task, result, cfg); err == nil {
		t.Fatal("missing non-reused copy accepted")
	}
}
func TestMediaCodecRefusesUnknownCommandsAndVersions(t *testing.T) {
	task, _ := artifactManifest()
	raw, err := mediacodec.EncodeTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mediacodec.DecodeTask(raw); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(raw, `"Version":1`, `"Version":99`, 1), strings.Replace(raw, `"Version":1`, `"Version":1,"Command":"ffmpeg -i /etc/passwd"`, 1), raw + "{}", strings.Repeat(" ", clip.MediaPayloadMaxBytes+1)} {
		if _, err := mediacodec.DecodeTask(bad); err == nil {
			t.Fatal("invalid contract decoded")
		}
	}
}

func TestMediaRenderArtifactRequiresDeliveredMediaProperties(t *testing.T) {
	plan, sources := validPlan()
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := sources[0]
	task := clip.MediaTask{Version: clip.MediaContractVersion, Plan: raw, Sources: []clip.MediaTaskSource{{ID: s.ID, Info: s.Info, SourceMetadata: clip.SourceMetadata{Fingerprint: s.Fingerprint, Filename: "original.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: s.Info.DurationMS, Width: s.Info.Width, Height: s.Info.Height}}}}
	cfg := clip.DefaultMediaConfig(clip.Environment{})
	if err := clip.ValidateMediaTask(clip.MediaRender, task, cfg); err != nil {
		t.Fatal(err)
	}
	out := clip.MediaOutput{Slot: "result", ContentType: "video/mp4", Bytes: 100, Digest: strings.Repeat("a", 64), DurationMS: 15000, Info: clip.MediaInfo{Width: 1080, Height: 1920, DurationMS: 15000, PixelFormat: "yuv420p", SampleAspectRatio: "1:1", FrameRateNumerator: 30, FrameRateDenominator: 1, DecodedFrames: 450, Streams: []clip.MediaStream{{Kind: "video", Codec: "h264", Profile: "High"}}}}
	if err := clip.ValidateMediaOutput(clip.MediaRender, task, out, cfg); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*clip.MediaOutput){
		func(o *clip.MediaOutput) { o.Bytes = clip.SourceFileBytes + 1 }, func(o *clip.MediaOutput) { o.Slot = "analysis/source/0" }, func(o *clip.MediaOutput) { o.Info.DecodedFrames = 449 - 1 }, func(o *clip.MediaOutput) { o.Info.Width = 720 }, func(o *clip.MediaOutput) { o.Info.PixelFormat = "yuv444p" }, func(o *clip.MediaOutput) { o.Info.FrameRateNumerator = 15 }, func(o *clip.MediaOutput) { o.Info.HasAudio = true }, func(o *clip.MediaOutput) { o.Info.Streams = nil },
	} {
		bad := out
		mutate(&bad)
		if err := clip.ValidateMediaOutput(clip.MediaRender, task, bad, cfg); err == nil {
			t.Fatal("invalid delivered metadata accepted")
		}
	}
}
