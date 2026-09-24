package media

import (
	"golang.org/x/sys/unix"
	"sync"
	"time"
)

var reaperOnce sync.Once
var reaperError error

func enableChildReaper() error {
	reaperOnce.Do(func() { reaperError = unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0) })
	return reaperError
}
func reapChildren(group int, bound time.Duration) {
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		pid, err := unix.Wait4(-group, nil, unix.WNOHANG, nil)
		if err == unix.ECHILD {
			return
		}
		if err != nil && err != unix.EINTR {
			return
		}
		if pid == 0 {
			time.Sleep(time.Millisecond)
		}
	}
}
