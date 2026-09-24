// Package diagnostic runs isolated, local-file experiments. It has no worker
// transport, artifact publisher, database or provider dependency.
package diagnostic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/media"
)

const candidate = "nvenc-candidate-v1"
const fileLimit = 128 << 20

// Manifest contains already composed synthetic CPU outputs, never user jobs.
// Comparing their delivery re-encode isolates encoder differences, not total
// production speed or a lossless pre-encode master comparison.
type Manifest struct {
	Version  int
	Fixtures []Fixture
}
type Fixture struct {
	Name                      string
	File                      string
	CPUCompositionAndEncodeMS int64
}
type Probe struct {
	Status string
	Detail string
}
type Encoding struct {
	ElapsedMS   int64
	Bytes       int64
	SHA256      string
	Streams     json.RawMessage
	Frames      []string
	AudioSHA256 string
}
type Comparison struct {
	Fixture                        Fixture
	InputSHA256                    string
	CPU                            *Encoding
	GPU                            *Encoding
	FrameMeanAbsoluteRGBDifference []float64
	AudioIdentical                 *bool
}
type Report struct {
	Version            int
	Candidate          string
	ProductionApproved bool
	Scope              string
	OS                 string
	Architecture       string
	Status             string
	Error              string
	CPUManifest        json.RawMessage
	Tools              map[string]string
	Parameters         map[string][]string
	Probes             map[string]Probe
	ResourcesBefore    map[string]string
	ResourcesAfter     map[string]string
	OutputBytes        int64
	Comparisons        []Comparison
}

type experiment struct {
	env    clip.Environment
	runner media.Runner
	out    string
	report Report
}

