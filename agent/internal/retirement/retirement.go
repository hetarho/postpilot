package retirement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/postpilot/agent/internal/config"
)

const receiptVersion = 1

type CredentialStore interface {
	Delete(context.Context, string) error
}

type OwnedPath struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	OwnerID string `json:"owner_id,omitempty"`
}

type Receipt struct {
	Version            int         `json:"version"`
	Status             string      `json:"status"`
	StartedAt          time.Time   `json:"started_at"`
	CompletedAt        *time.Time  `json:"completed_at,omitempty"`
	LaunchAgentLabel   string      `json:"launch_agent_label"`
	ShutdownVerified   bool        `json:"shutdown_verified"`
	InventoryComplete  bool        `json:"inventory_complete"`
	CredentialAccounts []string    `json:"credential_accounts,omitempty"`
	OwnedPaths         []OwnedPath `json:"owned_paths,omitempty"`
	ProfilePaths       []OwnedPath `json:"profile_paths,omitempty"`
	ObservedLocalPaths []string    `json:"observed_local_paths,omitempty"`
	ProfileWarnings    []string    `json:"profile_warnings,omitempty"`
	Failures           []string    `json:"failures,omitempty"`
}

type Options struct {
	Apply          bool
	DeleteProfiles bool
}

type Result struct {
	Receipt     Receipt
	ReceiptPath string
	Applied     bool
}

type Runner struct {
	Paths       config.Paths
	BinaryPath  string
	PlistPath   string
	ReceiptPath string
	Credentials CredentialStore
	Stop        func(context.Context, string, string) error
	Now         func() time.Time
	Remove      func(string) error
	RemoveAll   func(string) error
}

func DefaultReceiptPath(paths config.Paths) string {
	return filepath.Join(filepath.Dir(paths.Root), "Postpilot Agent Retirement", "shutdown-receipt.json")
}

func (r Runner) Run(ctx context.Context, options Options) (Result, error) {
	if err := r.defaults(); err != nil {
		return Result{}, err
	}
	receipt, err := r.inventory(options.DeleteProfiles)
	result := Result{Receipt: receipt, ReceiptPath: r.ReceiptPath}
	if err != nil {
		return result, err
	}
	if !options.Apply {
		return result, nil
	}
	result.Applied = true
	receipt.Status = "in_progress"
	receipt.StartedAt = r.Now().UTC()
	if err := writeReceipt(r.ReceiptPath, receipt); err != nil {
		return result, fmt.Errorf("write retirement recovery receipt before cleanup: %w", err)
	}

	lockPath := filepath.Join(r.Paths.Root, "run.lock")
	if r.Stop == nil {
		receipt.Failures = append(receipt.Failures, "no process shutdown adapter is available")
		return r.finish(receipt)
	}
	if err := r.Stop(ctx, r.BinaryPath, lockPath); err != nil {
		receipt.Failures = append(receipt.Failures, "companion shutdown: "+err.Error())
		return r.finish(receipt)
	}
	receipt.ShutdownVerified = true

	if len(receipt.CredentialAccounts) > 0 && r.Credentials == nil {
		receipt.Failures = append(receipt.Failures, "credential deletion adapter is unavailable")
	} else {
		for _, account := range receipt.CredentialAccounts {
			if err := r.Credentials.Delete(ctx, account); err != nil {
				receipt.Failures = append(receipt.Failures, fmt.Sprintf("delete Keychain account %q: %v", account, err))
			}
		}
	}

	for _, owned := range receipt.OwnedPaths {
		if owned.Kind == "config" && !receipt.InventoryComplete {
			continue
		}
		if err := r.removeOwned(owned); err != nil {
			receipt.Failures = append(receipt.Failures, fmt.Sprintf("remove %s: %v", owned.Path, err))
		}
	}
	if options.DeleteProfiles {
		for _, profile := range receipt.ProfilePaths {
			if err := r.removeProfile(profile); err != nil {
				receipt.Failures = append(receipt.Failures, fmt.Sprintf("remove profile %s: %v", profile.Path, err))
			}
		}
	}
	// Empty app-owned container directories may be removed, but the application
	// support root and any unknown children are deliberately retained.
	for _, dir := range []string{filepath.Dir(r.BinaryPath), r.Paths.Jobs, r.Paths.Logs} {
		if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, fs.ErrInvalid) {
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || !errors.Is(pathErr.Err, syscall.ENOTEMPTY) {
				receipt.Failures = append(receipt.Failures, fmt.Sprintf("remove empty directory %s: %v", dir, err))
			}
		}
	}
	return r.finish(receipt)
}

