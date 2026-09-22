package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

type cleanupStoreFake struct {
	snapshot       RetirementSnapshot
	assetKeys      []string
	deleteErrCount int
	deleteCalls    int
}

func (s *cleanupStoreFake) RetirementSnapshot(context.Context) (RetirementSnapshot, error) {
	return s.snapshot, nil
}

func (s *cleanupStoreFake) RetirementAssetKeys(context.Context) ([]string, error) {
	return append([]string{}, s.assetKeys...), nil
}

func (s *cleanupStoreFake) DeleteRetirementRows(context.Context) error {
	s.deleteCalls++
	if s.deleteErrCount > 0 {
		s.deleteErrCount--
		return errors.New("sqlite busy")
	}
	s.snapshot = RetirementSnapshot{CutoffAt: s.snapshot.CutoffAt}
	s.assetKeys = nil
	return nil
}

type cleanupObjectsFake struct {
	objects        map[string][]byte
	deleteFailures map[string]int
	listFailures   map[int]error
	listedOverride []StagedObject
	listCalls      int
	deleteCalls    []string
}

func (f *cleanupObjectsFake) ListStaged(context.Context, string) ([]StagedObject, error) {
	f.listCalls++
	if err := f.listFailures[f.listCalls]; err != nil {
		return nil, err
	}
	if f.listedOverride != nil {
		return append([]StagedObject{}, f.listedOverride...), nil
	}
	var found []StagedObject
	for key := range f.objects {
		if strings.HasPrefix(key, retirementObjectPrefix) {
			found = append(found, StagedObject{Key: key})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Key < found[j].Key })
	return found, nil
}

func (f *cleanupObjectsFake) Delete(_ context.Context, key string) error {
	f.deleteCalls = append(f.deleteCalls, key)
	if f.deleteFailures[key] > 0 {
		f.deleteFailures[key]--
		return errors.New("object delete failed")
	}
	delete(f.objects, key)
	return nil
}

type cleanupFixture struct {
	store         *cleanupStoreFake
	objects       *cleanupObjectsFake
	cleaner       RetirementCleaner
	options       RetirementCleanupOptions
	report        RetirementReport
	reportBytes   []byte
	outsideBefore map[string][]byte
}

func newCleanupFixture(t *testing.T) *cleanupFixture {
	t.Helper()
	snapshot := RetirementSnapshot{
		CutoffAt: "2026-09-22 00:00:00",
		Agents: []RetirementAgent{
			{ID: "alice-agent", AccountID: "alice", Label: "Alice Mac", RevokedAt: "2026-09-22T00:00:00Z"},
			{ID: "bob-agent", AccountID: "bob", Label: "Bob Mac", RevokedAt: "2026-09-22T00:00:00Z"},
		},
		Jobs: []RetirementJob{
			{ID: "published", AccountID: "alice", PostSlug: "kept-post", PostCreatedAt: "2026-09-01T00:00:00Z", Status: "published", Stage: "published", PlatformPostURL: "https://blog.naver.com/alice/1"},
			{ID: "failed", AccountID: "bob", PostSlug: "deleted-post", PostCreatedAt: "2026-09-02T00:00:00Z", Status: "failed", Stage: "preparing", FailureReason: "PUBLISH_BROWSER_LOST"},
			{ID: "unknown", AccountID: "alice", PostSlug: "ambiguous", PostCreatedAt: "2026-09-03T00:00:00Z", Status: "outcome_unknown", Stage: "verifying", FailureReason: "PUBLISH_OUTCOME_UNKNOWN"},
			{ID: "canceled", AccountID: "bob", PostSlug: "never-started", PostCreatedAt: "2026-09-04T00:00:00Z", Status: "canceled", Stage: "queued"},
		},
		Pairings: 2, Reservations: 5, Assets: 2,
	}
	store := &cleanupStoreFake{snapshot: snapshot, assetKeys: []string{"publishing/alice/published/0000.jpg", "publishing/bob/failed/0000.jpg"}}
	objects := &cleanupObjectsFake{
		objects: map[string][]byte{
			"publishing/alice/published/0000.jpg": []byte("copy-a"),
			"publishing/bob/failed/0000.jpg":      []byte("copy-b"),
			"publishing/orphan/no-row.jpg":        []byte("orphan"),
			"posts/alice/source/photo.jpg":        []byte("source-photo-checksum"),
			"clip-inputs/alice/source.mp4":        []byte("source-video-checksum"),
		},
		deleteFailures: map[string]int{}, listFailures: map[int]error{},
	}
	report, err := BuildRetirementReport(context.Background(), store, "production", "sha256:database")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.json")
	if err := WriteRetirementReport(reportPath, report); err != nil {
		t.Fatal(err)
	}
	reportBytes, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	inventory := ShutdownInventory{
		SchemaVersion: retirementCleanupSchema, Environment: "production", ReportDigest: report.Digest,
		Devices: []ShutdownDevice{
			{AgentID: "bob-agent", Disposition: "destroyed", EvidenceDigest: "sha256:" + strings.Repeat("b", 64)},
			{AgentID: "alice-agent", Disposition: "shutdown", EvidenceDigest: "sha256:" + strings.Repeat("a", 64)},
		},
	}
	inventoryPath := filepath.Join(dir, "shutdown.json")
	writeOwnerJSON(t, inventoryPath, inventory)
	inventoryDigest, err := testFileDigest(inventoryPath)
	if err != nil {
		t.Fatal(err)
	}
	outside := map[string][]byte{}
	for _, key := range []string{"posts/alice/source/photo.jpg", "clip-inputs/alice/source.mp4"} {
		outside[key] = append([]byte{}, objects.objects[key]...)
	}
	return &cleanupFixture{
		store: store, objects: objects,
		cleaner: RetirementCleaner{Store: store, Objects: objects, Now: func() time.Time { return time.Date(2026, 9, 22, 3, 4, 5, 0, time.UTC) }},
		options: RetirementCleanupOptions{
			Mode: RetirementInspect, Environment: "production", DatabaseIdentity: "sha256:database",
			ReportPath: reportPath, ExpectedReportDigest: report.Digest, ShutdownInventoryPath: inventoryPath,
			ExpectedShutdownDigest: inventoryDigest, ReceiptPath: filepath.Join(dir, "cleanup-receipt.json"),
		},
		report: report, reportBytes: reportBytes, outsideBefore: outside,
	}
}

