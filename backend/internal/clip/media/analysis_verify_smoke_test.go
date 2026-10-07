package media

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"github.com/postpilot/backend/internal/clip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func analysisBinaryAdapter(t *testing.T, host bool) *Adapter {
	t.Helper()
	ffmpeg, ffprobe := "/usr/local/bin/ffmpeg", "/usr/local/bin/ffprobe"
	if host {
		ffmpeg, ffprobe = os.Getenv("CLIP_FFMPEG_PATH"), os.Getenv("CLIP_FFPROBE_PATH")
		if ffmpeg == "" || ffprobe == "" {
			t.Fatal("host diagnostic requires explicit binary paths")
		}
	}
	cfg := clip.DefaultMediaConfig(clip.Environment{WorkRoot: t.TempDir(), FFmpegPath: ffmpeg, FFprobePath: ffprobe, WorkStaleAge: time.Hour, MediaTimeout: time.Minute, DecodeThreads: 1})
	var e error
	cfg, e = AnalysisVerificationConfig(cfg)
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(cfg, nil)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func makeAnalysisSmokeCopy(t *testing.T, a *Adapter, ws clip.MediaWorkspace, name string, duration int, audio bool) string {
	t.Helper()
	path := filepath.Join(ws.Path, name+".mp4")
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=176x96:r=15"}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
	}
	args = append(args, "-t", strconv.FormatFloat(float64(duration)/1000, 'f', 3, 64), "-c:v", "libx264", "-pix_fmt", "yuv420p", "-threads", "1")
	if audio {
		args = append(args, "-c:a", "aac", "-ac", "1", "-ar", "48000", "-b:a", "64000")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-movflags", "+faststart", "-y", path)
	if _, e := a.runner.Run(t.Context(), Command{Binary: a.cfg.FFmpegPath, Dir: ws.Path, Args: args}); e != nil {
		t.Fatal("fixture generation failed", e)
	}
	return path
}
func smokeCopyDescriptor(t *testing.T, path string, duration int, audio bool) clip.AnalysisCopy {
	t.Helper()
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(data)
	return clip.AnalysisCopy{Slot: clip.MediaAnalysisSlot("source", 0), SourceID: "source", Fingerprint: hex.EncodeToString(hash[:]), DurationMS: duration, Width: 176, Height: 96, HasAudio: audio, Bytes: int64(len(data)), Digest: hex.EncodeToString(hash[:])}
}
func runAnalysisVerificationBinaryChecks(t *testing.T, host bool) {
	a := analysisBinaryAdapter(t, host)
	e := a.WithWorkspace(t.Context(), "verification-smoke", func(ws clip.MediaWorkspace) error {
		for _, test := range []struct {
			name     string
			duration int
			audio    bool
		}{{"silent", 1000, false}, {"speech", 1001, true}, {"limit", 60000, true}} {
			path := makeAnalysisSmokeCopy(t, a, ws, test.name, test.duration, test.audio)
			c := smokeCopyDescriptor(t, path, test.duration, test.audio)
			measured, e := a.VerifyAnalysisCopy(t.Context(), ws, path, c)
			if e != nil {
				t.Fatalf("%s: %v", test.name, e)
			}
			if measured.Info.DecodedFrames <= 0 || test.audio && measured.AudioSamples <= 0 {
				t.Fatal("actual decoded counts missing")
			}
		}
		long := makeAnalysisSmokeCopy(t, a, ws, "hidden", 65000, false)
		data, e := os.ReadFile(long)
		if e != nil {
			return e
		}
		patchAnalysisDurationHeaders(t, data, 0, len(data))
		if e = os.WriteFile(long, data, 0600); e != nil {
			return e
		}
		info, e := a.ProbeContainer(t.Context(), ws, long)
		if e != nil || info.ContainerDurationMS != 60000 {
			t.Fatalf("attack headers=%+v %v", info, e)
		}
		c := smokeCopyDescriptor(t, long, 60000, false)
		if _, e = a.VerifyAnalysisCopy(t.Context(), ws, long, c); e == nil {
			t.Fatal("65-second packet tail hidden by 60-second headers was accepted")
		}
		bad := filepath.Join(ws.Path, "malformed.mp4")
		if e = os.WriteFile(bad, []byte("not MP4"), 0600); e != nil {
			return e
		}
		c = smokeCopyDescriptor(t, bad, 1000, false)
		if _, e = a.VerifyAnalysisCopy(t.Context(), ws, bad, c); e == nil {
			t.Fatal("malformed copy accepted")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}

// This is the matching-image gate. Host diagnostics remain a separately named
// run and never make an unexecuted image gate pass.
func TestAnalysisCopyVerificationSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("matching execution image only")
	}
	runAnalysisVerificationBinaryChecks(t, false)
}
func TestAnalysisCopyVerificationHostDiagnostic(t *testing.T) {
	if os.Getenv("CLIP_ANALYSIS_HOST_DIAGNOSTIC") != "1" {
		t.Skip("explicit host-only diagnostic")
	}
	runAnalysisVerificationBinaryChecks(t, true)
}

// Keep sample tables/packets intact while shortening the declared movie,
// track, media and edit durations: a clipped decode would stop normally at60s.
func patchAnalysisDurationHeaders(t *testing.T, data []byte, start, end int) {
	t.Helper()
	for start+8 <= end {
		size := int(binary.BigEndian.Uint32(data[start : start+4]))
		kind := string(data[start+4 : start+8])
		header := 8
		if size == 1 {
			if start+16 > end {
				t.Fatal("invalid extended box")
			}
			size = int(binary.BigEndian.Uint64(data[start+8 : start+16]))
			header = 16
		}
		if size < header || start+size > end {
			t.Fatal("invalid MP4 diagnostic box")
		}
		body := start + header
		switch kind {
		case "moov", "trak", "mdia", "edts":
			patchAnalysisDurationHeaders(t, data, body, start+size)
		case "mvhd", "mdhd":
			if data[body] != 0 {
				t.Fatal("diagnostic expects v0 durations")
			}
			scale := binary.BigEndian.Uint32(data[body+12 : body+16])
			binary.BigEndian.PutUint32(data[body+16:body+20], 60*scale)
		case "tkhd":
			binary.BigEndian.PutUint32(data[body+20:body+24], 60000)
		case "elst":
			if data[body] != 0 || binary.BigEndian.Uint32(data[body+4:body+8]) != 1 {
				t.Fatal("diagnostic expects one v0 edit")
			}
			binary.BigEndian.PutUint32(data[body+8:body+12], 60000)
		}
		start += size
	}
}
