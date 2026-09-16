package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// The baseline the speed work must hold: the delivered clip of one fixed plan,
// recorded against the pinned media image (CLIP-125). A change that only moves
// work around must reproduce it byte for byte; a deliberate change to the
// picture re-records it and says so.
const deliveredIdentityBaseline = `{
 "digest": "ba5a3fb91b6787cc5c1be4347967a599b427449f1c2eeb3f72c5fcd1ee6edeae",
 "frames": [
  "5e284929898c54b52f0a3eac5754528ef738c729ff8ff6c00af8b0f2c73e4c27",
  "a8c964918b6d020000595001b0a9e791256414e688b9715eacfd1c8958ce021f",
  "4d183d5a381dfa5ac1ac2ec35111bbe50bb2439162e1ad421c4f0aeb4f31dd06",
  "0cb6971344729bc23cab84b1d192744d087dee8a4d5efcc20bf2745c6e313476",
  "4cc212cb6e1a8cff4099fa4b8ca9024b1603123acb94b9b1ebe5638345e264bb",
  "5872c831e7949a7110976816f61adaf3b0aa6a74294017e74b767faaa5198529",
  "debcc569b18614373fdde59224918595702b0a84d85b99f92e1ab5658821c193",
  "80c778a536a9ed668c785fa69bb69a9165b2c276d7cb1ee18d97e8fd1b3382b4",
  "bd8fb12dddd9672ac1bce29609a7fd0ac3c02dfae5874a88d08f23ae7b027aec",
  "386925233ac1de5f33e59e2100da93ee9ee62f11eb2a5758e8300418e30ee76a",
  "4c7c460edae87eca9a63cdb8437f4ce686733b207827da30ee6f4046b50b424e",
  "a98438b611c35defae87202b7005138e4093ba6043b8ce4e1d3531155e9aba79",
  "c34ab421a37aeed1ce95a161ece954504309e7124eb6cb9ad30401b6d7361fe3",
  "3aba4bf1e5aaf06ff60517c1c219306f14b9335c1218f6dd2416ab353e9f7314",
  "a3ea3729dd97d76b4e6fed5be2b0dae1cef552c4ceb53b529392f79f6012f4b9",
  "bbbefe1f3fa7436410e2aeb67bf310b00d1b7a46ce88560fb11ee90a30862c4f"
 ],
 "properties": {
  "audio.channels": "2",
  "audio.codec": "aac",
  "audio.frames": "742",
  "audio.sample_rate": "48000",
  "container": "mov,mp4,m4a,3gp,3g2,mj2",
  "duration": "15.800000",
  "video.codec": "h264",
  "video.frame_rate": "30/1",
  "video.frames": "474",
  "video.pixel_format": "yuv420p",
  "video.profile": "High",
  "video.size": "1080x1920"
 }
}`

// What identity means for a delivered clip: the bytes themselves, the
// properties a player reads, and one frame per second of output. The frames are
// there so a mismatch names the second that moved rather than only an unequal
// digest (CLIP-97).
type deliveredFingerprint struct {
	Digest     string            `json:"digest"`
	Properties map[string]string `json:"properties"`
	Frames     []string          `json:"frames"`
}