func TestRetirementCleanupInspectionIsReadOnlyAndApplyPreservesOriginalEvidenceAndSources(t *testing.T) {
	f := newCleanupFixture(t)
	inspected, err := f.cleaner.Run(context.Background(), f.options)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Status != "inspection" || inspected.RemainingObjects != 3 || f.store.deleteCalls != 0 || len(f.objects.deleteCalls) != 0 {
		t.Fatalf("inspection = %+v store deletes=%d object deletes=%v", inspected, f.store.deleteCalls, f.objects.deleteCalls)
	}
	if _, err := os.Stat(f.options.ReceiptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspection wrote a receipt: %v", err)
	}

	f.options.Mode = RetirementApply
	receipt, err := f.cleaner.Run(context.Background(), f.options)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "complete" || receipt.RemainingObjects != 0 || !reportCountsEmpty(receipt.RemainingCounts) || f.store.deleteCalls != 1 {
		t.Fatalf("receipt = %+v row deletes=%d", receipt, f.store.deleteCalls)
	}
	for key, want := range f.outsideBefore {
		if got := f.objects.objects[key]; !reflect.DeepEqual(got, want) {
			t.Fatalf("source object %s changed: %q want %q", key, got, want)
		}
	}
	for key := range f.objects.objects {
		if strings.HasPrefix(key, retirementObjectPrefix) {
			t.Fatalf("publishing object survived: %s", key)
		}
	}
	afterReport, err := os.ReadFile(f.options.ReportPath)
	if err != nil || !reflect.DeepEqual(afterReport, f.reportBytes) {
		t.Fatalf("original report changed: %v", err)
	}
	info, err := os.Stat(f.options.ReceiptPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("cleanup receipt mode = %v err=%v", info.Mode().Perm(), err)
	}
	if receipt.OriginalCounts != f.report.Counts || len(receipt.ShutdownDevices) != 2 || receipt.ReportDigest != f.report.Digest || receipt.Digest == "" {
		t.Fatalf("receipt omitted verification evidence: %+v", receipt)
	}

	deletes := len(f.objects.deleteCalls)
	f.options.Mode = RetirementVerify
	verified, err := f.cleaner.Run(context.Background(), f.options)
	if err != nil || verified.Digest != receipt.Digest {
		t.Fatalf("verify = %+v err=%v", verified, err)
	}
	f.options.Mode = RetirementApply
	if _, err := f.cleaner.Run(context.Background(), f.options); err != nil {
		t.Fatalf("repeated apply: %v", err)
	}
	if len(f.objects.deleteCalls) != deletes || f.store.deleteCalls != 1 {
		t.Fatal("repeated apply performed another deletion")
	}
}

func TestRetirementCleanupFailuresKeepRowsAndAReceiptForRetry(t *testing.T) {
	for name, arrange := range map[string]func(*cleanupFixture){
		"initial listing":      func(f *cleanupFixture) { f.objects.listFailures[1] = errors.New("list unavailable") },
		"one object":           func(f *cleanupFixture) { f.objects.deleteFailures["publishing/orphan/no-row.jpg"] = 1 },
		"final listing":        func(f *cleanupFixture) { f.objects.listFailures[2] = errors.New("final list unavailable") },
		"database transaction": func(f *cleanupFixture) { f.store.deleteErrCount = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			f := newCleanupFixture(t)
			arrange(f)
			f.options.Mode = RetirementApply
			receipt, err := f.cleaner.Run(context.Background(), f.options)
			if err == nil || receipt.Status != "incomplete" {
				t.Fatalf("first result = %+v err=%v", receipt, err)
			}
			if snapshotIsEmpty(f.store.snapshot) {
				t.Fatal("publishing rows were cleared before object cleanup verified")
			}
			if _, statErr := os.Stat(f.options.ReceiptPath); statErr != nil {
				t.Fatalf("recovery receipt missing: %v", statErr)
			}
			delete(f.objects.listFailures, 1)
			delete(f.objects.listFailures, 2)
			receipt, err = f.cleaner.Run(context.Background(), f.options)
			if err != nil || receipt.Status != "complete" || !snapshotIsEmpty(f.store.snapshot) {
				t.Fatalf("retry = %+v err=%v", receipt, err)
			}
		})
	}
}