func (r *Runner) defaults() error {
	if r.Paths.Root == "" || r.Paths.ConfigFile == "" || r.Paths.Profiles == "" || r.Paths.Jobs == "" || r.Paths.Logs == "" {
		return errors.New("retirement paths are incomplete")
	}
	if r.PlistPath == "" {
		return errors.New("LaunchAgent plist path is missing")
	}
	if r.BinaryPath == "" {
		r.BinaryPath = filepath.Join(r.Paths.Root, "bin", "postpilot-agent")
	}
	if r.ReceiptPath == "" {
		r.ReceiptPath = DefaultReceiptPath(r.Paths)
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Remove == nil {
		r.Remove = os.Remove
	}
	if r.RemoveAll == nil {
		r.RemoveAll = os.RemoveAll
	}
	return nil
}

func (r Runner) inventory(deleteProfiles bool) (Receipt, error) {
	previous, previousErr := readReceipt(r.ReceiptPath)
	receipt := Receipt{Version: receiptVersion, Status: "inspection", LaunchAgentLabel: "com.postpilot.publishing-agent"}
	if previousErr != nil && !errors.Is(previousErr, os.ErrNotExist) {
		return receipt, fmt.Errorf("read existing retirement receipt without overwriting it: %w", previousErr)
	}
	if previousErr == nil {
		receipt = previous
		receipt.Status = "inspection"
		receipt.Failures = nil
		receipt.ProfileWarnings = nil
	}

	parsed, present, parseErr := readRawConfig(r.Paths.ConfigFile)
	if parseErr != nil {
		receipt.InventoryComplete = false
		receipt.Failures = append(receipt.Failures, "connection inventory unresolved: "+parseErr.Error())
	} else if !present {
		if previousErr != nil || !previous.InventoryComplete {
			receipt.InventoryComplete = false
			receipt.Failures = append(receipt.Failures, "connection inventory unresolved: config is missing and no prior complete receipt exists")
		}
	} else {
		receipt.InventoryComplete = true
		accounts, paths, profiles, issues, profileWarnings := r.pathsFromConfig(parsed)
		receipt.CredentialAccounts = mergeStrings(receipt.CredentialAccounts, accounts)
		receipt.OwnedPaths = mergeOwned(receipt.OwnedPaths, paths)
		receipt.ProfilePaths = mergeOwned(receipt.ProfilePaths, profiles)
		receipt.Failures = append(receipt.Failures, issues...)
		receipt.ProfileWarnings = append(receipt.ProfileWarnings, profileWarnings...)
	}

	fixed := []OwnedPath{
		{Path: r.PlistPath, Kind: "file"},
		{Path: r.BinaryPath, Kind: "file"},
		{Path: filepath.Join(r.Paths.Root, "run.lock"), Kind: "file"},
		{Path: r.Paths.Logs, Kind: "logs"},
		{Path: r.Paths.ConfigFile, Kind: "config"},
	}
	receipt.OwnedPaths = mergeOwned(receipt.OwnedPaths, fixed)
	receipt.CredentialAccounts = sortedUnique(receipt.CredentialAccounts)
	sortOwned(receipt.OwnedPaths)
	sortOwned(receipt.ProfilePaths)

	for _, owned := range receipt.OwnedPaths {
		if err := r.validateOwned(owned); err != nil {
			return receipt, err
		}
	}
	if deleteProfiles {
		if !receipt.InventoryComplete {
			return receipt, errors.New("profile deletion refused because connection ownership is unresolved")
		}
		if len(receipt.ProfileWarnings) > 0 {
			return receipt, fmt.Errorf("profile deletion refused: %s", strings.Join(receipt.ProfileWarnings, "; "))
		}
		for _, profile := range receipt.ProfilePaths {
			if err := r.validateProfile(profile); err != nil {
				return receipt, fmt.Errorf("profile deletion refused: %w", err)
			}
		}
	}
	receipt.ObservedLocalPaths = r.observe(receipt)
	return receipt, nil
}

func readRawConfig(path string) (config.File, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config.File{}, false, nil
	}
	if err != nil {
		return config.File{}, false, err
	}
	var found config.File
	if err := json.Unmarshal(data, &found); err != nil {
		return config.File{}, true, fmt.Errorf("decode config: %w", err)
	}
	return found, true, nil
}

