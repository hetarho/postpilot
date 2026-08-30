package workdir

import (
	"errors"
	"os"
	"path/filepath"
)

// Cleanup removes only per-connection work directories beneath the configured
// jobs root. It is run before polling so payloads left by a crash or power loss
// do not survive the next agent start.
func Cleanup(root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("jobs root must be an absolute path")
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func Create(root, connectionID, jobID string) (string, error) {
	if !safeSegment(connectionID) || !safeSegment(jobID) {
		return "", errors.New("unsafe connection or job id")
	}
	parent := filepath.Join(root, connectionID)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(parent, jobID+"-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func safeSegment(value string) bool {
	return value != "" && value != "." && value != ".." && filepath.Base(value) == value
}

func SafeJoin(root, filename string) (string, error) {
	if filename == "" || filepath.Base(filename) != filename {
		return "", errors.New("unsafe staged filename")
	}
	target := filepath.Join(root, filename)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return "", errors.New("staged path escapes job directory")
	}
	return target, nil
}
