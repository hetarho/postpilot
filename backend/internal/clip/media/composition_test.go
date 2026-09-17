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

func TestCompositionMergeBoundsVideoDecodersAndKeepsAudioForFinalPass(t *testing.T) {
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
			// Every boundary fades, which is the densest merge the renderer can
			// be asked for: each round has to carry its own overlaps.
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
			// 100 cuts merge in rounds of six: 17 passes, then 3, then the
			// delivery encode takes what is left. A pairwise tree needed 98.
			if len(fake.calls) != 20 {
				t.Fatal("wrong merge shape", len(fake.calls))
			}
			for _, call := range fake.calls {
				joined := strings.Join(call.Args, " ")
				if inputs := strings.Count(joined, ":v:0]"); inputs < 2 || inputs > r.cfg.MergeInputs {
					t.Fatal("a merge left the input bound", inputs, joined)
				}
				if strings.Contains(joined, ":a:0]") || !strings.Contains(joined, "-crf 0") || !strings.Contains(joined, "-pix_fmt yuv444p") || !strings.Contains(joined, "-threads 1") || !strings.Contains(joined, "-preset ultrafast") {
					t.Fatal("unbounded/lossy intermediate", joined)
				}
			}
			graph := args[slices.Index(args, "-filter_complex")+1]
			if inputs := strings.Count(graph, ":v:0]"); inputs != 3 || !strings.Contains(graph, "trim=end_frame=1506,") {
				t.Fatal("lost timeline", inputs, graph)
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
			if remaining != 3 {
				t.Fatal("consumed merge nodes remain", remaining)
			}
			profile := strings.Join(r.encodeArgs(audio), " ")
			if !strings.Contains(profile, "-crf 20") || !strings.Contains(profile, "-pix_fmt yuv420p") || !strings.Contains(profile, "-preset veryfast") {
				t.Fatal("lowered final render settings", profile)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// The merge plan itself is golden: how many passes a plan takes, how many
// inputs each one opens and the graph each one runs, for hard cuts only, fades
// only and both, at the input bound and above it. A change to the rounds is
// visible here without running a render (CDS-36, CLIP-124).
func TestFootageMergePlanGoldens(t *testing.T) {
	for _, plan := range []struct {
		name       string
		cuts       int
		transition func(int) int
	}{
		{"cut", 6, func(int) int { return 0 }},
		{"cut", 11, func(int) int { return 0 }},
		{"fade", 6, func(int) int { return 200 }},
		{"fade", 11, func(int) int { return 200 }},
		{"mixed", 6, func(i int) int { return 200 * (i % 2) }},
		{"mixed", 11, func(i int) int { return 200 * (i % 2) }},
	} {
		t.Run(fmt.Sprintf("%s-%d", plan.name, plan.cuts), func(t *testing.T) {
			fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
				return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("node"), 0600)
			}}
			a := newAdapter(t, fake)
			r := testRenderer(t, a)
			if err := a.WithWorkspace(t.Context(), "merge-plan", func(ws clip.MediaWorkspace) error {
				cuts, frames, transitions := make([]string, plan.cuts), make([]int, plan.cuts), make([]int, plan.cuts)
				for i := range cuts {
					cuts[i] = filepath.Join(ws.Path, fmt.Sprintf("render-cut-%04d.mp4", i))
					frames[i] = 60 + i
					if i > 0 {
						transitions[i] = plan.transition(i)
					}
					if err := os.WriteFile(cuts[i], []byte("cut"), 0600); err != nil {
						return err
					}
				}
				var cleanup []string
				args, err := r.compositionInputs(t.Context(), ws, cuts, frames, transitions, "", &loudness{}, &cleanup)
				if err != nil {
					return err
				}
				var recorded strings.Builder
				for _, pass := range append(fake.calls, Command{Args: args}) {
					graph := pass.Args[slices.Index(pass.Args, "-filter_complex")+1]
					fmt.Fprintf(&recorded, "pass inputs=%d\n%s\n", strings.Count(strings.Join(pass.Args, " "), " -i "), graph)
				}
				golden(t, fmt.Sprintf("merge-%s-%d.filter", plan.name, plan.cuts), recorded.String())
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The merge tree is where a bounded workspace is won or lost. A leaf the tree
// has already read is dead, but the render used to hold every one of them until
// after the root encode: leaves, merges and the growing root were three lossless
// copies of one clip in a directory sized for about two.
func TestConsumedFootageIsFreedBeforeTheRootEncode(t *testing.T) {
	fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("video"), 0600)
	}}
	a := newAdapter(t, fake)
	r := testRenderer(t, a)
	if err := a.WithWorkspace(t.Context(), "consumed", func(ws clip.MediaWorkspace) error {
		// Forty leaves take two rounds above the bound of six, so this covers a
		// merge reading leaves and a merge reading other merges.
		leaves, frames, transitions := make([]string, 40), make([]int, 40), make([]int, 40)
		for i := range leaves {
			leaves[i] = filepath.Join(ws.Path, fmt.Sprintf("bare-%04d.mp4", i))
			frames[i] = 21
			if err := os.WriteFile(leaves[i], []byte("leaf"), 0600); err != nil {
				return err
			}
		}
		var cleanup []string
		args, err := r.compositionInputsFormat(t.Context(), ws, leaves, frames, transitions, "", nil, &cleanup, "yuv444p")
		if err != nil {
			return err
		}
		if err := removeConsumed(append(append([]string{}, leaves...), cleanup...), args); err != nil {
			return err
		}
		alive := map[string]bool{}
		for i, arg := range args {
			if arg == "-i" {
				alive[filepath.Base(args[i+1])] = true
			}
		}
		if len(alive) == 0 || len(alive) > r.cfg.MergeInputs {
			return fmt.Errorf("wrong merge shape: %d branches", len(alive))
		}
		entries, err := os.ReadDir(ws.Path)
		if err != nil {
			return err
		}
		// Only what the root encode still has to read may exist by now.
		for _, entry := range entries {
			if !alive[entry.Name()] {
				return fmt.Errorf("a consumed input outlived its merge: %s", entry.Name())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
