package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestCompositionTreeBoundsVideoDecodersAndKeepsAudioForFinalPass(t *testing.T) {
	for _, audio := range []bool{false, true} {
		fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
			return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("video"), 0600)
		}}
		a := newAdapter(t, fake)
		r := testRenderer(t, a)
		err := a.WithWorkspace(t.Context(), "composition", func(ws clip.MediaWorkspace) error {
			cuts := make([]string, 100)
			frames := make([]int, 100)
			for i := range cuts {
				cuts[i] = filepath.Join(ws.Path, fmt.Sprintf("render-cut-%04d.mp4", i))
				frames[i] = 21
				if err := os.WriteFile(cuts[i], []byte("original cut with audio"), 0600); err != nil {
					return err
				}
			}
			// Every boundary fades, which is the densest tree the renderer can
			// be asked for: each merge has to carry its own overlap.
			transitions := make([]int, 100)
			for i := 1; i < len(transitions); i++ {
				transitions[i] = 200
			}
			assembled := ""
			if audio {
				assembled = filepath.Join(ws.Path, "compose-audio.wav")
			}
			var cleanup []string
			args, err := r.compositionInputs(t.Context(), ws, cuts, frames, transitions, assembled, &loudness{I: -20, TP: -3, LRA: 7, Threshold: -30, Offset: 0.1}, &cleanup)
			if err != nil {
				return err
			}
			if len(fake.calls) != 98 {
				t.Fatal("wrong tree shape", len(fake.calls))
			}
			for _, call := range fake.calls {
				joined := strings.Join(call.Args, " ")
				if strings.Count(joined, ":v:0]") != 2 || strings.Contains(joined, ":a:0]") || !strings.Contains(joined, "-crf 0") || !strings.Contains(joined, "-pix_fmt yuv444p") || !strings.Contains(joined, "-threads 1") {
					t.Fatal("unbounded/lossy intermediate", joined)
				}
			}
			graph := args[slices.Index(args, "-filter_complex")+1]
			if strings.Count(graph, ":v:0]") != 2 || !strings.Contains(graph, "trim=end_frame=1506,") {
				t.Fatal("lost timeline", graph)
			}
			// The audio is one assembled, already-measured track by now: the
			// final pass opens a single audio decoder and only normalises it.
			if audio && (strings.Count(graph, ":a:0]") != 1 || !strings.Contains(graph, "measured_I=-20") || !strings.Contains(graph, "linear=true")) {
				t.Fatal("did not apply the measured loudness", graph)
			}
			if !audio && strings.Contains(graph, ":a:0]") {
				t.Fatal("invented audio")
			}
			for _, cut := range cuts {
				if _, err := os.Stat(cut); err != nil {
					t.Fatal("removed original audio before final composition", err)
				}
			}
			remaining := 0
			for _, path := range cleanup {
				if _, err := os.Stat(path); err == nil {
					remaining++
				}
			}
			if remaining != 2 {
				t.Fatal("consumed tree nodes remain", remaining)
			}
			profile := strings.Join(r.encodeArgs(audio), " ")
			if !strings.Contains(profile, "-crf 20") || !strings.Contains(profile, "-pix_fmt yuv420p") {
				t.Fatal("lowered final render settings", profile)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
