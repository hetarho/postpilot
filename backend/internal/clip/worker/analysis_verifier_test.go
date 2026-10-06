package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type analysisFakeMedia struct {
	dir   string
	calls int
}
type analysisTestDownloads struct {
	data      []byte
	downloads map[string]int
	uploaded  []clip.MediaOutput
}

func (a *analysisTestDownloads) Download(_ context.Context, _ clip.MediaLeaseCredentials, slot string, w io.Writer, _ int64) (int64, error) {
	if a.downloads == nil {
		a.downloads = map[string]int{}
	}
	a.downloads[slot]++
	n, e := w.Write(a.data)
	return int64(n), e
}

func (m *analysisFakeMedia) WithWorkspace(ctx context.Context, _ string, run func(clip.MediaWorkspace) error) error {
	return run(clip.MediaWorkspace{Path: m.dir})
}
func (m *analysisFakeMedia) VerifyAnalysisCopy(_ context.Context, _ clip.MediaWorkspace, path string, c clip.AnalysisCopy) (clip.AnalysisCopyVerification, error) {
	m.calls++
	if _, e := os.Stat(path); e != nil {
		return clip.AnalysisCopyVerification{}, e
	}
	return clip.AnalysisCopyVerification{Slot: c.Slot, Bytes: c.Bytes, Digest: c.Digest, Provenance: clip.AnalysisCopyProvenance, VideoPackets: 15, Info: clip.MediaInfo{Width: c.Width, Height: c.Height, DurationMS: 1000, ContainerDurationMS: 1000, VideoDurationMS: 1000, DecodedDurationMS: 1000, FrameRateNumerator: 15, FrameRateDenominator: 1, CadenceVerified: true, DecodedFrames: 15, PixelFormat: "yuv420p", SampleAspectRatio: "1:1", Streams: []clip.MediaStream{{Kind: "video", Codec: "h264"}}}}, nil
}
func TestAnalysisVerifierReadsOnlyBoundedCopiesAndHashesBeforeDecode(t *testing.T) {
	for _, test := range []string{"valid", "digest", "overrun", "contract"} {
		t.Run(test, func(t *testing.T) {
			body := []byte("copy")
			digest := sha256.Sum256(body)
			c := clip.AnalysisCopy{Slot: clip.MediaAnalysisSlot("source", 0), SourceID: "source", Fingerprint: strings.Repeat("a", 64), DurationMS: 1000, Width: 320, Height: 180, Bytes: 4, Digest: hex.EncodeToString(digest[:])}
			task := clip.AnalysisVerificationTask{Version: 1, ProfileVersion: clip.BrowserAnalysisProfileVersion, ManifestDigest: strings.Repeat("b", 64), Copies: []clip.AnalysisCopy{c}}
			if test == "digest" {
				task.Copies[0].Digest = strings.Repeat("c", 64)
			}
			if test == "contract" {
				task.ProfileVersion = "other"
			}
			data, _ := json.Marshal(task)
			work := clip.MediaWork{Operation: clip.MediaVerifyAnalysis, ContractVersion: 3, RendererVersion: clip.AnalysisVerificationRenderer, AssetVersion: clip.AnalysisVerificationAssets, Payload: string(data), InputDigest: clip.MediaPayloadDigest(string(data)), Credentials: clip.MediaLeaseCredentials{AttemptID: "verify"}}
			media := &analysisFakeMedia{dir: t.TempDir()}
			transfer := &analysisTestDownloads{data: body}
			if test == "overrun" {
				transfer.data = []byte("copy-extra")
			}
			verifier := NewAnalysisVerifier(media, transfer, clip.DefaultMediaConfig(clip.Environment{}))
			result, e := verifier.Execute(t.Context(), work)
			if test == "valid" {
				if e != nil || result == "" || media.calls != 1 || transfer.downloads[c.Slot] != 1 {
					t.Fatal(result, e, media.calls, transfer.downloads)
				}
			} else if e == nil || media.calls != 0 {
				t.Fatal("invalid object reached decode", test, e, media.calls)
			}
			if len(transfer.uploaded) != 0 {
				t.Fatal("verification uploaded output")
			}
			if _, e := os.Stat(filepath.Join(media.dir, "copy-0.mp4")); !os.IsNotExist(e) {
				t.Fatal("copy file retained", e)
			}
			for slot := range transfer.downloads {
				if strings.HasPrefix(slot, "source/") {
					t.Fatal("verification fetched an original")
				}
			}
		})
	}
	if nextOperation(clip.MediaVerifyAnalysis) != clip.MediaVerifyAnalysis {
		t.Fatal("verification role entered native rotation")
	}
}
