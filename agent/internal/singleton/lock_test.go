package singleton

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAcquireRejectsASecondProcessLockAndReleasesCleanly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}
