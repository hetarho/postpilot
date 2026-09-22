package publishing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const (
	retirementCleanupSchema = 1
	retirementObjectPrefix  = "publishing/"
)

type RetirementCleanupStore interface {
	RetirementSnapshot(context.Context) (RetirementSnapshot, error)
	RetirementAssetKeys(context.Context) ([]string, error)
	DeleteRetirementRows(context.Context) error
}

type RetirementObjectStore interface {
	ListStaged(context.Context, string) ([]StagedObject, error)
	Delete(context.Context, string) error
}

type RetirementCleanupMode string

const (
	RetirementInspect RetirementCleanupMode = "inspect"
	RetirementApply   RetirementCleanupMode = "apply"
	RetirementVerify  RetirementCleanupMode = "verify"
)

type ShutdownDevice struct {
	AgentID        string `json:"agent_id"`
	Disposition    string `json:"disposition"`
	EvidenceDigest string `json:"evidence_digest"`
}

type ShutdownInventory struct {
	SchemaVersion int              `json:"schema_version"`
	Environment   string           `json:"environment"`
	ReportDigest  string           `json:"report_digest"`
	Devices       []ShutdownDevice `json:"devices"`
}

type RetirementCleanupOptions struct {
	Mode                   RetirementCleanupMode
	Environment            string
	DatabaseIdentity       string
	ReportPath             string
	ExpectedReportDigest   string
	ShutdownInventoryPath  string
	ExpectedShutdownDigest string
	ReceiptPath            string
}

type RetirementCleanupReceipt struct {
	SchemaVersion           int              `json:"schema_version"`
	Status                  string           `json:"status"`
	Environment             string           `json:"environment"`
	DatabaseIdentity        string           `json:"database_identity"`
	CutoffAt                string           `json:"cutoff_at"`
	ReportDigest            string           `json:"report_digest"`
	ShutdownInventoryDigest string           `json:"shutdown_inventory_digest"`
	ShutdownDevices         []ShutdownDevice `json:"shutdown_devices"`
	OriginalCounts          RetirementCounts `json:"original_counts"`
	ObjectKeys              []string         `json:"object_keys,omitempty"`
	RemainingCounts         RetirementCounts `json:"remaining_counts"`
	RemainingObjects        int64            `json:"remaining_objects"`
	StartedAt               time.Time        `json:"started_at,omitempty"`
	CompletedAt             *time.Time       `json:"completed_at,omitempty"`
	Failures                []string         `json:"failures,omitempty"`
	Digest                  string           `json:"digest"`
}

type RetirementCleaner struct {
	Store   RetirementCleanupStore
	Objects RetirementObjectStore
	Now     func() time.Time
}

