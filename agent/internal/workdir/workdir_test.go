package workdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupRemovesCrashLeftoversOnlyInsideJobsRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "must-remain")
	if err := os.WriteFile(outside, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	dir, err := Create(root, "connection", "job")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	draftDir := filepath.Join(root, "unfinished-draft")
	if err := os.MkdirAll(draftDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Cleanup(root); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "connection"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("connection work dir entries=%v err=%v", entries, err)
	}
	if info, err := os.Stat(draftDir); err != nil || !info.IsDir() {
		t.Fatalf("empty draft work dir was removed: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("cleanup escaped jobs root: %v", err)
	}
	if err := Cleanup("relative/jobs"); err == nil {
		t.Fatal("relative cleanup root was accepted")
	}
}

func TestCreateRejectsServerPathEscape(t *testing.T) {
	root := t.TempDir()
	for _, input := range [][2]string{{"../other", "job"}, {"connection", "../job"}, {"/tmp", "job"}, {"connection", "nested/job"}} {
		if _, err := Create(root, input[0], input[1]); err == nil {
			t.Fatalf("unsafe identifiers %q/%q accepted", input[0], input[1])
		}
	}
	if dir, err := Create(root, "connection", "job"); err != nil || filepath.Dir(dir) != filepath.Join(root, "connection") {
		t.Fatalf("safe job dir=%q err=%v", dir, err)
	}
}

func TestSafeJoinRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	path, err := SafeJoin(root, "0001.jpg")
	if err != nil || path != filepath.Join(root, "0001.jpg") {
		t.Fatalf("safe path = %q, %v", path, err)
	}
	for _, filename := range []string{"../secret", "nested/photo.jpg", "/tmp/photo.jpg", ""} {
		if _, err := SafeJoin(root, filename); err == nil {
			t.Fatalf("unsafe filename %q accepted", filename)
		}
	}
}