func TestRepeatedCleanupNeverDowngradesACompleteReceiptOnReadFailure(t *testing.T) {
	f := newCleanupFixture(t)
	f.options.Mode = RetirementApply
	complete, err := f.cleaner.Run(context.Background(), f.options)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.options.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	f.objects.listFailures[f.objects.listCalls+1] = errors.New("temporary listing failure")
	if _, err := f.cleaner.Run(context.Background(), f.options); err == nil {
		t.Fatal("repeated cleanup hid a listing failure")
	}
	after, err := os.ReadFile(f.options.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || complete.Status != "complete" {
		t.Fatal("complete cleanup receipt was downgraded")
	}
}

func TestRetirementCleanupRefusesStaleEvidenceUnresolvedDevicesAndUnsafeKeys(t *testing.T) {
	for name, test := range map[string]struct {
		arrange func(*cleanupFixture)
		want    string
	}{
		"stale database": {
			arrange: func(f *cleanupFixture) { f.store.snapshot.Reservations++ }, want: "stale or differs",
		},
		"unresolved device": {
			arrange: func(f *cleanupFixture) {
				inventory := ShutdownInventory{SchemaVersion: 1, Environment: "production", ReportDigest: f.report.Digest, Devices: []ShutdownDevice{{AgentID: "alice-agent", Disposition: "shutdown", EvidenceDigest: "sha256:" + strings.Repeat("a", 64)}}}
				writeOwnerJSON(t, f.options.ShutdownInventoryPath, inventory)
				f.options.ExpectedShutdownDigest, _ = testFileDigest(f.options.ShutdownInventoryPath)
			}, want: "every reported companion",
		},
		"out of prefix reference": {
			arrange: func(f *cleanupFixture) { f.store.assetKeys = append(f.store.assetKeys, "posts/alice/source/photo.jpg") }, want: "unsafe referenced",
		},
		"malicious listed key": {
			arrange: func(f *cleanupFixture) {
				f.objects.listedOverride = []StagedObject{{Key: "publishing/../posts/alice/source.jpg"}}
			}, want: "unsafe listed",
		},
		"report digest mismatch": {
			arrange: func(f *cleanupFixture) { f.options.ExpectedReportDigest = strings.Repeat("0", 64) }, want: "expected digest",
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newCleanupFixture(t)
			test.arrange(f)
			f.options.Mode = RetirementApply
			_, err := f.cleaner.Run(context.Background(), f.options)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if f.store.deleteCalls != 0 || len(f.objects.deleteCalls) != 0 {
				t.Fatal("unsafe evidence caused deletion")
			}
			for key, expected := range f.outsideBefore {
				if !reflect.DeepEqual(f.objects.objects[key], expected) {
					t.Fatalf("source %s changed", key)
				}
			}
		})
	}
}

func TestRetirementCleanupAcceptsEmptyInstallation(t *testing.T) {
	f := newCleanupFixture(t)
	f.store.snapshot = RetirementSnapshot{CutoffAt: f.store.snapshot.CutoffAt}
	f.store.assetKeys = nil
	report, err := BuildRetirementReport(context.Background(), f.store, "production", "sha256:database")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.options.ReportPath); err != nil {
		t.Fatal(err)
	}
	if err := WriteRetirementReport(f.options.ReportPath, report); err != nil {
		t.Fatal(err)
	}
	f.options.ExpectedReportDigest = report.Digest
	writeOwnerJSON(t, f.options.ShutdownInventoryPath, ShutdownInventory{SchemaVersion: 1, Environment: "production", ReportDigest: report.Digest, Devices: []ShutdownDevice{}})
	f.options.ExpectedShutdownDigest, _ = testFileDigest(f.options.ShutdownInventoryPath)
	for key := range f.objects.objects {
		if strings.HasPrefix(key, retirementObjectPrefix) {
			delete(f.objects.objects, key)
		}
	}
	f.options.Mode = RetirementApply
	receipt, err := f.cleaner.Run(context.Background(), f.options)
	if err != nil || receipt.Status != "complete" || f.store.deleteCalls != 0 {
		t.Fatalf("empty cleanup = %+v deletes=%d err=%v", receipt, f.store.deleteCalls, err)
	}
}

func writeOwnerJSON(t *testing.T, filePath string, value any) {
	t.Helper()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, append(payload, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func testFileDigest(filePath string) (string, error) {
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return digestBytes(payload), nil
}
