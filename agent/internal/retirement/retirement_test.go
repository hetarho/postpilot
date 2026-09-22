package retirement

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

	"github.com/postpilot/agent/internal/config"
)

type fakeCredentials struct {
	deleted []string
	fail    map[string]error
	check   func()
}

func (f *fakeCredentials) Delete(_ context.Context, account string) error {
	if f.check != nil {
		f.check()
	}
	f.deleted = append(f.deleted, account)
	return f.fail[account]
}

type fixture struct {
	root        string
	paths       config.Paths
	binary      string
	plist       string
	receipt     string
	unknown     string
	unknownJob  string
	profiles    []string
	browser     string
	credentials *fakeCredentials
	stops       int
	runner      Runner
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "Library", "Application Support", "Postpilot Agent")
	paths := config.Paths{
		Root:       root,
		ConfigFile: filepath.Join(root, "config.json"),
		Profiles:   filepath.Join(root, "browser-profiles"),
		Jobs:       filepath.Join(root, "jobs"),
		Logs:       filepath.Join(root, "logs"),
	}
	binary := filepath.Join(root, "bin", "postpilot-agent")
	plist := filepath.Join(base, "Library", "LaunchAgents", "com.postpilot.publishing-agent.plist")
	receipt := filepath.Join(base, "Library", "Application Support", "Postpilot Agent Retirement", "shutdown-receipt.json")
	browser := filepath.Join(base, "Applications", "Chromium")
	unknown := filepath.Join(root, "unknown-legacy", "keep.txt")
	unknownJob := filepath.Join(paths.Jobs, "unknown-device", "keep.txt")
	for path, body := range map[string]string{
		binary:                                 "binary",
		plist:                                  "plist",
		filepath.Join(paths.Logs, "agent.log"): "log",
		filepath.Join(root, "run.lock"):        "lock",
		unknown:                                "legacy",
		unknownJob:                             "unknown job",
		browser:                                "browser",
		filepath.Join(paths.Jobs, "active", "job"):          "active job",
		filepath.Join(paths.Jobs, "pending", "job"):         "pending job",
		filepath.Join(paths.Jobs, "draft", "job"):           "draft job",
		filepath.Join(paths.Profiles, "active", "Cookies"):  "active cookie",
		filepath.Join(paths.Profiles, "pending", "Cookies"): "pending cookie",
		filepath.Join(paths.Profiles, "draft", "Cookies"):   "draft cookie",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	found := config.File{
		Connections: []config.Connection{
			{ID: "active", KeychainAccount: "account-active", BrowserBinary: browser, ProfileDir: filepath.Join(paths.Profiles, "active"), WorkDir: filepath.Join(paths.Jobs, "active"), Armed: true},
			{ID: "pending", KeychainAccount: "account-pending", BrowserBinary: browser, ProfileDir: filepath.Join(paths.Profiles, "pending"), WorkDir: filepath.Join(paths.Jobs, "pending")},
			{ID: "duplicate-token-owner", KeychainAccount: "account-pending", BrowserBinary: browser, ProfileDir: filepath.Join(paths.Profiles, "duplicate-token-owner"), WorkDir: filepath.Join(paths.Jobs, "duplicate-token-owner")},
		},
		Drafts: []config.ConnectionDraft{{ID: "draft", BrowserBinary: browser, ProfileDir: filepath.Join(paths.Profiles, "draft"), WorkDir: filepath.Join(paths.Jobs, "draft")}},
	}
	data, err := json.Marshal(found)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ConfigFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	credentials := &fakeCredentials{}
	result := &fixture{root: root, paths: paths, binary: binary, plist: plist, receipt: receipt, unknown: unknown, unknownJob: unknownJob, browser: browser, credentials: credentials}
	result.profiles = []string{filepath.Join(paths.Profiles, "active"), filepath.Join(paths.Profiles, "pending"), filepath.Join(paths.Profiles, "draft")}
	result.runner = Runner{
		Paths: paths, BinaryPath: binary, PlistPath: plist, ReceiptPath: receipt, Credentials: credentials,
		Now: func() time.Time { return time.Unix(1_800_000_000, 0) },
		Stop: func(_ context.Context, gotBinary, gotLock string) error {
			result.stops++
			if gotBinary != binary || gotLock != filepath.Join(root, "run.lock") {
				t.Fatalf("stop paths = %q %q", gotBinary, gotLock)
			}
			for _, path := range []string{binary, plist} {
				if _, err := os.Stat(path); err != nil {
					if result.stops == 1 {
						t.Fatalf("cleanup began before shutdown verification: %s: %v", path, err)
					}
				}
			}
			return nil
		},
	}
	credentials.check = func() {
		if _, err := os.Stat(paths.ConfigFile); err != nil && result.stops == 1 {
			t.Fatalf("config was removed before its stored credentials: %v", err)
		}
	}
	return result
}

