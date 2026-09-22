package launchd

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const Label = "com.postpilot.publishing-agent"

// launchctlBootout unloads one agent. It is a package variable so a test can prove the
// unload is requested without stopping a genuinely installed agent on the developer's Mac.
var launchctlBootout = func(domainTarget string) {
	_ = exec.Command("/bin/launchctl", "bootout", domainTarget).Run()
}

func domainTarget() string { return "gui/" + strconv.Itoa(os.Getuid()) + "/" + Label }

func PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

func Install(binary, logDir string) error {
	if !filepath.IsAbs(binary) || !filepath.IsAbs(logDir) {
		return errors.New("LaunchAgent binary and log directory must be absolute")
	}
	path, err := PlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(logDir, 0o700); err != nil {
		return err
	}
	if err := writePlist(path, binary, logDir); err != nil {
		return err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	launchctlBootout(domainTarget())
	return exec.Command("/bin/launchctl", "bootstrap", domain, path).Run()
}

func writePlist(path, binary, logDir string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>run</string></array>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
<key>ProcessType</key><string>Background</string>
<key>StandardOutPath</key><string>%s</string>
<key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, Label, html.EscapeString(binary), html.EscapeString(filepath.Join(logDir, "agent.log")), html.EscapeString(filepath.Join(logDir, "agent-error.log")))
	return os.WriteFile(path, []byte(plist), 0o600)
}

func Uninstall() error {
	path, err := PlistPath()
	if err != nil {
		return err
	}
	launchctlBootout(domainTarget())
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var commandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// StopAndVerify unloads the one reviewed user LaunchAgent and any manually
// started copy that can be proven to be the installed binary. It never guesses
// from a process name and never targets a browser or another Go process.
func StopAndVerify(ctx context.Context, binary, lockPath string) error {
	target := domainTarget()
	loaded, err := launchAgentLoaded(ctx, target)
	if err != nil {
		return err
	}
	if loaded {
		_, bootoutErr := commandOutput(ctx, "/bin/launchctl", "bootout", target)
		stillLoaded, verifyErr := launchAgentLoaded(ctx, target)
		if verifyErr != nil {
			return verifyErr
		}
		if stillLoaded {
			if bootoutErr != nil {
				return fmt.Errorf("stop LaunchAgent %s: %w", target, bootoutErr)
			}
			return fmt.Errorf("LaunchAgent %s is still loaded after bootout", target)
		}
	}
	if err := stopVerifiedManualRuns(ctx, binary, lockPath); err != nil {
		return err
	}
	loaded, err = launchAgentLoaded(ctx, target)
	if err != nil {
		return err
	}
	if loaded {
		return fmt.Errorf("LaunchAgent %s became loaded again during shutdown", target)
	}
	return nil
}

func launchAgentLoaded(ctx context.Context, target string) (bool, error) {
	output, err := commandOutput(ctx, "/bin/launchctl", "print", target)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	message := strings.ToLower(string(output))
	if errors.As(err, &exitErr) && (strings.Contains(message, "could not find service") || strings.Contains(message, "service not found")) {
		return false, nil
	}
	return false, fmt.Errorf("inspect LaunchAgent %s: %w (%s)", target, err, strings.TrimSpace(string(output)))
}

func stopVerifiedManualRuns(ctx context.Context, binary, lockPath string) error {
	if _, err := os.Lstat(lockPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect companion lock: %w", err)
	}
	out, err := commandOutput(ctx, "/usr/sbin/lsof", "-t", lockPath)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(strings.TrimSpace(string(out))) == 0 {
			return nil // no process currently owns the stale lock file
		}
		return fmt.Errorf("inspect companion process: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	for _, field := range strings.Fields(string(out)) {
		pid, parseErr := strconv.Atoi(field)
		if parseErr != nil || pid <= 0 {
			return fmt.Errorf("inspect companion process: unexpected pid %q", field)
		}
		command, commandErr := commandOutput(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "command=")
		if commandErr != nil {
			if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
				continue
			}
			return fmt.Errorf("inspect companion process %d: %w", pid, commandErr)
		}
		line := strings.TrimSpace(string(command))
		if line != binary && !strings.HasPrefix(line, binary+" ") {
			return fmt.Errorf("process %d owns the companion lock but is not the reviewed binary", pid)
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("stop manual companion process %d: %w", pid, err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}
		}
		if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("manual companion process %d is still running", pid)
		}
	}
	return nil
}
