//go:build linux || darwin

package media

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func ownProcessGroup(cmd *exec.Cmd) error {
	if err := enableChildReaper(); err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}
func stopProcessGroup(cmd *exec.Cmd, bound time.Duration) {
	// Also kill a descendant left behind by an otherwise successful parent.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	reapChildren(cmd.Process.Pid, bound)
}
