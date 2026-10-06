package analysisquality

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// Reuse an operator-selected existing synthetic MP4 and its private Input JSON;
// this smoke creates no media corpus and establishes no semantic quality.
func TestLocalVerifierExistingAuthorizedSyntheticCopy(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("root-serialized execution-image gate")
	}
	path := os.Getenv("T603_VERIFIER_INPUT_JSON")
	if path == "" {
		t.Fatal("explicit private existing synthetic copy input is required")
	}
	stat, e := os.Lstat(path)
	if e != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() > MaxDocumentBytes {
		t.Fatal("invalid private smoke input")
	}
	data, e := os.ReadFile(path)
	var in Input
	if e != nil || strict(data, &in, MaxDocumentBytes) != nil || (clip.AnalysisVerificationTask{Version: 1, ProfileVersion: in.Profile, ManifestDigest: hash(data), Copies: []clip.AnalysisCopy{in.Copy}}).Validate() != nil {
		t.Fatal("invalid smoke copy declaration", e)
	}
	root := filepath.Dir(path)
	data, e = readPrivate(root, in.File, 8<<20)
	if e != nil {
		t.Fatal(e)
	}
	ffmpeg, ffprobe := os.Getenv("FFMPEG_PATH"), os.Getenv("FFPROBE_PATH")
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	if ffprobe == "" {
		ffprobe = "ffprobe"
	}
	verify, e := LocalVerifier(filepath.Join(t.TempDir(), "work"), ffmpeg, ffprobe)
	if e != nil {
		t.Fatal(e)
	}
	v, e := verify(t.Context(), in, data)
	if e != nil || clip.ValidateAnalysisCopyVerification(in.Copy, v, clip.DefaultMediaConfig(clip.Environment{})) != nil || v.Info.DecodedFrames <= 0 || in.Copy.HasAudio && v.AudioSamples <= 0 {
		t.Fatal("real EOF/frame/audio verifier failed", e, v)
	}
	if clip.BrowserAnalysisQualified(in.Profile) {
		t.Fatal("media structure smoke opened semantic qualification")
	}
}