func TestInspectionIsReadOnlyAndShowsExactOwnedProfiles(t *testing.T) {
	f := newFixture(t)
	result, err := f.runner.Run(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || f.stops != 0 || len(f.credentials.deleted) != 0 {
		t.Fatalf("inspection mutated state: %+v stops=%d deleted=%v", result, f.stops, f.credentials.deleted)
	}
	if _, err := os.Stat(f.receipt); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspection wrote a receipt: %v", err)
	}
	var gotProfiles []string
	for _, profile := range result.Receipt.ProfilePaths {
		gotProfiles = append(gotProfiles, profile.Path)
	}
	wantProfiles := append(append([]string{}, f.profiles...), filepath.Join(f.paths.Profiles, "duplicate-token-owner"))
	sort.Strings(wantProfiles)
	if !reflect.DeepEqual(gotProfiles, wantProfiles) {
		t.Fatalf("profiles = %v, want %v", gotProfiles, wantProfiles)
	}
	if _, err := os.Stat(f.binary); err != nil {
		t.Fatalf("inspection removed binary: %v", err)
	}
}

func TestApplyStopsFirstDeletesStoredCredentialsAndPreservesProfilesAndUnknownFiles(t *testing.T) {
	f := newFixture(t)
	result, err := f.runner.Run(context.Background(), Options{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Receipt.Status != "complete" || !result.Receipt.ShutdownVerified || f.stops != 1 {
		t.Fatalf("receipt = %+v stops=%d", result.Receipt, f.stops)
	}
	if want := []string{"account-active", "account-pending"}; !reflect.DeepEqual(f.credentials.deleted, want) {
		t.Fatalf("deleted credentials = %v, want %v", f.credentials.deleted, want)
	}
	for _, path := range []string{f.binary, f.plist, f.paths.ConfigFile, f.paths.Logs, filepath.Join(f.root, "run.lock"), filepath.Join(f.paths.Jobs, "active"), filepath.Join(f.paths.Jobs, "pending"), filepath.Join(f.paths.Jobs, "draft")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owned path survived %s: %v", path, err)
		}
	}
	for _, path := range append(append([]string{}, f.profiles...), f.unknown, f.unknownJob, f.browser) {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preserved path was disturbed %s: %v", path, err)
		}
	}
	info, err := os.Stat(f.receipt)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("receipt mode = %v err=%v", info.Mode().Perm(), err)
	}
	data, err := os.ReadFile(f.receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "active cookie") || !strings.Contains(string(data), "account-active") {
		t.Fatalf("receipt contains secret material or lacks retry inventory: %s", data)
	}

	// A repeat succeeds from the retained receipt even though config and Keychain
	// items are now absent. The fake Delete is deliberately idempotent.
	f.credentials.deleted = nil
	result, err = f.runner.Run(context.Background(), Options{Apply: true})
	if err != nil || result.Receipt.Status != "complete" {
		t.Fatalf("repeated retirement = %+v err=%v", result.Receipt, err)
	}
	if want := []string{"account-active", "account-pending"}; !reflect.DeepEqual(f.credentials.deleted, want) {
		t.Fatalf("retry accounts = %v, want %v", f.credentials.deleted, want)
	}
}

