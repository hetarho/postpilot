package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotRetainsKernelPeakAndCountsNestedWorkspaceBytes(t *testing.T) {
	root := t.TempDir()
	peak := filepath.Join(root, "memory.peak")
	if err := os.WriteFile(peak, []byte(" 268435456\n"), 0600); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(filepath.Join(work, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"source.mp4": "12345", "nested/result.mp4": "1234567"} {
		if err := os.WriteFile(filepath.Join(work, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := snapshot(peak, []string{work, filepath.Join(root, "absent")})
	if err != nil {
		t.Fatal(err)
	}
	if report.MemoryPeak != 268435456 || report.DiskBytes != 12 {
		t.Fatalf("lost actual cgroup peak or workspace bytes: %+v", report)
	}
	raw, err := json.Marshal(report)
	if err != nil || string(raw) != `{"memory_peak":268435456,"disk_bytes":12}` {
		t.Fatalf("resource report contract changed: %s, %v", raw, err)
	}
}

func TestSnapshotRejectsMissingOrInvalidMemoryEvidence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "memory.peak")
	if _, err := snapshot(path, nil); err == nil {
		t.Fatal("missing cgroup evidence accepted")
	}
	for _, value := range []string{"", "0", "-1", "max", "18446744073709551616"} {
		t.Run(value, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := snapshot(path, nil); err == nil {
				t.Fatal("invalid cgroup evidence accepted")
			}
		})
	}
}