// Run is CLI plumbing for offline diagnostics. Only explicit local paths are
// accepted; API environment values are neither read nor passed to a client.
func Run(ctx context.Context, command string, args []string, env clip.Environment) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	output := flags.String("output", "", "new local report directory")
	manifest := flags.String("manifest", "", "local synthetic fixture manifest (benchmark only)")
	cpuOnly := flags.Bool("cpu-only", false, "record CPU baseline and leave GPU unverified")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *output == "" || (command == "benchmark" && *manifest == "") || (command == "gpu-probe" && (*manifest != "" || *cpuOnly)) {
		return errors.New("usage: gpu-probe --output NEW_DIR | benchmark --manifest FILE --output NEW_DIR [--cpu-only]")
	}
	if command != "gpu-probe" && command != "benchmark" {
		return errors.New("unknown diagnostic")
	}
	if *manifest != "" {
		absolute, err := filepath.Abs(*manifest)
		if err != nil {
			return err
		}
		*manifest = absolute
	}
	out, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	// Exclusive directory prevents overwriting a report, input or live workspace.
	if err = os.Mkdir(out, 0700); err != nil {
		return fmt.Errorf("create new diagnostic directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	env.WorkRoot = filepath.Join(out, "work")
	e := experiment{env: env, runner: &media.ExecRunner{StdoutLimit: 4 << 20, StderrLimit: 32 << 10, WaitDelay: 2 * time.Second}, out: out,
		report: Report{Version: 1, Candidate: candidate, Scope: "delivery re-encode of synthetic CPU-composed MP4; not end-to-end GPU qualification", OS: runtime.GOOS, Architecture: runtime.GOARCH, Status: "unverified", Tools: map[string]string{}, Probes: map[string]Probe{}, Parameters: map[string][]string{}}}
	e.report.ResourcesBefore = resources()
	err = e.execute(ctx, command, *manifest, *cpuOnly)
	if err != nil {
		e.report.Status = "failed"
		e.report.Error = err.Error()
	}
	e.report.ResourcesAfter = resources()
	_ = filepath.Walk(out, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr == nil && info.Mode().IsRegular() {
			e.report.OutputBytes += info.Size()
		}
		return nil
	})
	raw, marshalErr := json.MarshalIndent(e.report, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	if writeErr := os.WriteFile(filepath.Join(out, "report.json"), append(raw, '\n'), 0600); writeErr != nil {
		return writeErr
	}
	return err
}
func (e *experiment) call(ctx context.Context, binary string, args ...string) ([]byte, error) {
	c, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	return e.runner.Run(c, media.Command{Binary: binary, Args: args, Dir: e.out})
}
func (e *experiment) execute(ctx context.Context, command, path string, cpuOnly bool) error {
	adapter, err := media.New(clip.DefaultMediaConfig(e.env), nil)
	if err != nil {
		return err
	}
	renderer, err := media.NewRenderer(adapter, clip.DefaultRenderConfig(e.env))
	if err != nil {
		return err
	}
	profile, err := renderer.RuntimeProfile(ctx, "cpu")
	if err != nil {
		return err
	}
	e.report.CPUManifest = json.RawMessage(profile.RuntimeManifest)
	for _, binary := range []string{e.env.FFmpegPath, e.env.FFprobePath, "/opt/nvidia/bin/ffmpeg", "/opt/nvidia/bin/ffprobe"} {
		version, callErr := e.call(ctx, binary, "-version")
		if callErr != nil {
			return callErr
		}
		sum, hashErr := digest(binary)
		if hashErr != nil {
			return hashErr
		}
		e.report.Tools[binary] = strings.SplitN(string(version), "\n", 2)[0] + " sha256=" + sum
	}
	cpu := []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-profile:v", "high"}
	gpu := []string{"-c:v", "h264_nvenc", "-preset", "p4", "-tune", "hq", "-rc", "vbr", "-cq", "20", "-b:v", "0", "-profile:v", "high"}
	e.report.Parameters["cpu"] = cpu
	e.report.Parameters["gpu"] = gpu
	e.report.Parameters["common"] = []string{"-threads", "1", "-filter_threads", "1", "-pix_fmt", "yuv420p", "-r", "30", "-fps_mode", "cfr", "-c:a", "copy"}
	e.report.Probes["scale_cuda"] = Probe{"unverified", "not built: no CUDA compiler/NPP; CPU composition and scale retained"}
	if cpuOnly {
		e.report.Probes["device"] = Probe{"unverified", "CPU-only baseline explicitly requested"}
	} else if err = e.gpuProbe(ctx, gpu); err != nil {
		return err
	}
	if command == "gpu-probe" {
		e.report.Status = "diagnostic-only"
		return nil
	}
	fixtures, err := loadManifest(path)
	if err != nil {
		return err
	}
	for _, f := range fixtures.Fixtures {
		input := filepath.Join(filepath.Dir(path), f.File)
		sum, err := digest(input)
		if err != nil {
			return err
		}
		c := Comparison{Fixture: f, InputSHA256: sum}
		// Probe before decode: dimensions/duration bound the subprocess work.
		if err = e.validateInput(ctx, input); err != nil {
			return err
		}
		c.CPU, err = e.encode(ctx, e.env.FFmpegPath, input, f.Name+"-cpu", cpu)
		if err != nil {
			return err
		}
		if !cpuOnly {
			c.GPU, err = e.encode(ctx, "/opt/nvidia/bin/ffmpeg", input, f.Name+"-gpu", gpu)
			if err != nil {
				return err
			}
			for i := range c.CPU.Frames {
				difference, err := frameDifference(filepath.Join(e.out, fmt.Sprintf("%s-cpu-%d.png", f.Name, i)), filepath.Join(e.out, fmt.Sprintf("%s-gpu-%d.png", f.Name, i)))
				if err != nil {
					return err
				}
				c.FrameMeanAbsoluteRGBDifference = append(c.FrameMeanAbsoluteRGBDifference, difference)
			}
			same := c.CPU.AudioSHA256 == c.GPU.AudioSHA256
			c.AudioIdentical = &same
		}
		e.report.Comparisons = append(e.report.Comparisons, c)
	}
	e.report.Status = "diagnostic-only"
	if cpuOnly {
		e.report.Status = "cpu-only; GPU unverified"
	}
	return nil
}
func (e *experiment) gpuProbe(ctx context.Context, gpu []string) error {
	smi, err := e.call(ctx, "nvidia-smi", "--query-gpu=name,uuid,driver_version,memory.total,memory.used", "--format=csv,noheader")
	if err != nil {
		e.report.Probes["device"] = Probe{"failed", err.Error()}
		return errors.New("NVIDIA device/driver unavailable; see report.json")
	}
	e.report.Probes["device"] = Probe{"observed", strings.TrimSpace(string(smi))}
	args := []string{"-hide_banner", "-v", "error", "-nostdin", "-f", "lavfi", "-i", "color=c=blue:s=256x144:r=30", "-frames:v", "3"}
	args = append(args, gpu...)
	args = append(args, "-pix_fmt", "yuv420p", "-threads", "1", "-fs", "1048576", "gpu-probe.mp4")
	_, err = e.call(ctx, "/opt/nvidia/bin/ffmpeg", args...)
	if err != nil {
		e.report.Probes["encode"] = Probe{"failed", err.Error()}
		return errors.New("actual NVENC encode failed; see report.json")
	}
	e.report.Probes["encode"] = Probe{"passed", "three H.264 frames encoded; not production approval"}
	_, err = e.call(ctx, "/opt/nvidia/bin/ffmpeg", "-hide_banner", "-v", "error", "-nostdin", "-c:v", "h264_cuvid", "-i", "gpu-probe.mp4", "-frames:v", "3", "-f", "null", "-")
	if err != nil {
		e.report.Probes["decode"] = Probe{"failed", err.Error()}
	} else {
		e.report.Probes["decode"] = Probe{"passed", "independent CUVID decode; benchmark decode remains CPU"}
	}
	return nil
}
func loadManifest(path string) (Manifest, error) {
	var m Manifest
	f, err := os.Open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 65537))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&m); err != nil {
		return m, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return m, errors.New("trailing manifest data")
	}
	if m.Version != 1 || len(m.Fixtures) == 0 || len(m.Fixtures) > 6 {
		return m, errors.New("manifest needs version 1 and 1..6 fixtures")
	}
	names := map[string]bool{}
	for _, f := range m.Fixtures {
		if f.Name == "" || len(f.Name) > 64 || strings.Trim(f.Name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != "" || names[f.Name] {
			return m, errors.New("invalid or duplicate fixture name")
		}
		names[f.Name] = true
		if filepath.Base(f.File) != f.File || filepath.Ext(f.File) != ".mp4" || f.CPUCompositionAndEncodeMS < 0 {
			return m, errors.New("fixtures must name local MP4 basenames")
		}
		info, err := os.Lstat(filepath.Join(filepath.Dir(path), f.File))
		if err != nil {
			return m, err
		}
		if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > fileLimit {
			return m, errors.New("fixture must be a regular file of 1..128 MiB")
		}
	}
	return m, nil
}
func (e *experiment) validateInput(ctx context.Context, path string) error {
	raw, err := e.call(ctx, e.env.FFprobePath, "-v", "error", "-protocol_whitelist", "file,pipe", "-show_streams", "-show_format", "-of", "json", path)
	if err != nil {
		return err
	}
	var info struct {
		Streams []struct {
			CodecType     string `json:"codec_type"`
			Width, Height int
		}
		Format struct{ Duration string }
	}
	if err = json.Unmarshal(raw, &info); err != nil {
		return err
	}
	d, err := time.ParseDuration(info.Format.Duration + "s")
	if err != nil || d <= 0 || d > 90*time.Second {
		return errors.New("fixture duration must be <=90 seconds")
	}
	video := 0
	for _, s := range info.Streams {
		if s.CodecType == "video" {
			video++
			if s.Width <= 0 || s.Height <= 0 || s.Width > 1920 || s.Height > 1920 || s.Width*s.Height > 1920*1080 {
				return errors.New("fixture exceeds 1080p canvas budget")
			}
		}
	}
	if video != 1 || len(info.Streams) > 2 {
		return errors.New("fixture must have one video and at most one audio stream")
	}
	return nil
}
func (e *experiment) encode(ctx context.Context, binary, input, name string, encoder []string) (*Encoding, error) {
	output := filepath.Join(e.out, name+".mp4")
	args := []string{"-hide_banner", "-v", "error", "-nostdin", "-protocol_whitelist", "file,pipe", "-threads", "1", "-i", input, "-map", "0:v:0", "-map", "0:a:0?", "-map_metadata", "-1", "-filter_threads", "1", "-threads", "1"}
	args = append(args, encoder...)
	args = append(args, "-pix_fmt", "yuv420p", "-r", "30", "-fps_mode", "cfr", "-c:a", "copy", "-t", "90", "-fs", fmt.Sprint(fileLimit), "-movflags", "+faststart", output)
	started := time.Now()
	_, err := e.call(ctx, binary, args...)
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	if info.Size() >= fileLimit {
		return nil, errors.New("diagnostic output hit file budget")
	}
	sum, err := digest(output)
	if err != nil {
		return nil, err
	}
	result := &Encoding{ElapsedMS: elapsed, Bytes: info.Size(), SHA256: sum}
	result.Streams, err = e.call(ctx, e.env.FFprobePath, "-v", "error", "-show_streams", "-show_format", "-of", "json", output)
	if err != nil {
		return nil, err
	}
	// Frame positions cover starts, caption/motion/transition intervals and late output.
	var measured struct {
		Format  struct{ Duration string }
		Streams []struct {
			CodecType string `json:"codec_type"`
		}
	}
	if err = json.Unmarshal(result.Streams, &measured); err != nil {
		return nil, err
	}
	duration, err := time.ParseDuration(measured.Format.Duration + "s")
	if err != nil {
		return nil, err
	}
	for i := range 8 {
		frame := filepath.Join(e.out, fmt.Sprintf("%s-%d.png", name, i))
		_, err = e.call(ctx, e.env.FFmpegPath, "-hide_banner", "-v", "error", "-nostdin", "-threads", "1", "-ss", fmt.Sprintf("%.6f", duration.Seconds()*float64(i)/8), "-i", output, "-frames:v", "1", "-threads", "1", "-filter_threads", "1", "-c:v", "png", "-f", "image2", frame)
		if err != nil {
			return nil, err
		}
		hash, err := digest(frame)
		if err != nil {
			return nil, err
		}
		result.Frames = append(result.Frames, hash)
	}
	for _, s := range measured.Streams {
		if s.CodecType == "audio" {
			audio := filepath.Join(e.out, name+".wav")
			_, err = e.call(ctx, e.env.FFmpegPath, "-hide_banner", "-v", "error", "-nostdin", "-threads", "1", "-i", output, "-vn", "-c:a", "pcm_s16le", "-ar", "48000", "-ac", "2", "-t", "90", "-fs", fmt.Sprint(fileLimit), audio)
			if err != nil {
				return nil, err
			}
			result.AudioSHA256, err = digest(audio)
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
func digest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func frameDifference(a, b string) (float64, error) {
	left, err := os.Open(a)
	if err != nil {
		return 0, err
	}
	defer left.Close()
	right, err := os.Open(b)
	if err != nil {
		return 0, err
	}
	defer right.Close()
	x, err := png.Decode(left)
	if err != nil {
		return 0, err
	}
	y, err := png.Decode(right)
	if err != nil {
		return 0, err
	}
	if x.Bounds() != y.Bounds() {
		return 0, errors.New("decoded frame dimensions differ")
	}
	var total float64
	for py := 0; py < x.Bounds().Dy(); py++ {
		for px := 0; px < x.Bounds().Dx(); px++ {
			r, g, b, _ := x.At(px, py).RGBA()
			rr, gg, bb, _ := y.At(px, py).RGBA()
			for _, pair := range [][2]uint32{{r, rr}, {g, gg}, {b, bb}} {
				d := float64(pair[0])/257 - float64(pair[1])/257
				if d < 0 {
					d = -d
				}
				total += d
			}
		}
	}
	return total / float64(x.Bounds().Dx()*x.Bounds().Dy()*3), nil
}
func resources() map[string]string {
	out := map[string]string{}
	for _, name := range []string{"memory.peak", "memory.max", "cpu.max", "cpu.stat"} {
		if raw, err := os.ReadFile("/sys/fs/cgroup/" + name); err == nil {
			out[name] = strings.TrimSpace(string(raw))
		}
	}
	return out
}
