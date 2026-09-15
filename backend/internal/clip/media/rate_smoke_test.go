package media

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// dominantHz estimates the strongest frequency in one PCM window by counting
// zero crossings. It is coarse on purpose: what it has to tell apart is a tone
// that kept its pitch from one that was resampled up or down by the playback
// rate, which is a factor of two, not a few hertz.
func dominantHz(data []byte, rate int) float64 {
	samples := len(data) / 2
	if samples < 2 {
		return 0
	}
	crossings, previous := 0, float64(int16(binary.LittleEndian.Uint16(data)))
	for i := 2; i+1 < len(data); i += 2 {
		v := float64(int16(binary.LittleEndian.Uint16(data[i:])))
		if previous < 0 && v >= 0 {
			crossings++
		}
		previous = v
	}
	return float64(crossings) * float64(rate) / float64(samples)
}

// The real renderer, the real binaries: every supported rate materialized from
// original footage, with the owner's source-sound settings deciding what is
// heard (CLIP-98, CDS-6, CDS-35, CDS-68).
func TestRenderSmokeRatesAndSourceAudio(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 15 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	err = a.WithWorkspace(t.Context(), "rates", func(ws clip.MediaWorkspace) error {
		// 60 fps so the slow rates have the cadence CDS-68 asks for, and a
		// steady 440 Hz tone so pitch preservation is measurable.
		loud := filepath.Join(ws.Path, "loud.mp4")
		if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=60", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "40", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", "-r", "60", "-c:a", "aac", loud); err != nil {
			return err
		}
		quiet := filepath.Join(ws.Path, "quiet.mp4")
		if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=60", "-t", "40", "-c:v", "libx264", "-preset", "ultrafast", "-threads", "1", "-pix_fmt", "yuv420p", "-r", "60", "-c:a", "aac", quiet); err != nil {
			return err
		}
		loudInfo, err := a.Probe(t.Context(), ws, loud)
		if err != nil {
			return err
		}
		quietInfo, err := a.Probe(t.Context(), ws, quiet)
		if err != nil {
			return err
		}
		if !loudInfo.HasAudio || quietInfo.HasAudio {
			return fmt.Errorf("fixtures do not carry the audio the matrix needs")
		}
		sources := []clip.RenderSource{{ID: "loud", Fingerprint: "loud", Info: loudInfo}, {ID: "quiet", Fingerprint: "quiet", Info: quietInfo}}
		load := func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
			source := clip.MediaSource{SourceID: "loud", Fingerprint: "loud", Info: loudInfo, Path: loud}
			if id == "quiet" {
				source = clip.MediaSource{SourceID: "quiet", Fingerprint: "quiet", Info: quietInfo, Path: quiet}
			}
			return consume(source)
		}
		for _, audio := range []string{"on", "off", "mixed", "none"} {
			t.Run(audio, func(t *testing.T) {
				// Six adjacent cuts off the SAME source, one per rate, each
				// three seconds of output: 18 s of clip in total.
				plan := clip.EditPlan{Ratio: "vertical", DurationMS: 18000, Disclosure: "ad", Preset: "restaurant",
					Facts: []clip.Answer{{Label: "상호", Text: "속도 테스트"}}}
				start := 0
				settings := &clip.SourceAudioSettings{}
				for i, rate := range clip.PlaybackRates() {
					span := 3000 * rate / clip.RateUnitPermille
					id := "loud"
					if audio == "none" || audio == "mixed" && i%2 == 1 {
						id = "quiet"
					}
					plan.Cuts = append(plan.Cuts, clip.Cut{ID: fmt.Sprint("cut-", i), SourceID: id, Fingerprint: id,
						StartMS: start, EndMS: start + span, PlaybackRatePermille: rate,
						Focal: clip.Point{X: .5, Y: .5}, Volume: volume(1)})
					start += span
				}
				// The snapshot is exactly the identities the cuts use: a
				// foreign source is as invalid as a forgotten one (CDS-35).
				used := []string{}
				for _, cut := range plan.Cuts {
					if !slices.Contains(used, cut.SourceID) {
						used = append(used, cut.SourceID)
					}
				}
				for _, key := range used {
					retain := audio == "on" || audio == "mixed"
					settings.Values = append(settings.Values, clip.SourceAudioSetting{SourceID: key, Fingerprint: key, RetainOriginal: retain})
				}
				plan.SourceAudio = settings
				result, err := r.Render(t.Context(), ws, plan, sources, load)
				if err != nil {
					t.Fatalf("%s: %v", audio, err)
				}
				// V12's delivered length is the decoded video track, within one
				// output frame of the transformed timeline.
				delivered := float64(result.Info.DecodedFrames) * 1000 / 30
				if math.Abs(delivered-18000) > 1000/30.0 {
					t.Fatalf("%s: delivered %.1f ms of %d frames, want 18000", audio, delivered, result.Info.DecodedFrames)
				}
				// An audio track exists only where an enabled source carries one.
				wantAudio := audio == "on" || audio == "mixed"
				if result.Info.HasAudio != wantAudio {
					t.Fatalf("%s: audio track present=%v, want %v", audio, result.Info.HasAudio, wantAudio)
				}
				if !wantAudio {
					if err := os.Remove(result.Path); err != nil {
						t.Fatal(err)
					}
					return
				}
				// Pitch survives every rate: each cut's tone is still 440 Hz,
				// not 220 or 880.
				for i, at := range []string{"1", "4", "7", "10", "13", "16"} {
					data, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-ss", at, "-i", result.Path, "-t", "0.2", "-vn", "-ac", "1", "-ar", "48000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
					if err != nil {
						t.Fatal(err)
					}
					hz := dominantHz(data, 48000)
					if audio == "mixed" && i%2 == 1 {
						if pcmPeak(data) > 2 {
							t.Fatal("an enabled source without audio contributed sound")
						}
						continue
					}
					if hz < 400 || hz > 480 {
						t.Fatalf("%s: cut %d plays at %.0f Hz, want about 440", audio, i, hz)
					}
				}
				if err := os.Remove(result.Path); err != nil {
					t.Fatal(err)
				}
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func pcmPeak(data []byte) int {
	peak := 0
	for i := 0; i+1 < len(data); i += 2 {
		value := int(int16(binary.LittleEndian.Uint16(data[i:])))
		if value < 0 {
			value = -value
		}
		peak = max(peak, value)
	}
	return peak
}

// Aggregate frame count and average rate alone cannot distinguish CFR from VFR.
// These originals have real timestamps, including VFR that averages above 40 fps.
func TestMediaSmokeCadenceAdmission(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real cadence gate runs inside Docker")
	}
	cfg := mediaConfig(t)
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		fps  int
		vfr  bool
		want []int
	}{
		{"cfr30", 30, false, []int{1000, 1250, 1500, 2000}},
		{"cfr40", 40, false, []int{750, 1000, 1250, 1500, 2000}},
		{"cfr60", 60, false, clip.PlaybackRates()},
		{"vfr", 60, true, []int{1000, 1250, 1500, 2000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := a.WithWorkspace(t.Context(), tc.name, func(ws clip.MediaWorkspace) error {
				path := filepath.Join(ws.Path, "original.mp4")
				args := []string{"-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("color=c=blue:s=320x240:r=%d", tc.fps), "-frames:v", fmt.Sprint(tc.fps * 4)}
				if tc.vfr {
					args = append(args, "-vf", "setpts=if(lt(N\\,120)\\,N/(40*TB)\\,(3+(N-120)/60)/TB)", "-fps_mode", "vfr")
				}
				args = append(args, "-c:v", "libx264", "-threads", "1", "-preset", "ultrafast", "-pix_fmt", "yuv420p", path)
				if _, err := a.run(t.Context(), ws, cfg.FFmpegPath, args...); err != nil {
					return err
				}
				info, err := a.Probe(t.Context(), ws, path)
				if err != nil {
					return err
				}
				if info.CadenceVerified == tc.vfr {
					return fmt.Errorf("wrong decoded cadence evidence: %+v", info)
				}
				if got := clip.AllowedPlaybackRates(info); !slices.Equal(got, tc.want) {
					return fmt.Errorf("cadence admitted %v, want %v: %+v", got, tc.want, info)
				}
				info.DecodedFrames = 0
				if slices.Contains(clip.AllowedPlaybackRates(info), 750) {
					return fmt.Errorf("unmeasured original admitted slow playback")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
