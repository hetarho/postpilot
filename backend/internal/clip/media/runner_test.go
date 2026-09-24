package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
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
	case "tree":
		child := exec.Command(os.Args[0], "-test.run=^TestMediaRunnerHelper$", "--", "--media-helper", "wait")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(8)
		}
		if err := os.WriteFile(os.Args[marker+2], []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			os.Exit(7)
		}
		_ = child.Wait()
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

// Runs in the distroless execution image too, without a shell or ps utility.
func TestExecRunnerReapsDescendants(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidfile := filepath.Join(t.TempDir(), "child.pid")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := (ExecRunner{StdoutLimit: 1024, StderrLimit: 1024, WaitDelay: time.Second}).Run(ctx, Command{Binary: binary, Args: []string{"-test.run=^TestMediaRunnerHelper$", "--", "--media-helper", "tree", pidfile}})
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(pidfile); err == nil {
			pid, _ = strconv.Atoi(string(raw))
			if pid > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("helper did not start")
	}
	cancel()
	select {
	case err = <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("parent not reaped")
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("descendant not reaped", pid)
}
