package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/platform/config"
)

func renderConfig(t *testing.T) clip.RenderConfig {
	t.Helper()
	// Both faces, from the same variables the image sets (CDS-17): the renderer
	// refuses to start without the display face, so a config that names only
	// Pretendard fails the constructor rather than any render.
	font := path("CLIP_FONT_PATH", "/usr/share/postpilot-fonts/pretendard/PretendardVariable.ttf")
	display := path("CLIP_FONT_PAPERLOGY_PATH", "/usr/share/postpilot-fonts/paperlogy/Paperlogy-8ExtraBold.ttf")
	return config.ClipRender(&config.Config{ClipResvgPath: "/usr/local/bin/resvg", ClipFontPath: font, ClipDisplayFontPath: display})
}
func path(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func TestRenderSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "glyphs", func(ws clip.MediaWorkspace) error {
		semibold, err := r.measure(t.Context(), ws, []string{"한글 여행 W"}, 600, 0, fontFamily)
		if err != nil {
			return err
		}
		bold, err := r.measure(t.Context(), ws, []string{"한글 여행 W"}, 800, 0, fontFamily)
		if err != nil {
			return err
		}
		if semibold["한글 여행 W"] == bold["한글 여행 W"] {
			t.Fatal("variable font weight was ignored")
		}
		canvas, _ := clip.ClipCanvas("vertical")
		copy := clip.Copy{Text: "한글 여행", Style: "clean", Anchor: "bottom", Align: "center"}
		layout, err := r.layoutCopy(t.Context(), ws, canvas, copy)
		if err != nil {
			return err
		}
		plate, err := r.copyPlate(t.Context(), ws, canvas, copy, layout, 0, Luminance{})
		if err != nil {
			return err
		}
		img, err := readPNG(plate)
		if err != nil {
			return err
		}
		_, _, _, alpha := img.At(0, 0).RGBA()
		if alpha != 0 {
			t.Fatal("copy plate background is not transparent")
		}
		// Every style, rasterized by the real resvg against the real font, is
		// checked in pixels: a plated style paints its plate token and its accent
		// where CDS puts it, an unplated one paints the stroke instead, and
		// 형광펜's highlight sits where the measured advance puts it (CDS-23..26).
		amber := design.Accent["amber"]
		for style, rule := range design.Styles {
			c := clip.Copy{Text: "가격 9900원", Keyword: "9900원", Style: style, Anchor: rule.Anchor, Align: rule.Align, Accent: "amber"}
			l, err := r.layoutCopy(t.Context(), ws, canvas, c)
			if err != nil {
				return fmt.Errorf("%s: %w", style, err)
			}
			path, err := r.copyPlate(t.Context(), ws, canvas, c, l, 1, Luminance{})
			if err != nil {
				return fmt.Errorf("%s: %w", style, err)
			}
			img, err := readPNG(path)
			if err != nil {
				return err
			}
			p := l.Region
			if rule.Plate != "" {
				// A point on the plate's top edge, inside its own padding and
				// clear of the corner radius: only the plate can have painted it.
				token := design.Color[rule.Plate]
				red, _, _, alpha := img.At(int(p.X+p.Width/2), int(p.Y+2)).RGBA()
				want := uint32(math.Round(token.Alpha * 0xffff))
				if alpha < want-0x300 || alpha > want+0x300 {
					return fmt.Errorf("%s plate alpha %d want %d", style, alpha, want)
				}
				// Premultiplied by that alpha: ink reads dark, paper reads light.
				if dark := red < alpha/2; dark != (rule.Plate == "ink_900") {
					return fmt.Errorf("%s plate colour %d over alpha %d", style, red, alpha)
				}
				// The accent bar runs the plate's full height at its left edge;
				// the dot sits inside the top-left padding instead.
				x, y := int(p.X+2), int(p.Y+p.Height/2)
				if rule.Dot {
					x, y = int(p.X+rule.Padding.H+design.Spacing.DotAccent/2), int(p.Y+rule.Padding.V+design.Spacing.DotAccent/2)
				}
				if !isAccent(img, x, y) {
					return fmt.Errorf("%s is missing its accent at %d,%d", style, x, y)
				}
				continue
			}
			// The stroke under the fill is the only thing an unplated style paints
			// nearly opaque and nearly black; the shadow is softer than α0.85.
			if !scan(img, p, func(r, g, b, a uint32) bool {
				return a > 0xd000 && r < 0x3000 && g < 0x3000 && b < 0x3000
			}) {
				return fmt.Errorf("%s painted no %s stroke", style, rule.Stroke)
			}
			if style == "simple" {
				if scan(img, p, func(r, g, b, a uint32) bool { return a > 0x8000 && r > 2*b }) {
					return fmt.Errorf("simple coloured a keyword")
				}
				continue
			}
			if !rule.Highlight {
				// 크게 강조 colours the word itself, so the accent is on a glyph.
				if !scan(img, p, func(r, g, b, a uint32) bool { return a > 0x8000 && r > 2*b }) {
					return fmt.Errorf("%s did not colour its accent word %s", style, amber)
				}
				continue
			}
			u := design.Spacing.UnderlineMark
			x := int(p.X + l.Keyword.Offset + l.Keyword.Width/2)
			y := int(p.Y + (1-u.RaiseEM-u.HeightEM/2)*l.FontSize)
			if !isAccent(img, x, y) {
				return fmt.Errorf("%s highlight is missing at %d,%d", style, x, y)
			}
			// It is BEHIND the keyword, not before or after it.
			if isAccent(img, int(p.X+2), y) {
				return fmt.Errorf("%s highlighted the whole line", style)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"vertical", "horizontal", "square", "silent-rounded", "audio-rounded", "caption-timed", "bright-scrim"} {
		t.Run(variant, func(t *testing.T) {
			ratio := variant
			if variant == "silent-rounded" || variant == "audio-rounded" || variant == "caption-timed" {
				ratio = "square"
			}
			if variant == "bright-scrim" {
				ratio = "vertical"
			}
			err := a.WithWorkspace(t.Context(), ratio, func(ws clip.MediaWorkspace) error {
				infos := map[string]clip.MediaInfo{}
				// No source is retained between callbacks; the fixture loader creates
				// exactly one file at a time, like the streaming T076 object adapter.
				load := func(ctx context.Context, id string, consume func(clip.MediaSource) error) error {
					path := filepath.Join(ws.Path, "fixture.mp4")
					defer os.Remove(path)
					// CDS-44 only has something to measure on bright footage, so
					// that one variant is shot on white.
					colour := "blue"
					if variant == "bright-scrim" {
						colour = "white"
					}
					args := []string{"-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=" + colour + ":s=1280x720:r=30"}
					if id != "silent" {
						args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
					}
					length := "6"
					if variant == "silent-rounded" || variant == "audio-rounded" || variant == "caption-timed" || variant == "bright-scrim" {
						length = "16"
					}
					args = append(args, "-t", length, "-c:v", "libx264", "-preset", "ultrafast", "-threads", "2", "-pix_fmt", "yuv420p")
					if id != "silent" {
						args = append(args, "-c:a", "aac")
					}
					args = append(args, path)
					if _, err := a.run(ctx, ws, a.cfg.FFmpegPath, args...); err != nil {
						return err
					}
					if id == "rotated" {
						rotated := filepath.Join(ws.Path, "fixture-rotated.mov")
						defer os.Remove(rotated)
						if _, err := a.run(ctx, ws, a.cfg.FFmpegPath, "-v", "error", "-display_rotation:v:0", "90", "-i", path, "-c", "copy", rotated); err != nil {
							return err
						}
						if err := os.Remove(path); err != nil {
							return err
						}
						path = rotated
					}
					info, err := a.Probe(ctx, ws, path)
					if err != nil {
						return err
					}
					return consume(clip.MediaSource{Path: path, SourceID: id, Fingerprint: id, Info: info})
				}
				var sources []clip.RenderSource
				for _, id := range []string{"audio", "rotated", "silent"} {
					if err := load(t.Context(), id, func(s clip.MediaSource) error { infos[id] = s.Info; return nil }); err != nil {
						return err
					}
					sources = append(sources, clip.RenderSource{ID: id, Fingerprint: id, Info: infos[id]})
				}
				// Every clip carries its disclosure badge, and the first cut
				// carries the chips its facts earn (CDS-5, CDS-30).
				// The clip opens on a hook card and closes on a CTA card
				// (CDS-28, CDS-29). The last cut's copy leaves before the
				// ending card arrives, because nothing shows under a card
				// (CDS-45) — which the verifier would otherwise refuse.
				// The second cut joins with a hard cut and the third fades, so
				// one render exercises both boundaries CDS-36 admits and the
				// audio has to stay locked to the picture across each of them.
				plan := clip.EditPlan{Ratio: ratio, DurationMS: 15200, Disclosure: "ad", Preset: "restaurant", Hook: "정확한 한글", Accent: "coral", Facts: []clip.Answer{
					{Label: "상호", Text: "연남 김밥"}, {Label: "위치", Text: "서울 연남동"}, {Label: "가격", Text: "9,900원"},
				}, Cuts: []clip.EditCut{
					{ID: "one", SourceID: "audio", Fingerprint: "audio", EndMS: 5200, Focal: clip.Point{X: .5, Y: .5}, Chips: []string{"위치", "가격"}, Copies: []clip.Copy{{Text: "정확한 한글 & 여행", Style: "clean", Anchor: "bottom", Align: "center", Accent: "coral"}}},
					{ID: "two", SourceID: "rotated", Fingerprint: "rotated", EndMS: 5000, Focal: clip.Point{X: .5, Y: .5}, Volume: volume(.5), Copies: []clip.Copy{{Text: "기록처럼 <오늘>", Style: "memo", Anchor: "lower_mid", Align: "left", Accent: "teal"}}},
					{ID: "three", SourceID: "silent", Fingerprint: "silent", EndMS: 5200, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Volume: volume(0), Copies: []clip.Copy{{Text: "다시 오고 싶은 곳", Style: "bold", Anchor: "upper_mid", Align: "center", Accent: "amber", StartMS: 120, EndMS: 2400}}},
				}}
				if variant != "vertical" && variant != "horizontal" && variant != "square" {
					// One cut, no cards: these variants are about timing and
					// rounding, and a card over the only cut would cover the
					// copy they measure.
					plan.Hook, plan.Facts = "", nil
					plan.Disclosure = "ad"
				}
				if variant == "silent-rounded" || variant == "audio-rounded" {
					plan.DurationMS = 15017
					if variant == "silent-rounded" {
						plan.Cuts = plan.Cuts[2:]
					} else {
						plan.Cuts = plan.Cuts[:1]
					}
					plan.Cuts[0].EndMS = 15017
					plan.Cuts[0].Copies[0].Text = ""
				}
				if variant == "bright-scrim" {
					// One 형광펜 cut on white footage: the sampler has to find a
					// bright ground and the scrim has to reach the pixels.
					plan.Cuts = plan.Cuts[:1]
					plan.Cuts[0].EndMS = 15000
					plan.Cuts[0].Copies = []clip.Copy{{Text: "가격 9900원", Keyword: "9900원", Style: "mark", Anchor: "bottom", Align: "center", Accent: "amber"}}
					plan.Cuts[0].Chips = nil
				}
				if variant == "caption-timed" {
					plan.Cuts = plan.Cuts[:1]
					plan.Cuts[0].EndMS = 15000
					plan.Cuts[0].Copies[0].StartMS = 4000
					plan.Cuts[0].Copies[0].EndMS = 10000
					width, height, err := r.CaptionSize(t.Context(), ratio, plan.Cuts[0].FirstCopy())
					if err != nil || width <= 0 || height <= 0 {
						return fmt.Errorf("measure timed caption: %v", err)
					}
				}
				// Whatever the variant kept, the first cut leads in from nothing
				// and the declared duration is the footage less its overlaps.
				plan.Cuts[0].TransitionMS = 0
				selected := 0
				for _, c := range plan.Cuts {
					selected += c.EndMS - c.StartMS
				}
				plan.DurationMS = selected - plan.TransitionTotal()
				result, err := r.Render(t.Context(), ws, plan, sources, load)
				if err != nil {
					return err
				}
				if math.Abs(float64(result.Info.DurationMS-plan.DurationMS)) > 1000.0/30 || result.Info.HasAudio != (variant != "silent-rounded") {
					t.Fatalf("result=%+v", result)
				}
				// V12 on the delivered file: 30 fps, H.264 High and 48 kHz AAC,
				// and the track measured again at CDS-35's −16 LUFS ±1. Render
				// already refuses a miss; this says what the miss would be.
				for _, stream := range result.Info.Streams {
					if stream.Kind == "video" && (stream.Codec != "h264" || stream.Profile != "High") {
						t.Fatalf("V12 video: %+v", stream)
					}
				}
				if result.Info.FrameRateNumerator != 30*result.Info.FrameRateDenominator {
					t.Fatalf("V12 frame rate: %d/%d", result.Info.FrameRateNumerator, result.Info.FrameRateDenominator)
				}
				if result.Info.HasAudio {
					if result.Info.AudioRate != 48000 {
						t.Fatalf("V12 sample rate: %d", result.Info.AudioRate)
					}
					measured, err := r.measureLoudness(t.Context(), ws, result.Path)
					if err != nil {
						return err
					}
					if !measured.Silent && math.Abs(measured.I-design.Audio.Loudnorm.I) > 1 {
						t.Fatalf("%s delivered %.2f LUFS, not %.1f ±1", variant, measured.I, design.Audio.Loudnorm.I)
					}
				}
				fileBytes, err := os.ReadFile(result.Path)
				if err != nil {
					return err
				}
				moov, mdat := bytes.Index(fileBytes, []byte("moov")), bytes.Index(fileBytes, []byte("mdat"))
				if moov < 0 || mdat < 0 || moov > mdat {
					t.Fatal("MP4 is not fast-start")
				}
				if variant == "silent-rounded" || variant == "audio-rounded" {
					return nil
				}
				if variant == "bright-scrim" {
					canvas, _ := clip.ClipCanvas(ratio)
					l, _ := design.Layout(ratio)
					// The manifest records what the sampler decided, so the
					// scrim is there to be found before any pixel is read.
					scrim := clip.Region{}
					for _, e := range result.Manifest {
						if e.Kind == "scrim" {
							scrim = clip.Region(e.Region)
						}
					}
					if scrim != clip.Region(l.ScrimBottom) {
						return fmt.Errorf("white footage did not earn CDS-32's bottom scrim: %+v", result.Manifest)
					}
					path := filepath.Join(ws.Path, "scrim.png")
					if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-ss", "7", "-i", result.Path, "-frames:v", "1", "-c:v", "png", "-threads", "1", path); err != nil {
						return err
					}
					frame, err := readPNG(path)
					if err != nil {
						return err
					}
					// The gradient is 0 at the band's top edge and 0.55 at the
					// bottom, so the frame darkens down the band and the white
					// above it is untouched.
					above := blueAt(frame, canvas.Width/2, int(scrim.Y)-40)
					low := blueAt(frame, 20, int(scrim.Y+scrim.Height)-4)
					if above < 0xf000 || low > above*3/4 {
						return fmt.Errorf("scrim did not reach the pixels: %d above, %d inside", above, low)
					}
					if err := exportRenderSmoke("vertical-scrim.png", path); err != nil {
						return err
					}
					return os.Remove(path)
				}
				if variant == "caption-timed" {
					for i, at := range []string{"2", "7", "12"} {
						path := filepath.Join(ws.Path, fmt.Sprintf("timed-%d.png", i))
						if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-ss", at, "-i", result.Path, "-frames:v", "1", "-c:v", "png", "-threads", "1", path); err != nil {
							return err
						}
						frame, err := readPNG(path)
						if err != nil {
							return err
						}
						// Only the lower half: the disclosure badge and the chips
						// are on screen for the WHOLE clip by design (CDS-5,
						// CDS-30) and both sit at the top, so counting them
						// would say nothing about the caption's own window.
						bright := 0
						for y := frame.Bounds().Dy() / 2; y < frame.Bounds().Dy(); y += 2 {
							for x := 0; x < frame.Bounds().Dx(); x += 2 {
								red, green, _, _ := frame.At(x, y).RGBA()
								if red > 50000 && green > 50000 {
									bright++
								}
							}
						}
						if (i == 1 && bright < 100) || (i != 1 && bright != 0) {
							t.Fatalf("caption exposure at %ss: %d bright pixels", at, bright)
						}
					}
					return nil
				}
				canvas, _ := clip.ClipCanvas(ratio)
				// The badge alone uses the symmetric header bounds (CDS-57).
				// Exempt only its verified box, not the whole header row.
				badge := clip.Region{}
				for _, e := range result.Manifest {
					if e.Kind == "badge" {
						badge = clip.Region(e.Region)
					}
				}
				for i, at := range []string{"2", "7", "12"} {
					path := filepath.Join(ws.Path, fmt.Sprintf("frame-%d.png", i))
					if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-ss", at, "-i", result.Path, "-frames:v", "1", "-c:v", "png", "-threads", "1", path); err != nil {
						return err
					}
					frame, err := readPNG(path)
					if err != nil {
						return err
					}
					if frame.Bounds().Dx() != canvas.Width || frame.Bounds().Dy() != canvas.Height {
						t.Fatal("wrong extracted dimensions")
					}
					bright := 0
					for y := int(canvas.Safe.Y); y < int(canvas.Safe.Y+canvas.Safe.Height); y += 2 {
						for x := int(canvas.Safe.X); x < int(canvas.Safe.X+canvas.Safe.Width); x += 2 {
							red, green, _, _ := frame.At(x, y).RGBA()
							if red > 50000 && green > 50000 {
								bright++
							}
						}
					}
					if bright < 100 {
						t.Fatalf("copy pixels missing for style %s: %d", plan.Cuts[i].Copies[0].Style, bright)
					}
					for y := 0; y < canvas.Height; y += 4 {
						for x := 0; x < canvas.Width; x += 4 {
							if float64(x) >= canvas.Safe.X && float64(x) < canvas.Safe.X+canvas.Safe.Width && float64(y) >= canvas.Safe.Y && float64(y) < canvas.Safe.Y+canvas.Safe.Height {
								continue
							}
							if float64(x) >= badge.X && float64(x) < badge.X+badge.Width && float64(y) >= badge.Y && float64(y) < badge.Y+badge.Height {
								continue
							}
							red, green, _, _ := frame.At(x, y).RGBA()
							if red > 50000 && green > 50000 {
								t.Fatal("copy escaped the safe region")
							}
						}
					}
					if err := exportRenderSmoke(ratio+"-"+plan.Cuts[i].Copies[0].Style+".png", path); err != nil {
						return err
					}
					if err := os.Remove(path); err != nil {
						return err
					}
				}
				// The two cards, in pixels: the hook card is up at the very first
				// frames and the ending card in the last second, each an ink
				// plate over the footage carrying its accent (CDS-28, CDS-29).
				for _, want := range []struct {
					kind, at string
					second   float64
				}{{"hook", "0.5", 0.5}, {"end", fmt.Sprintf("%.1f", float64(plan.DurationMS)/1000-1), float64(plan.DurationMS)/1000 - 1}} {
					var region clip.Region
					for _, e := range result.Manifest {
						if e.Kind == "card" && e.Text == want.kind {
							region = clip.Region(e.Region)
						}
					}
					if region.Width == 0 {
						t.Fatalf("%s card is not in the manifest", want.kind)
					}
					path := filepath.Join(ws.Path, "card-"+want.kind+".png")
					if _, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-ss", want.at, "-i", result.Path, "-frames:v", "1", "-c:v", "png", "-threads", "1", path); err != nil {
						return err
					}
					frame, err := readPNG(path)
					if err != nil {
						return err
					}
					// The footage is flat blue, so the plate is visible as blue
					// the ink took away: inside the card the blue channel is a
					// fraction of what the bare frame has beside it.
					inside := blueAt(frame, int(region.X+region.Width/2), int(region.Y+4))
					outside := blueAt(frame, int(region.X+region.Width/2), int(region.Y)-20)
					if inside > outside/2 {
						t.Fatalf("%s card plate is not on the frame: blue %d inside, %d outside", want.kind, inside, outside)
					}
					// And its own accent is on it: the category pill on the hook
					// card, the CTA line on the ending one.
					if !scan(frame, region, func(r, g, b, a uint32) bool { return r > 40000 && r > 2*b }) {
						t.Fatalf("%s card lost its accent", want.kind)
					}
					if err := exportRenderSmoke(ratio+"-card-"+want.kind+".png", path); err != nil {
						return err
					}
					if err := os.Remove(path); err != nil {
						return err
					}
				}
				var levels []float64
				for _, at := range []string{"2", "7", "12"} {
					data, err := a.run(t.Context(), ws, a.cfg.FFmpegPath, "-v", "error", "-ss", at, "-i", result.Path, "-t", "0.1", "-vn", "-ac", "1", "-ar", "48000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
					if err != nil {
						return err
					}
					sum := 0.0
					for i := 0; i+1 < len(data); i += 2 {
						v := float64(int16(binary.LittleEndian.Uint16(data[i:])))
						sum += v * v
					}
					if len(data) == 0 {
						return fmt.Errorf("empty PCM")
					}
					levels = append(levels, math.Sqrt(sum/float64(len(data)/2)))
				}
				if levels[0] < 500 || levels[1]/levels[0] < .4 || levels[1]/levels[0] > .6 || levels[2] > 2 {
					t.Fatalf("volume levels=%v", levels)
				}
				if err := exportRenderSmoke(ratio+".mp4", result.Path); err != nil {
					return err
				}
				files, err := os.ReadDir(ws.Path)
				if err != nil {
					return err
				}
				if len(files) != 1 || files[0].Name() != "clip-result.mp4" {
					t.Fatalf("render intermediates leaked: %v", files)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// isAccent reads amber #FFB020 through whatever is drawn over it: red leads and
// blue trails, which no other colour in these styles does.
func isAccent(img image.Image, x, y int) bool {
	r, g, b, a := img.At(x, y).RGBA()
	return a > 0x8000 && r > 2*b && g > b
}
func scan(img image.Image, region clip.Region, match func(r, g, b, a uint32) bool) bool {
	for y := int(region.Y); y < int(region.Y+region.Height); y++ {
		for x := int(region.X); x < int(region.X+region.Width); x++ {
			if match(img.At(x, y).RGBA()) {
				return true
			}
		}
	}
	return false
}
func readPNG(path string) (image.Image, error) { return readFrame(path) }

// Only an explicit local test export retains synthetic fixtures for owner picker
// verification. Ordinary build smoke leaves no file outside its test workspace.
func exportRenderSmoke(name, path string) error {
	dir := os.Getenv("CLIP_SMOKE_EXPORT")
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || filepath.Dir(dir) == "/" {
		return fmt.Errorf("unsafe smoke export directory")
	}
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(dest, source)
	return errorsJoinClose(err, dest)
}
func errorsJoinClose(err error, file *os.File) error {
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func volume(v float64) *float64 { return &v }

// blueAt is the blue channel at one pixel, which is what the flat blue fixture
// makes a plate measurable by: ink at α0.88 keeps only an eighth of it.
func blueAt(img image.Image, x, y int) uint32 {
	_, _, b, _ := img.At(x, y).RGBA()
	return b
}
