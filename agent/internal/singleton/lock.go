// Package singleton owns the process-wide lock that prevents two Mac companion
// processes from controlling the same browser profiles and work directories.
package singleton

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var ErrAlreadyRunning = errors.New("postpilot agent is already running")

type Lock struct {
	file *os.File
}

func Acquire(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open agent lock: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("protect agent lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("lock agent process: %w", err)
	}
	return &Lock{file: file}, nil
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	closeErr := file.Close()
	return errors.Join(unlockErr, closeErr)
}
