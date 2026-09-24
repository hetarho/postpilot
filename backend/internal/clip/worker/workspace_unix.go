//go:build unix

package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// A worker owns only its identity's directory, even if two configured roots
// share one local volume. The API's direct-child sweep cannot enter it.
func WorkRoot(base, identity string) string {
	digest := sha256.Sum256([]byte(identity))
	return filepath.Join(base, "worker-"+hex.EncodeToString(digest[:]))
}

// A live process keeps this advisory lock until its drain is complete. A crash
// releases it in the kernel, allowing the successor to collect abandoned files.
func LockWorkRoot(root string) (func(), error) {
	fd, err := unix.Open(filepath.Join(root, ".worker-lock"), unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, errors.New("media worker root lock unavailable")
	}
	f := os.NewFile(uintptr(fd), "worker root lock")
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, errors.New("media worker identity already owns this workspace")
	}
	return func() { _ = unix.Flock(fd, unix.LOCK_UN); _ = f.Close() }, nil
}

// Air remains alive after its child exits. Health must observe the running
// worker's kernel lock, not merely prove that another CLI can reach the API.
func CheckWorkRootActive(root string) error {
	fd, err := unix.Open(filepath.Join(root, ".worker-lock"), unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return errors.New("media worker is not running")
	}
	defer unix.Close(fd)
	err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return nil
	}
	if err == nil {
		_ = unix.Flock(fd, unix.LOCK_UN)
	}
	return errors.New("media worker is not running")
}