func (r Runner) pathsFromConfig(found config.File) ([]string, []OwnedPath, []OwnedPath, []string, []string) {
	var accounts []string
	var owned, profiles []OwnedPath
	var issues, profileWarnings []string
	for _, connection := range found.Connections {
		if account := strings.TrimSpace(connection.KeychainAccount); account != "" {
			accounts = append(accounts, account)
		}
		work := connection.WorkDir
		if work == "" && safeID(connection.ID) {
			work = filepath.Join(r.Paths.Jobs, connection.ID)
		}
		if item, err := r.ownedChild(work, "jobs", connection.ID); err != nil {
			issues = append(issues, fmt.Sprintf("connection %q work path unresolved: %v", connection.ID, err))
		} else {
			owned = append(owned, item)
		}
		if item, err := r.profileChild(connection.ProfileDir, connection.ID); err != nil {
			profileWarnings = append(profileWarnings, fmt.Sprintf("connection %q profile ownership unresolved: %v", connection.ID, err))
		} else {
			profiles = append(profiles, item)
		}
	}
	for _, draft := range found.Drafts {
		if item, err := r.ownedChild(draft.WorkDir, "jobs", draft.ID); err != nil {
			issues = append(issues, fmt.Sprintf("draft %q work path unresolved: %v", draft.ID, err))
		} else {
			owned = append(owned, item)
		}
		if item, err := r.profileChild(draft.ProfileDir, draft.ID); err != nil {
			profileWarnings = append(profileWarnings, fmt.Sprintf("draft %q profile ownership unresolved: %v", draft.ID, err))
		} else {
			profiles = append(profiles, item)
		}
	}
	return accounts, owned, profiles, issues, profileWarnings
}

func (r Runner) ownedChild(path, kind, owner string) (OwnedPath, error) {
	if !safeID(owner) || path == "" || filepath.Clean(path) != filepath.Join(r.Paths.Jobs, owner) {
		return OwnedPath{}, errors.New("path is not the exact owned jobs directory")
	}
	return OwnedPath{Path: filepath.Clean(path), Kind: kind, OwnerID: owner}, nil
}

func (r Runner) profileChild(path, owner string) (OwnedPath, error) {
	if !safeID(owner) || path == "" || filepath.Clean(path) != filepath.Join(r.Paths.Profiles, owner) {
		return OwnedPath{}, errors.New("path is not the exact owned profile directory")
	}
	return OwnedPath{Path: filepath.Clean(path), Kind: "profile", OwnerID: owner}, nil
}

func safeID(value string) bool {
	return value != "" && value != "." && value != ".." && filepath.Base(value) == value && !strings.ContainsAny(value, `/\\`)
}

func (r Runner) validateOwned(owned OwnedPath) error {
	want := map[string]string{
		r.PlistPath:                             "file",
		r.BinaryPath:                            "file",
		filepath.Join(r.Paths.Root, "run.lock"): "file",
		r.Paths.Logs:                            "logs",
		r.Paths.ConfigFile:                      "config",
	}
	if kind, ok := want[filepath.Clean(owned.Path)]; ok && owned.Kind == kind {
		return nil
	}
	if owned.Kind == "jobs" && safeID(owned.OwnerID) && filepath.Clean(owned.Path) == filepath.Join(r.Paths.Jobs, owned.OwnerID) {
		return noSymlinkComponents(r.Paths.Jobs, owned.Path)
	}
	return fmt.Errorf("receipt contains unowned deletion path %q", owned.Path)
}

