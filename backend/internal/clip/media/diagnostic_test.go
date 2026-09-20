package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestCommandDiagnosticsPreserveCausesWithoutPrivateLabels(t *testing.T) {
	for _, tc := range []struct {
		cause error
		class string
	}{{context.DeadlineExceeded, "timeout"}, {context.Canceled, "canceled"}, {clip.ErrWorkspaceLimit, "workspace_limit"}, {clip.ErrAnalysisTooLarge, "output_limit"}, {errors.New("private-canary"), "command_failed"}} {
		err := commandDiagnostic(fmt.Errorf("private-canary: %w", tc.cause), Command{Binary: "/private/ffmpeg", Args: []string{"/private/render-cut-0000.mp4"}}, 2*time.Second)
		var d *commandFailure
		if !errors.Is(err, tc.cause) || !errors.As(err, &d) || d.MediaOperation() != "render_cut" || d.MediaFailureClass() != tc.class || d.MediaElapsedMS() != 2000 {
			t.Fatalf("lost diagnosis: %v", err)
		}
	}
	if got := commandOperation(Command{Binary: "/private-canary", Args: []string{"private-canary"}}); got != "unknown" {
		t.Fatal(got)
	}
}

func TestEachRenderInputHasItsOwnResourceLimits(t *testing.T) {
	fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("video"), 0600)
	}}
	a := newAdapter(t, fake)
	r := testRenderer(t, a)
	err := a.WithWorkspace(t.Context(), "bounds", func(ws clip.MediaWorkspace) error {
		canvas, _ := clip.ClipCanvas("vertical")
		cut := clip.EditCut{EndMS: 4200, Focal: clip.Point{X: .5, Y: .5}}
		l := layers{Fixed: "fixed.png", Copies: []string{"copy1.png", "copy2.png"}}
		return r.renderCut(t.Context(), ws, canvas, cut, clip.MediaSource{Path: sourceFile(t, ws)}, 126, l, filepath.Join(ws.Path, "render-cut-0000.mp4"), false, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	args := fake.calls[0].Args
	previous, inputs := 0, 0
	for i, arg := range args {
		if arg != "-i" {
			continue
		}
		options := " " + strings.Join(args[previous:i], " ") + " "
		// Every input decodes on the decoder's own thread count, which may exceed
		// the encoder's; nothing loops a layer for the whole cut.
		if !strings.Contains(options, " -threads 2 ") || strings.Contains(options, " -loop ") {
			t.Fatalf("unbounded input %d: %s", inputs, options)
		}
		previous, inputs = i+2, inputs+1
	}
	// The encode this cut ends in stays single-threaded, so its bytes do not move.
	if output := " " + strings.Join(args[previous:], " ") + " "; !strings.Contains(output, " -threads 1 ") || strings.Contains(output, " -threads 2 ") {
		t.Fatalf("the encoder did not stay single-threaded: %s", output)
	}
	if inputs != 4 {
		t.Fatalf("expected source and three layers, got %d", inputs)
	}
	if strings.Count(strings.Join(args, " "), "loop=loop=125:size=1:start=0") != 2 {
		t.Fatal("animated copies must each reuse one frame for only 126 frames", args)
	}
}

func TestEveryCommandRecordsItsDurationOnSuccessAndFailure(t *testing.T) {
	fail := errors.New("private-canary")
	a := newAdapter(t, &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		if filepath.Base(c.Binary) == "ffprobe" {
			return []byte(probeJSON), nil
		}
		return nil, fail
	}})
	var records []clip.MediaRecord
	ctx := clip.WithMediaStageObserver(t.Context(), func(r clip.MediaRecord) { records = append(records, r) })
	var failed error
	if err := a.WithWorkspace(ctx, "durations", func(ws clip.MediaWorkspace) error {
		source := sourceFile(t, ws)
		if _, err := a.run(ctx, ws, "/usr/local/bin/ffprobe", "-i", source); err != nil {
			return err
		}
		_, failed = a.run(ctx, ws, "/usr/local/bin/ffmpeg", "-i", source, "render-cut-0000.mp4")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("a successful command went untimed: %+v", records)
	}
	if records[0] != (clip.MediaRecord{Operation: "probe", Outcome: "ok", Elapsed: records[0].Elapsed}) || records[0].Elapsed < 0 {
		t.Fatalf("success record: %+v", records[0])
	}
	if records[1] != (clip.MediaRecord{Operation: "render_cut", Outcome: "command_failed", Elapsed: records[1].Elapsed}) {
		t.Fatalf("failure record: %+v", records[1])
	}
	var d *commandFailure
	if !errors.Is(failed, fail) || !errors.As(failed, &d) || d.MediaOperation() != "render_cut" || d.MediaFailureClass() != "command_failed" {
		t.Fatalf("typed failure changed: %v", failed)
	}
}

func TestOperationRecordsCarryCodeOwnedLabelsOnly(t *testing.T) {
	var records []clip.MediaRecord
	ctx := clip.WithMediaStageObserver(t.Context(), func(r clip.MediaRecord) { records = append(records, r) })
	clip.ReportMediaOperation(ctx, "/private/canary.mp4", "private-canary", time.Second)
	if len(records) != 1 || records[0] != (clip.MediaRecord{Operation: "unknown", Outcome: "unknown", Elapsed: time.Second}) {
		t.Fatalf("private label reached the sink: %+v", records)
	}
}

// An operation label is how a failed render is read months later. The two
// passes that dominate a composition both fell through to "decode", the name
// reserved for a command that does the least of all.
func TestCompositionPassesAreNamedByTheirOwnOutput(t *testing.T) {
	for output, want := range map[string]string{
		"bare-0003.mp4":           "render_cut",
		"compose-01-0000.mp4":     "compose_video",
		"composition-footage.mp4": "compose_video",
		"composition-audio.wav":   "compose_audio",
		"compose-audio.wav":       "compose_audio",
		"clip-result.mp4":         "encode_final",
	} {
		c := Command{Binary: "/usr/local/bin/ffmpeg", Args: []string{"-i", "/private/in.mp4", filepath.Join("/private", output)}}
		if got := commandOperation(c); got != want {
			t.Fatalf("%s reported itself as %s, not %s", output, got, want)
		}
	}
}

// A runner-only red is diagnosed from the one line the smoke prints. CLIP-88
// makes the diagnostic's own sentence the privacy-safe one the SERVER may show,
// so the failure underneath it is invisible unless the smoke unwraps it — and
// twice now a red substage has reported only `check=render_encode values=map[]`,
// which names the step and nothing about why it refused. Inside the image there
// is no caller to keep it from, so renderFailure spells the cause out.
func TestRenderFailureNamesTheCauseTheDiagnosticHides(t *testing.T) {
	cause := commandDiagnostic(
		errors.New("ffmpeg: no space left on device"),
		Command{Binary: "/usr/local/bin/ffmpeg", Args: []string{"-i", "/w/in.mp4", "/w/clip-result.mp4"}},
		3*time.Second,
	)
	err := clip.WithAttemptDiagnostic(cause, clip.AttemptDiagnostic{Check: "render_encode", Phase: "render"})
	// The server's own sentence must stay exactly as bare as it is today.
	if err.Error() != "clip attempt validation failed" {
		t.Fatalf("the privacy-safe sentence changed: %v", err)
	}
	got := renderFailure(err).Error()
	for _, want := range []string{
		"check=render_encode", "phase=render",
		"media=encode_final", "class=command_failed", "elapsed=3000ms",
		"no space left on device",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderFailure dropped %q from: %s", want, got)
		}
	}
}
