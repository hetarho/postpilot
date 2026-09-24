package diagnostic

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestBoundsAndTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "fixture.mp4"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, file string
		version    int
		valid      bool
	}{
		{"normal", "fixture.mp4", 1, true}, {"../out", "fixture.mp4", 1, false}, {"valid", "../fixture.mp4", 1, false}, {"valid", "fixture.mp4", 2, false}, {"valid", "missing.mp4", 1, false},
	} {
		m := Manifest{Version: test.version, Fixtures: []Fixture{{Name: test.name, File: test.file}}}
		raw, _ := json.Marshal(m)
		path := filepath.Join(root, "manifest.json")
		_ = os.WriteFile(path, raw, 0600)
		_, err := loadManifest(path)
		if (err == nil) != test.valid {
			t.Errorf("%+v: %v", test, err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "fixture.mp4"), filepath.Join(root, "link.mp4")); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(Manifest{Version: 1, Fixtures: []Fixture{{Name: "link", File: "link.mp4"}}})
	_ = os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0600)
	if _, err := loadManifest(filepath.Join(root, "manifest.json")); err == nil {
		t.Fatal("symlink accepted")
	}
}
func TestCandidateOfflineDiagnostics(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("candidate image execution gate")
	}
	root := t.TempDir()
	// No network and deliberately invalid application configuration.
	environment := append(os.Environ(), "MEDIA_API_URL=http://invalid.invalid", "MEDIA_WORKER_TOKEN=invalid", "DATABASE_PATH=/forbidden/db", "PROVIDERS_CONFIG=/missing", "MEDIA_ACCEL=nvenc")
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(t.Context(), "/media-worker", args...)
		cmd.Env = environment
		return cmd.CombinedOutput()
	}
	probe := filepath.Join(root, "missing-device")
	out, err := run("gpu-probe", "--output", probe)
	if err == nil {
		t.Fatal("CPU-only test environment unexpectedly has a GPU")
	}
	var report Report
	raw, err := os.ReadFile(filepath.Join(probe, "report.json"))
	if err != nil {
		t.Fatalf("missing failure report: %v %s", err, out)
	}
	if err = json.Unmarshal(raw, &report); err != nil || report.ProductionApproved || report.Probes["device"].Status != "failed" {
		t.Fatal(string(raw), err)
	}
	input := filepath.Join(root, "fixture.mp4")
	cmd := exec.CommandContext(t.Context(), "/usr/local/bin/ffmpeg", "-hide_banner", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=256x144:r=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-threads", "1", "-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p", input)
	if out, err = cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	raw, _ = json.Marshal(Manifest{Version: 1, Fixtures: []Fixture{{Name: "smoke", File: "fixture.mp4"}}})
	manifest := filepath.Join(root, "manifest.json")
	if err = os.WriteFile(manifest, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "baseline")
	if out, err = run("benchmark", "--cpu-only", "--manifest", manifest, "--output", output); err != nil {
		t.Fatalf("offline baseline: %v %s", err, out)
	}
	raw, err = os.ReadFile(filepath.Join(output, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &report); err != nil || report.ProductionApproved || !strings.Contains(report.Status, "GPU unverified") || len(report.Comparisons) != 1 {
		t.Fatal(string(raw), err)
	}
	c := report.Comparisons[0]
	if c.GPU != nil || c.CPU == nil || c.CPU.Bytes == 0 || len(c.CPU.Frames) != 8 || c.CPU.AudioSHA256 == "" || len(report.CPUManifest) == 0 {
		t.Fatal("incomplete CPU baseline", string(raw))
	}
	if _, err = run("benchmark", "--cpu-only", "--manifest", manifest, "--output", output); err == nil {
		t.Fatal("overwrote existing report")
	}
	if _, err = run("gpu-probe", "--output", filepath.Join(root, "bad"), "--manifest", manifest); err == nil {
		t.Fatal("ignored unexpected manifest")
	}
}

func TestDecodedFrameDifferenceIsMeasurementNotAcceptance(t *testing.T) {
	root := t.TempDir()
	write := func(name string, c color.RGBA, width int) string {
		t.Helper()
		path := filepath.Join(root, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, 1))
		img.SetRGBA(0, 0, c)
		if err = png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a := write("a.png", color.RGBA{R: 255, A: 255}, 1)
	b := write("b.png", color.RGBA{B: 255, A: 255}, 1)
	difference, err := frameDifference(a, b)
	if err != nil || difference != 170 {
		t.Fatal(difference, err)
	}
	difference, err = frameDifference(a, a)
	if err != nil || difference != 0 {
		t.Fatal(difference, err)
	}
	if _, err = frameDifference(a, write("wide.png", color.RGBA{}, 2)); err == nil {
		t.Fatal("dimension mismatch hidden")
	}
}
