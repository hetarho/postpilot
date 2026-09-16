package media

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// withoutVerificationOutput is the analysis-copy command as it was before this
// task: the same encode with no verification output beside it. The copy it
// produces has to be the same file, byte for byte.
func withoutVerificationOutput(args []string) []string {
	null := slices.Index(args, "null")
	input := slices.Index(args, "-i")
	if null < 1 || input < 0 || args[null-1] != "-f" || args[null+1] != "-" {
		return args
	}
	return append(append([]string{}, args[:input+2]...), args[null+2:]...)
}

// One decode per original: the copies' own passes measure the source, and what
// they measure is what the separate verification pass measured (CLIP-33,
// CLIP-124). The copies themselves are unchanged (CLIP-125).
func TestPreparationDecodesEachOriginalOnce(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	cfg := mediaConfig(t)
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, length string
		timestamps   string
	}{
		{name: "constant", length: "20"},
		{name: "variable", length: "20", timestamps: "setpts='if(gt(N,150),PTS+0.013/TB,PTS)'"},
		{name: "multi-chunk", length: "61"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := a.WithWorkspace(t.Context(), "prepare-"+tc.name, func(ws clip.MediaWorkspace) error {
				original := filepath.Join(ws.Path, "original.mp4")
				build := []string{"-hide_banner", "-nostdin", "-v", "error",
					"-f", "lavfi", "-i", "color=c=0x2E4C8A:s=640x360:r=30",
					"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
					"-t", tc.length, "-map", "0:v", "-map", "1:a"}
				if tc.timestamps != "" {
					build = append(build, "-vf", tc.timestamps, "-fps_mode", "passthrough")
				}
				build = append(build, "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p", "-c:a", "aac", original)
				if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, build...); err != nil {
					return err
				}
				// What a separate full decode measures, which is what preparation
				// must agree with without decoding the original a second time.
				separate, err := a.Probe(t.Context(), ws, original)
				if err != nil {
					return err
				}
				claimed, err := a.ProbeContainer(t.Context(), ws, original)
				if err != nil {
					return err
				}
				source := clip.MediaSource{Path: original, SourceID: "one", Fingerprint: "hash", Info: claimed}
				var copies []string
				measured, err := a.PrepareAnalysisChunks(t.Context(), ws, source, func(c clip.AnalysisChunk) error {
					copies = append(copies, c.Path)
					return nil
				})
				if err != nil {
					return err
				}
				want := 1
				if tc.name == "multi-chunk" {
					want = 2
				}
				if len(copies) != want {
					t.Fatalf("%d copies, want %d", len(copies), want)
				}
				if measured.DurationMS != separate.DurationMS || measured.DecodedFrames != separate.DecodedFrames || measured.CadenceVerified != separate.CadenceVerified {
					t.Fatalf("preparation measured %dms/%d frames/cadence=%v, the separate pass %dms/%d frames/cadence=%v", measured.DurationMS, measured.DecodedFrames, measured.CadenceVerified, separate.DurationMS, separate.DecodedFrames, separate.CadenceVerified)
				}
				// And the copy itself is what the command without a verification
				// output beside it produces.
				plain := filepath.Join(ws.Path, "plain-0000.mp4")
				chunk := clip.AnalysisChunk{Path: plain, SourceID: "one", Fingerprint: "hash", OffsetMS: 0, DurationMS: min(cfg.ChunkDurationMS, claimed.DurationMS)}
				args := withoutVerificationOutput(a.chunkArgs(source, chunk, cfg.VideoMaxRate, cfg.VideoBufferSize))
				if slices.Contains(args, "vfrdet") {
					t.Fatal("the comparison command kept its verification output")
				}
				if _, err := a.runBounded(t.Context(), ws, cfg.FFmpegPath, plain, cfg.AnalysisMaxBytes, clip.ErrAnalysisTooLarge, args...); err != nil {
					return err
				}
				if fileDigest(t, plain) != fileDigest(t, copies[0]) {
					t.Fatal("the analysis copy moved when the verification output joined its command")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
