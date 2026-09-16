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
		if !strings.Contains(options, " -threads 1 ") || strings.Contains(options, " -loop ") {
			t.Fatalf("unbounded input %d: %s", inputs, options)
		}
		previous, inputs = i+2, inputs+1
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
