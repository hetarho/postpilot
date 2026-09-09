package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMediaRunnerHelper(t *testing.T) {
	marker := slices.Index(os.Args, "--media-helper")
	if marker < 0 || len(os.Args) <= marker+1 {
		return
	}
	switch os.Args[marker+1] {
	case "large":
		fmt.Print(strings.Repeat("x", 2048))
	case "error":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 20000))
		os.Exit(9)
	case "wait":
		time.Sleep(time.Minute)
	case "args":
		fmt.Print(strings.Join(os.Args[marker+2:], "\n"))
	}
	os.Exit(0)
}
func TestExecRunnerBoundsOutputAndCancels(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := ExecRunner{StdoutLimit: 1024, StderrLimit: 128, WaitDelay: time.Second}
	cmd := func(mode string) Command {
		return Command{Binary: binary, Dir: t.TempDir(), Args: []string{"-test.run=^TestMediaRunnerHelper$", "--", "--media-helper", mode}}
	}
	if _, err := r.Run(t.Context(), cmd("large")); err == nil || !strings.Contains(err.Error(), "bound") {
		t.Fatal(err)
	}
	if _, err := r.Run(t.Context(), cmd("error")); err == nil || len(err.Error()) > 400 {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.Run(ctx, cmd("wait")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	argsCommand := cmd("args")
	argsCommand.Args = append(argsCommand.Args, "literal:$(touch NO);*'\"")
	if out, err := r.Run(t.Context(), argsCommand); err != nil || string(out) != "literal:$(touch NO);*'\"" {
		t.Fatalf("%s %v", out, err)
	}
}
