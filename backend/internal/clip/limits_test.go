package clip_test

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// The values moved out of platform/config with T269: what one render and one
// media run are allowed to spend is a clip rule, and only the Environment half
// comes from the deployment.
func TestDefaultRenderAndMediaShape(t *testing.T) {
	env := clip.Environment{ResvgPath: "/usr/local/bin/resvg", FontPaths: map[string]string{"pretendard": "a.ttf"}, MediaTimeout: 15 * time.Minute}
	r := clip.DefaultRenderConfig(env)
	if r.FadeMS != 200 || r.FPS != 30 || r.MinDurationMS != 15000 || r.MaxDurationMS != 90000 || r.ResvgPath != "/usr/local/bin/resvg" || len(r.FontPaths) != 1 {
		t.Fatalf("%+v", r)
	}
	m := clip.DefaultMediaConfig(env)
	if m.ChunkDurationMS != 60000 || m.LongEdge != 720 || m.FPS != 15 || m.DecodeThreads != 2 || m.EncodeThreads != 1 || m.AudioBitrate != 64000 || m.DurationToleranceMS != 1000 || m.Sources.MaxCount != 20 || m.Sources.MaxDurationMS != 1800000 || m.OperationTimeout != 15*time.Minute {
		t.Fatalf("%+v", m)
	}
	if m.AnalysisMaxBytes != 8<<20 || m.PreparedMaxBytes != 512<<20 || m.WorkspaceMaxBytes != 8<<30 || m.VideoMaxRate != 900000 || m.VideoBufferSize != 1800000 || m.RetryMaxRate != 650000 || m.RetryBufferSize != 1300000 || m.DiskCheckInterval != 100*time.Millisecond {
		t.Fatalf("unbounded shared media configuration: %+v", m)
	}
}

func TestClipConfirmedRetentionDoesNotFollowIncompleteUploadTTL(t *testing.T) {
	for _, incomplete := range []time.Duration{time.Hour, 6 * time.Hour, 48 * time.Hour} {
		limits := clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: incomplete, PutTTL: 10 * time.Minute})
		if limits.RetentionTTL != 24*time.Hour || limits.PlaybackTTL != 5*time.Minute || limits.BatchTTL != incomplete {
			t.Fatal("original policy inherited the upload bound", limits)
		}
	}
}

// The generation config is the one place the deployment's presign and sweep
// windows reach the clip domain; the rest of its numbers are the context's own.
func TestDefaultGenerationConfigTakesOnlyItsWindowsFromTheEnvironment(t *testing.T) {
	g := clip.DefaultGenerationConfig(clip.Environment{GetTTL: time.Minute, OrphanMinAge: time.Hour, QuoteTTL: 5 * time.Minute})
	if g.ReadTTL != time.Minute || g.OrphanMinAge != time.Hour || g.QuoteTTL != 5*time.Minute {
		t.Fatalf("%+v", g)
	}
	if g.CleanupTimeout != 30*time.Second || g.Preview.MaxAssets != 8 || g.Analysis.MaxSegments != 60 {
		t.Fatalf("product numbers moved with the environment: %+v", g)
	}
}