func (r Runner) validateProfile(profile OwnedPath) error {
	if profile.Kind != "profile" || !safeID(profile.OwnerID) || filepath.Clean(profile.Path) != filepath.Join(r.Paths.Profiles, profile.OwnerID) {
		return fmt.Errorf("%q is not an exact owned profile path", profile.Path)
	}
	return noSymlinkComponents(r.Paths.Profiles, profile.Path)
}

func noSymlinkComponents(root, target string) error {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("path escapes its owned root")
	}
	current := root
	for _, part := range append([]string{""}, strings.Split(rel, string(filepath.Separator))...) {
		if part != "" {
			current = filepath.Join(current, part)
		}
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", current)
		}
	}
	return nil
}

func (r Runner) observe(receipt Receipt) []string {
	var observed []string
	for _, item := range receipt.OwnedPaths {
		if item.Kind == "config" && !receipt.InventoryComplete {
			continue
		}
		_ = filepath.WalkDir(item.Path, func(path string, entry fs.DirEntry, err error) error {
			if err == nil {
				observed = append(observed, path)
			}
			return nil
		})
	}
	return sortedUnique(observed)
}

func (r Runner) removeOwned(owned OwnedPath) error {
	if err := r.validateOwned(owned); err != nil {
		return err
	}
	if owned.Kind == "jobs" || owned.Kind == "logs" {
		if err := noSymlinkComponents(filepath.Dir(owned.Path), owned.Path); err != nil {
			return err
		}
		return r.RemoveAll(owned.Path)
	}
	if err := r.Remove(owned.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (r Runner) removeProfile(profile OwnedPath) error {
	if err := r.validateProfile(profile); err != nil {
		return err
	}
	return r.RemoveAll(profile.Path)
}

func (r Runner) finish(receipt Receipt) (Result, error) {
	if !receipt.InventoryComplete && !containsPrefix(receipt.Failures, "connection inventory unresolved") {
		receipt.Failures = append(receipt.Failures, "connection inventory unresolved")
	}
	if len(receipt.Failures) == 0 && receipt.ShutdownVerified {
		receipt.Status = "complete"
		now := r.Now().UTC()
		receipt.CompletedAt = &now
	} else {
		receipt.Status = "incomplete"
		receipt.CompletedAt = nil
	}
	if err := writeReceipt(r.ReceiptPath, receipt); err != nil {
		return Result{Receipt: receipt, ReceiptPath: r.ReceiptPath, Applied: true}, err
	}
	result := Result{Receipt: receipt, ReceiptPath: r.ReceiptPath, Applied: true}
	if receipt.Status != "complete" {
		return result, fmt.Errorf("retirement incomplete: %s", strings.Join(receipt.Failures, "; "))
	}
	return result, nil
}

func writeReceipt(path string, receipt Receipt) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readReceipt(path string) (Receipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return Receipt{}, err
	}
	if receipt.Version != receiptVersion || receipt.LaunchAgentLabel != "com.postpilot.publishing-agent" {
		return Receipt{}, errors.New("unsupported retirement receipt")
	}
	return receipt, nil
}

func mergeStrings(left, right []string) []string {
	return sortedUnique(append(append([]string{}, left...), right...))
}

func sortedUnique(values []string) []string {
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

func mergeOwned(left, right []OwnedPath) []OwnedPath {
	set := make(map[string]OwnedPath, len(left)+len(right))
	for _, item := range append(append([]OwnedPath{}, left...), right...) {
		set[item.Kind+"\x00"+item.OwnerID+"\x00"+filepath.Clean(item.Path)] = item
	}
	result := make([]OwnedPath, 0, len(set))
	for _, item := range set {
		result = append(result, item)
	}
	return result
}

func sortOwned(items []OwnedPath) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Path < items[j].Path
	})
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
