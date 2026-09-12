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
		l := layers{Fixed: "fixed.png", Copies: []string{"copy1.png", "copy2.png"}, Card: "card.png", Window: cardLayout{Kind: "hook", EndMS: 1500}}
		return r.renderCut(t.Context(), ws, canvas, cut, clip.MediaSource{Path: sourceFile(t, ws)}, 126, l, filepath.Join(ws.Path, "render-cut-0000.mp4"), false)
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
	if inputs != 5 {
		t.Fatalf("expected source and four layers, got %d", inputs)
	}
	if strings.Count(strings.Join(args, " "), "loop=loop=125:size=1:start=0") != 3 {
		t.Fatal("animated copies and card must each reuse one frame for only 126 frames", args)
	}
}