func (c RetirementCleaner) Run(ctx context.Context, options RetirementCleanupOptions) (RetirementCleanupReceipt, error) {
	if c.Store == nil || c.Objects == nil {
		return RetirementCleanupReceipt{}, errors.New("retirement cleanup dependencies are incomplete")
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if options.Mode == "" {
		options.Mode = RetirementInspect
	}
	if options.Mode != RetirementInspect && options.Mode != RetirementApply && options.Mode != RetirementVerify {
		return RetirementCleanupReceipt{}, fmt.Errorf("unknown retirement cleanup mode %q", options.Mode)
	}

	report, err := readAndVerifyRetirementReport(options)
	if err != nil {
		return RetirementCleanupReceipt{}, err
	}
	inventory, shutdownDigest, err := readAndVerifyShutdownInventory(options, report)
	if err != nil {
		return RetirementCleanupReceipt{}, err
	}
	existing, exists, err := readCleanupReceipt(options.ReceiptPath)
	if err != nil {
		return RetirementCleanupReceipt{}, err
	}
	if exists {
		if err := validateCleanupReceipt(existing, report, shutdownDigest, inventory); err != nil {
			return RetirementCleanupReceipt{}, err
		}
	}

	snapshot, err := c.Store.RetirementSnapshot(ctx)
	if err != nil {
		return RetirementCleanupReceipt{}, fmt.Errorf("read current retirement snapshot: %w", err)
	}
	state, err := classifyRetirementState(snapshot, report, existing, exists)
	if err != nil {
		return RetirementCleanupReceipt{}, err
	}
	assetKeys, err := c.Store.RetirementAssetKeys(ctx)
	if err != nil {
		return RetirementCleanupReceipt{}, fmt.Errorf("read publishing asset inventory: %w", err)
	}
	if err := validatePublishingKeys(assetKeys); err != nil {
		return RetirementCleanupReceipt{}, fmt.Errorf("unsafe referenced publishing asset: %w", err)
	}

	receipt := newCleanupReceipt(report, inventory, shutdownDigest, snapshotCounts(snapshot), c.Now().UTC())
	if exists {
		receipt = existing
		receipt.Failures = nil
		receipt.CompletedAt = nil
		receipt.RemainingCounts = snapshotCounts(snapshot)
	}
	receipt.ObjectKeys = mergeSorted(receipt.ObjectKeys, assetKeys)

	objects, listErr := c.Objects.ListStaged(ctx, retirementObjectPrefix)
	if listErr != nil {
		failure := fmt.Sprintf("list all %s objects: %v", retirementObjectPrefix, listErr)
		if options.Mode == RetirementApply && !(exists && existing.Status == "complete") {
			receipt.Status = "incomplete"
			receipt.Failures = []string{failure}
			if writeErr := writeCleanupReceipt(options.ReceiptPath, &receipt); writeErr != nil {
				return receipt, errors.Join(errors.New(failure), writeErr)
			}
		}
		return receipt, errors.New(failure)
	}
	listedKeys := stagedKeys(objects)
	if err := validatePublishingKeys(listedKeys); err != nil {
		failure := fmt.Sprintf("unsafe listed publishing object: %v", err)
		if options.Mode == RetirementApply && !(exists && existing.Status == "complete") {
			receipt.Status = "incomplete"
			receipt.Failures = []string{failure}
			if writeErr := writeCleanupReceipt(options.ReceiptPath, &receipt); writeErr != nil {
				return receipt, errors.Join(errors.New(failure), writeErr)
			}
		}
		return receipt, errors.New(failure)
	}
	receipt.ObjectKeys = mergeSorted(receipt.ObjectKeys, listedKeys)
	receipt.RemainingObjects = int64(len(listedKeys))

	if options.Mode == RetirementInspect {
		receipt.Status = "inspection"
		receipt.Digest = ""
		return receipt, nil
	}
	if options.Mode == RetirementVerify {
		if !exists || existing.Status != "complete" {
			return receipt, errors.New("verification requires a complete cleanup receipt")
		}
		if state != "empty" || len(listedKeys) != 0 {
			return receipt, errors.New("cleanup verification found remaining publishing rows or objects")
		}
		return existing, nil
	}
	if exists && existing.Status == "complete" {
		if state != "empty" || len(listedKeys) != 0 {
			return receipt, errors.New("complete cleanup receipt disagrees with current publishing state")
		}
		return existing, nil
	}

	receipt.Status = "in_progress"
	if receipt.StartedAt.IsZero() {
		receipt.StartedAt = c.Now().UTC()
	}
	if err := writeCleanupReceipt(options.ReceiptPath, &receipt); err != nil {
		return receipt, fmt.Errorf("persist cleanup inventory before deletion: %w", err)
	}

	var deleteErr error
	for _, key := range receipt.ObjectKeys {
		if err := c.Objects.Delete(ctx, key); err != nil {
			deleteErr = errors.Join(deleteErr, fmt.Errorf("delete %s: %w", key, err))
		}
	}
	if deleteErr != nil {
		receipt.Status = "incomplete"
		receipt.Failures = []string{deleteErr.Error()}
		if err := writeCleanupReceipt(options.ReceiptPath, &receipt); err != nil {
			return receipt, errors.Join(deleteErr, err)
		}
		return receipt, deleteErr
	}

	remaining, err := c.Objects.ListStaged(ctx, retirementObjectPrefix)
	if err != nil {
		return c.failAfterMutation(options.ReceiptPath, receipt, fmt.Errorf("verify publishing object deletion: %w", err))
	}
	remainingKeys := stagedKeys(remaining)
	if err := validatePublishingKeys(remainingKeys); err != nil {
		return c.failAfterMutation(options.ReceiptPath, receipt, fmt.Errorf("unsafe object in final publishing listing: %w", err))
	}
	if len(remainingKeys) != 0 {
		receipt.ObjectKeys = mergeSorted(receipt.ObjectKeys, remainingKeys)
		receipt.RemainingObjects = int64(len(remainingKeys))
		return c.failAfterMutation(options.ReceiptPath, receipt, fmt.Errorf("publishing objects remain after deletion: %d", len(remainingKeys)))
	}
	receipt.RemainingObjects = 0

	if state != "empty" {
		if err := c.Store.DeleteRetirementRows(ctx); err != nil {
			return c.failAfterMutation(options.ReceiptPath, receipt, fmt.Errorf("delete publishing rows: %w", err))
		}
	}
	verified, err := c.Store.RetirementSnapshot(ctx)
	if err != nil {
		return c.failAfterMutation(options.ReceiptPath, receipt, fmt.Errorf("verify publishing rows: %w", err))
	}
	if !snapshotIsEmpty(verified) || verified.CutoffAt != report.CutoffAt {
		return c.failAfterMutation(options.ReceiptPath, receipt, errors.New("publishing rows remain after cleanup"))
	}
	finalObjects, err := c.Objects.ListStaged(ctx, retirementObjectPrefix)
	if err != nil {
		return c.failAfterMutation(options.ReceiptPath, receipt, fmt.Errorf("final publishing object verification: %w", err))
	}
	finalKeys := stagedKeys(finalObjects)
	if err := validatePublishingKeys(finalKeys); err != nil || len(finalKeys) != 0 {
		if err == nil {
			err = fmt.Errorf("%d objects remain", len(finalKeys))
		}
		return c.failAfterMutation(options.ReceiptPath, receipt, fmt.Errorf("final publishing object verification: %w", err))
	}

	receipt.Status = "complete"
	receipt.Failures = nil
	receipt.RemainingCounts = RetirementCounts{}
	receipt.RemainingObjects = 0
	completed := c.Now().UTC()
	receipt.CompletedAt = &completed
	if err := writeCleanupReceipt(options.ReceiptPath, &receipt); err != nil {
		return receipt, fmt.Errorf("write complete cleanup receipt: %w", err)
	}
	return receipt, nil
}

func (c RetirementCleaner) failAfterMutation(receiptPath string, receipt RetirementCleanupReceipt, failure error) (RetirementCleanupReceipt, error) {
	receipt.Status = "incomplete"
	receipt.Failures = []string{failure.Error()}
	if err := writeCleanupReceipt(receiptPath, &receipt); err != nil {
		return receipt, errors.Join(failure, err)
	}
	return receipt, failure
}

func readAndVerifyRetirementReport(options RetirementCleanupOptions) (RetirementReport, error) {
	var report RetirementReport
	if err := readOwnerOnlyJSON(options.ReportPath, &report); err != nil {
		return report, fmt.Errorf("read original retirement report: %w", err)
	}
	if report.SchemaVersion != 1 || report.MigrationVersion != RetirementMigrationVersion {
		return report, errors.New("retirement report schema or cutoff migration is unsupported")
	}
	if report.Environment != options.Environment || report.DatabaseIdentity != options.DatabaseIdentity {
		return report, errors.New("retirement report environment or database identity does not match this deployment")
	}
	if report.CutoffAt == "" || report.Digest == "" || report.Digest != strings.TrimSpace(options.ExpectedReportDigest) {
		return report, errors.New("retirement report cutoff or expected digest is missing/mismatched")
	}
	want, err := digestJSON(reportWithoutDigest(report))
	if err != nil {
		return report, err
	}
	if report.Digest != want {
		return report, errors.New("retirement report content does not match its digest")
	}
	if report.Counts.Agents != int64(len(report.Agents)) || report.Counts.Jobs != int64(len(report.Jobs)) {
		return report, errors.New("retirement report counts do not match its record inventory")
	}
	if report.Counts.Pairings < 0 || report.Counts.Agents < 0 || report.Counts.Reservations < 0 || report.Counts.Jobs < 0 || report.Counts.Assets < 0 {
		return report, errors.New("retirement report contains a negative row count")
	}
	for _, job := range report.Jobs {
		switch job.Status {
		case "published", "failed", "outcome_unknown", "canceled":
		default:
			return report, fmt.Errorf("retirement report job %q is not terminal", job.ID)
		}
	}
	return report, nil
}

func reportWithoutDigest(report RetirementReport) RetirementReport {
	report.Digest = ""
	return report
}

func readAndVerifyShutdownInventory(options RetirementCleanupOptions, report RetirementReport) (ShutdownInventory, string, error) {
	var inventory ShutdownInventory
	payload, err := readOwnerOnlyJSONBytes(options.ShutdownInventoryPath, &inventory)
	if err != nil {
		return inventory, "", fmt.Errorf("read shutdown inventory: %w", err)
	}
	digest := digestBytes(payload)
	if digest != strings.TrimSpace(options.ExpectedShutdownDigest) {
		return inventory, digest, errors.New("shutdown inventory file digest does not match the expected digest")
	}
	if inventory.SchemaVersion != retirementCleanupSchema || inventory.Environment != report.Environment || inventory.ReportDigest != report.Digest {
		return inventory, digest, errors.New("shutdown inventory does not match the retirement report")
	}
	expected := make(map[string]struct{}, len(report.Agents))
	for _, agent := range report.Agents {
		if agent.ID == "" {
			return inventory, digest, errors.New("retirement report contains an agent without an id")
		}
		expected[agent.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(inventory.Devices))
	for _, device := range inventory.Devices {
		if _, ok := expected[device.AgentID]; !ok {
			return inventory, digest, fmt.Errorf("shutdown inventory contains unknown agent %q", device.AgentID)
		}
		if _, duplicate := seen[device.AgentID]; duplicate {
			return inventory, digest, fmt.Errorf("shutdown inventory repeats agent %q", device.AgentID)
		}
		seen[device.AgentID] = struct{}{}
		switch device.Disposition {
		case "shutdown", "never_installed", "destroyed":
		default:
			return inventory, digest, fmt.Errorf("agent %q has unresolved disposition %q", device.AgentID, device.Disposition)
		}
		if !strings.HasPrefix(device.EvidenceDigest, "sha256:") || len(device.EvidenceDigest) != len("sha256:")+64 {
			return inventory, digest, fmt.Errorf("agent %q has no valid evidence digest", device.AgentID)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(device.EvidenceDigest, "sha256:")); err != nil {
			return inventory, digest, fmt.Errorf("agent %q has a malformed evidence digest", device.AgentID)
		}
	}
	if len(seen) != len(expected) {
		return inventory, digest, errors.New("shutdown inventory does not reconcile every reported companion")
	}
	sort.Slice(inventory.Devices, func(i, j int) bool { return inventory.Devices[i].AgentID < inventory.Devices[j].AgentID })
	return inventory, digest, nil
}

func classifyRetirementState(snapshot RetirementSnapshot, report RetirementReport, receipt RetirementCleanupReceipt, receiptExists bool) (string, error) {
	if snapshot.CutoffAt != report.CutoffAt {
		return "", errors.New("current retirement cutoff does not match the original report")
	}
	if snapshotIsEmpty(snapshot) && reportCountsEmpty(report.Counts) {
		return "empty", nil
	}
	if retirementSnapshotMatchesReport(snapshot, report) {
		return "original", nil
	}
	if snapshotIsEmpty(snapshot) {
		if reportCountsEmpty(report.Counts) || receiptExists && (receipt.Status == "in_progress" || receipt.Status == "incomplete" || receipt.Status == "complete") {
			return "empty", nil
		}
		return "", errors.New("publishing rows disappeared without a matching cleanup receipt")
	}
	return "", errors.New("current publishing inventory is stale or differs from the original report")
}

func retirementSnapshotMatchesReport(snapshot RetirementSnapshot, report RetirementReport) bool {
	return snapshot.CutoffAt == report.CutoffAt && snapshotCounts(snapshot) == report.Counts && reflect.DeepEqual(snapshot.Agents, report.Agents) && reflect.DeepEqual(snapshot.Jobs, report.Jobs)
}

func snapshotCounts(snapshot RetirementSnapshot) RetirementCounts {
	return RetirementCounts{Pairings: snapshot.Pairings, Agents: int64(len(snapshot.Agents)), Reservations: snapshot.Reservations, Jobs: int64(len(snapshot.Jobs)), Assets: snapshot.Assets}
}

func snapshotIsEmpty(snapshot RetirementSnapshot) bool {
	return reportCountsEmpty(snapshotCounts(snapshot))
}

func reportCountsEmpty(counts RetirementCounts) bool {
	return counts.Pairings == 0 && counts.Agents == 0 && counts.Reservations == 0 && counts.Jobs == 0 && counts.Assets == 0
}

func validatePublishingKeys(keys []string) error {
	for _, key := range keys {
		if key == retirementObjectPrefix || !strings.HasPrefix(key, retirementObjectPrefix) || path.Clean(key) != key || strings.Contains(key, `\`) {
			return fmt.Errorf("key %q is outside the exact %s namespace", key, retirementObjectPrefix)
		}
	}
	return nil
}

func stagedKeys(objects []StagedObject) []string {
	keys := make([]string, 0, len(objects))
	for _, object := range objects {
		keys = append(keys, object.Key)
	}
	return sortedUniqueStrings(keys)
}

func mergeSorted(left, right []string) []string {
	return sortedUniqueStrings(append(append([]string{}, left...), right...))
}

func sortedUniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func newCleanupReceipt(report RetirementReport, inventory ShutdownInventory, shutdownDigest string, remaining RetirementCounts, now time.Time) RetirementCleanupReceipt {
	return RetirementCleanupReceipt{
		SchemaVersion: retirementCleanupSchema, Status: "inspection", Environment: report.Environment,
		DatabaseIdentity: report.DatabaseIdentity, CutoffAt: report.CutoffAt, ReportDigest: report.Digest,
		ShutdownInventoryDigest: shutdownDigest, ShutdownDevices: inventory.Devices, OriginalCounts: report.Counts,
		RemainingCounts: remaining, StartedAt: now,
	}
}

func validateCleanupReceipt(receipt RetirementCleanupReceipt, report RetirementReport, shutdownDigest string, inventory ShutdownInventory) error {
	if receipt.SchemaVersion != retirementCleanupSchema || receipt.Environment != report.Environment || receipt.DatabaseIdentity != report.DatabaseIdentity || receipt.CutoffAt != report.CutoffAt || receipt.ReportDigest != report.Digest || receipt.ShutdownInventoryDigest != shutdownDigest || !reflect.DeepEqual(receipt.ShutdownDevices, inventory.Devices) || receipt.OriginalCounts != report.Counts {
		return errors.New("existing cleanup receipt does not match current retirement evidence")
	}
	want, err := cleanupReceiptDigest(receipt)
	if err != nil || receipt.Digest == "" || receipt.Digest != want {
		return errors.New("existing cleanup receipt digest is invalid")
	}
	if err := validatePublishingKeys(receipt.ObjectKeys); err != nil {
		return fmt.Errorf("existing cleanup receipt is unsafe: %w", err)
	}
	switch receipt.Status {
	case "in_progress", "incomplete":
		if receipt.CompletedAt != nil {
			return errors.New("incomplete cleanup receipt has a completion timestamp")
		}
	case "complete":
		if receipt.CompletedAt == nil || !reportCountsEmpty(receipt.RemainingCounts) || receipt.RemainingObjects != 0 || len(receipt.Failures) != 0 {
			return errors.New("complete cleanup receipt contains unfinished work")
		}
	default:
		return fmt.Errorf("existing cleanup receipt has unknown status %q", receipt.Status)
	}
	return nil
}

func readCleanupReceipt(path string) (RetirementCleanupReceipt, bool, error) {
	var receipt RetirementCleanupReceipt
	if strings.TrimSpace(path) == "" {
		return receipt, false, errors.New("cleanup receipt path is required")
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return receipt, false, nil
	} else if err != nil {
		return receipt, false, err
	}
	if err := readOwnerOnlyJSON(path, &receipt); err != nil {
		return receipt, false, fmt.Errorf("read cleanup receipt without overwriting it: %w", err)
	}
	return receipt, true, nil
}

func writeCleanupReceipt(filePath string, receipt *RetirementCleanupReceipt) error {
	if strings.TrimSpace(filePath) == "" {
		return errors.New("cleanup receipt path is required")
	}
	digest, err := cleanupReceiptDigest(*receipt)
	if err != nil {
		return err
	}
	receipt.Digest = digest
	payload, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".retirepublishing-cleanup-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err == nil {
		err = directory.Sync()
		directory.Close()
	}
	return err
}

func cleanupReceiptDigest(receipt RetirementCleanupReceipt) (string, error) {
	receipt.Digest = ""
	return digestJSON(receipt)
}

func digestJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func digestBytes(payload []byte) string {
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func readOwnerOnlyJSON(filePath string, target any) error {
	_, err := readOwnerOnlyJSONBytes(filePath, target)
	return err
}

func readOwnerOnlyJSONBytes(filePath string, target any) ([]byte, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, errors.New("path is required")
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("file must be regular and owner-only")
	}
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("JSON file contains trailing data")
	}
	return payload, nil
}
