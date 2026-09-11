package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

func renderConfig(t *testing.T) clip.RenderConfig {
	t.Helper()
	font := os.Getenv("CLIP_FONT_PATH")
	if font == "" {
		font = "/usr/share/postpilot-fonts/pretendard/PretendardVariable.ttf"
	}
	return config.ClipRender(&config.Config{ClipResvgPath: "/usr/local/bin/resvg", ClipFontPath: font})
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
		semibold, err := r.measure(t.Context(), ws, []string{"한글 여행 W"}, 600)
		if err != nil {
			return err
		}
		bold, err := r.measure(t.Context(), ws, []string{"한글 여행 W"}, 800)
		if err != nil {
			return err
		}
		if semibold["한글 여행 W"] == bold["한글 여행 W"] {
			t.Fatal("variable font weight was ignored")
		}
		canvas, _ := clip.ClipCanvas("vertical")
		plate, err := r.copyPlate(t.Context(), ws, canvas, clip.Copy{Text: "한글 여행", Style: "clean", Anchor: "bottom", Align: "center"}, 0)
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
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"vertical", "horizontal", "square", "silent-rounded", "audio-rounded", "caption-timed"} {
		t.Run(variant, func(t *testing.T) {
			ratio := variant
			if variant == "silent-rounded" || variant == "audio-rounded" || variant == "caption-timed" {
				ratio = "square"
			}
			err := a.WithWorkspace(t.Context(), ratio, func(ws clip.MediaWorkspace) error {
				infos := map[string]clip.MediaInfo{}
				// No source is retained between callbacks; the fixture loader creates
				// exactly one file at a time, like the streaming T076 object adapter.
				load := func(ctx context.Context, id string, consume func(clip.MediaSource) error) error {
					path := filepath.Join(ws.Path, "fixture.mp4")
					defer os.Remove(path)
					args := []string{"-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=1280x720:r=30"}
					if id != "silent" {
						args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
					}
					length := "6"
					if variant == "silent-rounded" || variant == "audio-rounded" || variant == "caption-timed" {
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
				plan := clip.EditPlan{Ratio: ratio, DurationMS: 15000, Cuts: []clip.EditCut{
					{ID: "one", SourceID: "audio", Fingerprint: "audio", EndMS: 5200, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "정확한 한글 & 여행", Style: "clean", Anchor: "bottom", Align: "center", Accent: "coral"}},
					{ID: "two", SourceID: "rotated", Fingerprint: "rotated", EndMS: 5000, Focal: clip.Point{X: .5, Y: .5}, Volume: volume(.5), Copy: clip.Copy{Text: "기록처럼 <오늘>", Style: "memo", Anchor: "top", Align: "left", Accent: "teal"}},
					{ID: "three", SourceID: "silent", Fingerprint: "silent", EndMS: 5200, Focal: clip.Point{X: .5, Y: .5}, Volume: volume(0), Copy: clip.Copy{Text: "다시 오고 싶은 곳", Style: "bold", Anchor: "lower_mid", Align: "center", Accent: "amber"}},
				}}
				if variant == "silent-rounded" || variant == "audio-rounded" {
					plan.DurationMS = 15017
					if variant == "silent-rounded" {
						plan.Cuts = plan.Cuts[2:]
					} else {
						plan.Cuts = plan.Cuts[:1]
					}
					plan.Cuts[0].EndMS = 15017
					plan.Cuts[0].Copy.Text = ""
				}
				if variant == "caption-timed" {
					plan.Cuts = plan.Cuts[:1]
					plan.Cuts[0].EndMS = 15000
					plan.Cuts[0].Copy.StartMS = 4000
					plan.Cuts[0].Copy.EndMS = 10000
					width, height, err := r.CaptionSize(t.Context(), ratio, plan.Cuts[0].Copy)
					if err != nil || width <= 0 || height <= 0 {
						return fmt.Errorf("measure timed caption: %v", err)
					}
				}
				result, err := r.Render(t.Context(), ws, plan, sources, load)
				if err != nil {
					return err
				}
				if math.Abs(float64(result.Info.DurationMS-plan.DurationMS)) > 1000.0/30 || result.Info.HasAudio != (variant != "silent-rounded") {
					t.Fatalf("result=%+v", result)
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
						bright := 0
						for y := 0; y < frame.Bounds().Dy(); y += 2 {
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
						t.Fatalf("copy pixels missing for style %s: %d", plan.Cuts[i].Copy.Style, bright)
					}
					for y := 0; y < canvas.Height; y += 4 {
						for x := 0; x < canvas.Width; x += 4 {
							if float64(x) >= canvas.Safe.X && float64(x) < canvas.Safe.X+canvas.Safe.Width && float64(y) >= canvas.Safe.Y && float64(y) < canvas.Safe.Y+canvas.Safe.Height {
								continue
							}
							red, green, _, _ := frame.At(x, y).RGBA()
							if red > 50000 && green > 50000 {
								t.Fatal("copy escaped the safe region")
							}
						}
					}
					if err := exportRenderSmoke(ratio+"-"+plan.Cuts[i].Copy.Style+".png", path); err != nil {
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
func readPNG(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}

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