func TestProfileDeletionRequiresExplicitSafeOwnedPaths(t *testing.T) {
	f := newFixture(t)
	if _, err := f.runner.Run(context.Background(), Options{Apply: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.runner.Run(context.Background(), Options{Apply: true, DeleteProfiles: true}); err != nil {
		t.Fatal(err)
	}
	for _, path := range f.profiles {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("explicit profile survived %s: %v", path, err)
		}
	}
	if _, err := os.Stat(f.browser); err != nil {
		t.Fatalf("browser binary was removed with profiles: %v", err)
	}
	if _, err := os.Stat(f.unknown); err != nil {
		t.Fatalf("unknown directory was removed with profiles: %v", err)
	}
}

func TestProfileDeletionRefusesPathEscapeAndSymlinkBeforeShutdown(t *testing.T) {
	for name, alter := range map[string]func(*fixture){
		"path escape": func(f *fixture) {
			writeRawConfig(t, f.paths.ConfigFile, config.File{Connections: []config.Connection{{ID: "active", KeychainAccount: "key", WorkDir: filepath.Join(f.paths.Jobs, "active"), ProfileDir: filepath.Join(filepath.Dir(f.paths.Profiles), "elsewhere")}}})
		},
		"symlink": func(f *fixture) {
			profile := filepath.Join(f.paths.Profiles, "active")
			if err := os.RemoveAll(profile); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), profile); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			alter(f)
			_, err := f.runner.Run(context.Background(), Options{Apply: true, DeleteProfiles: true})
			if err == nil || !strings.Contains(err.Error(), "profile deletion refused") {
				t.Fatalf("unsafe profile error = %v", err)
			}
			if f.stops != 0 {
				t.Fatal("shutdown began before unsafe profile request was refused")
			}
			if _, err := os.Stat(f.binary); err != nil {
				t.Fatalf("refused request changed files: %v", err)
			}
		})
	}
}

func TestMissingOrMalformedConfigLeavesAnIncompleteRecoveryReceipt(t *testing.T) {
	for name, change := range map[string]func(*fixture){
		"missing":   func(f *fixture) { _ = os.Remove(f.paths.ConfigFile) },
		"malformed": func(f *fixture) { _ = os.WriteFile(f.paths.ConfigFile, []byte("{"), 0o600) },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			change(f)
			result, err := f.runner.Run(context.Background(), Options{Apply: true})
			if err == nil || result.Receipt.Status != "incomplete" || result.Receipt.InventoryComplete {
				t.Fatalf("result = %+v err=%v", result.Receipt, err)
			}
			if name == "malformed" {
				if _, statErr := os.Stat(f.paths.ConfigFile); statErr != nil {
					t.Fatalf("malformed recovery source was deleted: %v", statErr)
				}
			}
			if _, statErr := os.Stat(f.receipt); statErr != nil {
				t.Fatalf("recovery receipt missing: %v", statErr)
			}
		})
	}
}

func TestPartialCredentialOrFilesystemFailureIsRetriableFromReceipt(t *testing.T) {
	for name, fail := range map[string]func(*fixture){
		"credential": func(f *fixture) { f.credentials.fail = map[string]error{"account-pending": errors.New("locked")} },
		"filesystem": func(f *fixture) {
			f.runner.Remove = func(path string) error {
				if path == f.paths.ConfigFile {
					return errors.New("busy")
				}
				return os.Remove(path)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			fail(f)
			result, err := f.runner.Run(context.Background(), Options{Apply: true})
			if err == nil || result.Receipt.Status != "incomplete" {
				t.Fatalf("partial result = %+v err=%v", result.Receipt, err)
			}
			f.credentials.fail = nil
			f.runner.Remove = os.Remove
			result, err = f.runner.Run(context.Background(), Options{Apply: true})
			if err != nil || result.Receipt.Status != "complete" {
				t.Fatalf("retry result = %+v err=%v", result.Receipt, err)
			}
		})
	}
}

func TestUnownedJobPathIsPreservedAndPreventsFalseSuccess(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(filepath.Dir(f.root), "outside-jobs")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRawConfig(t, f.paths.ConfigFile, config.File{Connections: []config.Connection{{ID: "active", KeychainAccount: "stored-account", WorkDir: outside, ProfileDir: filepath.Join(f.paths.Profiles, "active")}}})
	result, err := f.runner.Run(context.Background(), Options{Apply: true})
	if err == nil || result.Receipt.Status != "incomplete" {
		t.Fatalf("unowned job result = %+v err=%v", result.Receipt, err)
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatalf("unowned path was removed: %v", statErr)
	}
	if !reflect.DeepEqual(f.credentials.deleted, []string{"stored-account"}) {
		t.Fatalf("stored account was not deleted directly: %v", f.credentials.deleted)
	}
}

func writeRawConfig(t *testing.T, path string, found config.File) {
	t.Helper()
	data, err := json.Marshal(found)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
