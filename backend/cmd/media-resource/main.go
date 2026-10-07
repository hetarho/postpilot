// media-resource reads the release fixture's cgroup and workspace counters.
// It intentionally imports no API, worker, codec or test initialization.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type resourceReport struct {
	MemoryPeak uint64 `json:"memory_peak"`
	DiskBytes  int64  `json:"disk_bytes"`
}

func snapshot(memoryPath string, roots []string) (resourceReport, error) {
	raw, err := os.ReadFile(memoryPath)
	if err != nil {
		return resourceReport{}, fmt.Errorf("read cgroup memory peak: %w", err)
	}
	peak, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || peak == 0 {
		return resourceReport{}, errors.New("cgroup memory peak must be a positive byte count")
	}
	report := resourceReport{MemoryPeak: peak}
	for _, root := range roots {
		err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if !info.IsDir() {
				report.DiskBytes += info.Size()
			}
			return nil
		})
		if err != nil {
			return resourceReport{}, fmt.Errorf("read fixture disk usage: %w", err)
		}
	}
	return report, nil
}

func main() {
	report, err := snapshot("/sys/fs/cgroup/memory.peak", []string{"/tmp", "/var/lib/postpilot-media"})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("RESOURCE_REPORT %s\n", raw)
}