func (want deliveredFingerprint) differences(got deliveredFingerprint) []string {
	var out []string
	keys := []string{}
	for key := range want.Properties {
		keys = append(keys, key)
	}
	for key := range got.Properties {
		if _, ok := want.Properties[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		if want.Properties[key] != got.Properties[key] {
			out = append(out, fmt.Sprintf("property %s: want %q got %q", key, want.Properties[key], got.Properties[key]))
		}
	}
	if len(want.Frames) != len(got.Frames) {
		out = append(out, fmt.Sprintf("sampled frames: want %d got %d", len(want.Frames), len(got.Frames)))
	}
	for i := 0; i < min(len(want.Frames), len(got.Frames)); i++ {
		if want.Frames[i] != got.Frames[i] {
			out = append(out, fmt.Sprintf("frame at %ds: want %s got %s", i, short(want.Frames[i]), short(got.Frames[i])))
		}
	}
	if len(out) == 0 && want.Digest != got.Digest {
		out = append(out, fmt.Sprintf("container bytes differ where no sampled frame or property does: want %s got %s", short(want.Digest), short(got.Digest)))
	}
	return out
}
func short(digest string) string {
	if len(digest) <= 12 {
		return digest
	}
	return digest[:12]
}

func TestDeliveredClipIdentity(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	cfg := mediaConfig(t)
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// The release QA's first case is already the plan this task asks for: three
	// cuts, the second fading in over 200 ms and the others hard, the authored
	// overlay elements above them, the owner's source sound on, one ratio.
	plan := qaClipCases()[0]
	dir := t.TempDir()
	var sources []clip.RenderSource
	var info clip.MediaInfo
	originals := map[string]string{}
	if err := a.WithWorkspace(t.Context(), "identity-footage", func(ws clip.MediaWorkspace) error {
		built, paths, probed, err := qaFootage(t, a, ws, cfg)
		if err != nil {
			return err
		}
		sources, info = built, probed
		for id, path := range paths {
			held := filepath.Join(dir, id+".mp4")
			if err := copyIdentityFile(path, held); err != nil {
				return err
			}
			originals[id] = held
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "first.mp4")
	second := filepath.Join(dir, "second.mp4")
	renderIdentityClip(t, a, r, plan, sources, info, originals, "identity-1", first)
	renderIdentityClip(t, a, r, plan, sources, info, originals, "identity-2", second)
	var got, repeat deliveredFingerprint
	if err := a.WithWorkspace(t.Context(), "identity-fingerprint", func(ws clip.MediaWorkspace) error {
		got = fingerprintDelivered(t, a, ws, dir, first)
		repeat = fingerprintDelivered(t, a, ws, dir, second)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if d := got.differences(repeat); len(d) > 0 {
		t.Fatalf("one fixed plan rendered twice did not deliver the same clip, so no baseline can be recorded:\n%s", strings.Join(d, "\n"))
	}
	var want deliveredFingerprint
	if err := json.Unmarshal([]byte(deliveredIdentityBaseline), &want); err != nil {
		t.Fatal(err)
	}
	if d := want.differences(got); len(d) > 0 {
		recorded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("the delivered clip left its baseline:\n%s\n\nrecord this in deliveredIdentityBaseline only for a deliberate change to the picture:\n%s", strings.Join(d, "\n"), recorded)
	}
}

func renderIdentityClip(t *testing.T, a *Adapter, r *Rendering, c qaClip, sources []clip.RenderSource, info clip.MediaInfo, originals map[string]string, job, dest string) {
	t.Helper()
	// The plan is rebuilt for each render: the renderer lays out and repairs the
	// one it is handed, so a shared value would make the second render start
	// from the first one's result.
	plan, err := qaPlan(c, sources)
	if err != nil {
		t.Fatal(err)
	}
	laidOut, _, err := r.Layout(t.Context(), plan, sources)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), job, func(ws clip.MediaWorkspace) error {
		result, err := r.Render(t.Context(), ws, laidOut, sources, func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
			held, ok := originals[id]
			if !ok {
				return clip.ErrNotFound
			}
			path := filepath.Join(ws.Path, "source-"+id+".mp4")
			if err := copyIdentityFile(held, path); err != nil {
				return err
			}
			defer os.Remove(path)
			return consume(clip.MediaSource{SourceID: id, Fingerprint: id, Info: info, Path: path})
		})
		if err != nil {
			return err
		}
		return copyIdentityFile(result.Path, dest)
	}); err != nil {
		t.Fatal(err)
	}
}

func fingerprintDelivered(t *testing.T, a *Adapter, ws clip.MediaWorkspace, dir, path string) deliveredFingerprint {
	t.Helper()
	return deliveredFingerprint{Digest: fileDigest(t, path), Properties: deliveredProperties(t, a, ws, path), Frames: deliveredFrames(t, a, ws, dir, path)}
}

func deliveredProperties(t *testing.T, a *Adapter, ws clip.MediaWorkspace, path string) map[string]string {
	t.Helper()
	data, err := a.run(t.Context(), ws, a.cfg.FFprobePath, "-hide_banner", "-v", "error", "-print_format", "json", "-show_streams", "-show_format", path)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Profile    string `json:"profile"`
			PixFmt     string `json:"pix_fmt"`
			RFrameRate string `json:"r_frame_rate"`
			SampleRate string `json:"sample_rate"`
			NbFrames   string `json:"nb_frames"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{"container": probe.Format.FormatName, "duration": probe.Format.Duration}
	for _, s := range probe.Streams {
		switch s.CodecType {
		case "video":
			out["video.codec"], out["video.profile"] = s.CodecName, s.Profile
			out["video.size"] = fmt.Sprintf("%dx%d", s.Width, s.Height)
			out["video.pixel_format"], out["video.frame_rate"], out["video.frames"] = s.PixFmt, s.RFrameRate, s.NbFrames
		case "audio":
			out["audio.codec"], out["audio.sample_rate"] = s.CodecName, s.SampleRate
			out["audio.channels"], out["audio.frames"] = fmt.Sprint(s.Channels), s.NbFrames
		}
	}
	return out
}

// One frame per second of output, through the image2 muxer: the production
// image has no framemd5 or md5 muxer, and its filters include fps but not
// select, so a second is the finest position this check can name.
func deliveredFrames(t *testing.T, a *Adapter, ws clip.MediaWorkspace, dir, path string) []string {
	t.Helper()
	frames := filepath.Join(dir, "frames-"+filepath.Base(path))
	if err := os.MkdirAll(frames, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(frames)
	if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-hide_banner", "-nostdin", "-v", "error", "-i", path, "-vf", "fps=1", "-an", "-f", "image2", filepath.Join(frames, "frame-%04d.png")); err != nil {
		t.Fatal(err)
	}
	written, err := filepath.Glob(filepath.Join(frames, "frame-*.png"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(written)
	out := make([]string, 0, len(written))
	for _, frame := range written {
		out = append(out, fileDigest(t, frame))
	}
	return out
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func copyIdentityFile(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(dest, source)
	return errorsJoinClose(err, dest)
}

// The readable half of the check, without a render: a mismatch names the
// property that moved and the second whose frame moved, and bytes that move
// where neither does are still reported rather than passed.
func TestIdentityMismatchNamesTheFrameAndTheProperty(t *testing.T) {
	want := deliveredFingerprint{Digest: "aaaa000000000000", Properties: map[string]string{"duration": "15.800000", "video.size": "1080x1920"}, Frames: []string{"f0", "f1", "f2"}}
	moved := deliveredFingerprint{Digest: "bbbb000000000000", Properties: map[string]string{"duration": "15.800000", "video.size": "720x1280"}, Frames: []string{"f0", "changed", "f2"}}
	d := want.differences(moved)
	if len(d) != 2 || !strings.Contains(d[0], `property video.size: want "1080x1920" got "720x1280"`) || !strings.Contains(d[1], "frame at 1s") {
		t.Fatalf("unreadable difference: %v", d)
	}
	quiet := deliveredFingerprint{Digest: "cccc000000000000", Properties: want.Properties, Frames: want.Frames}
	if d := want.differences(quiet); len(d) != 1 || !strings.Contains(d[0], "container bytes differ") {
		t.Fatalf("a moved byte went unreported: %v", d)
	}
	if d := want.differences(want); len(d) != 0 {
		t.Fatalf("identical fingerprints differ: %v", d)
	}
}

// The other half of CLIP-125 for the decode split: an analysis copy is produced
// by decoding an original and encoding a proxy, and only the decoder's thread
// count changes here. Decoding is bit-exact whatever that count is, so the
// proxies have to come out identical.
func TestAnalysisCopyIdentityAcrossDecoderThreads(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	cfg := mediaConfig(t)
	if cfg.DecodeThreads < 2 {
		t.Fatalf("nothing to compare at %d decoder threads", cfg.DecodeThreads)
	}
	prepare := func(threads int) []string {
		t.Helper()
		with := cfg
		with.DecodeThreads = threads
		a, err := New(with, nil)
		if err != nil {
			t.Fatal(err)
		}
		var digests []string
		if err := a.WithWorkspace(t.Context(), fmt.Sprintf("proxy-%d", threads), func(ws clip.MediaWorkspace) error {
			original := filepath.Join(ws.Path, "original.mp4")
			if _, err := a.run(t.Context(), ws, with.FFmpegPath, "-hide_banner", "-nostdin", "-v", "error",
				"-f", "lavfi", "-i", "color=c=0x2E4C8A:s=1280x720:r=30",
				"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
				"-t", "20", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p",
				"-c:a", "aac", original); err != nil {
				return err
			}
			info, err := a.Probe(t.Context(), ws, original)
			if err != nil {
				return err
			}
			return a.PrepareAnalysisChunks(t.Context(), ws, clip.MediaSource{Path: original, SourceID: "one", Fingerprint: "hash", Info: info}, func(chunk clip.AnalysisChunk) error {
				digests = append(digests, fileDigest(t, chunk.Path))
				return nil
			})
		}); err != nil {
			t.Fatal(err)
		}
		return digests
	}
	single, parallel := prepare(1), prepare(cfg.DecodeThreads)
	if len(single) == 0 || !slices.Equal(single, parallel) {
		t.Fatalf("the analysis copies moved with the decoder thread count: %v vs %v", single, parallel)
	}
}
