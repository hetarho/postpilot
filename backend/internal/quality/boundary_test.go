package quality

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// What a measurement must never reach (QUAL-16): the model port, a provider, the credit ledger
// or the job queue. Each is the package itself and anything under it.
var measurementForbidden = []string{
	"github.com/postpilot/backend/internal/llm",
	"github.com/postpilot/backend/internal/provider",
	"github.com/postpilot/backend/internal/usage",
	"github.com/postpilot/backend/internal/job",
}

// TestMeasurementCallsNoProviderAndCostsNoCredit asks the Go toolchain for the real dependency
// closure of every quality package, as internal/llm's boundary test does, so an indirect leak
// (quality → helper → provider) is caught too.
func TestMeasurementCallsNoProviderAndCostsNoCredit(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}

	root := strings.TrimSpace(goOutput(t, goBin, "list", "-m", "-f", "{{.Dir}}"))
	listing := goOutput(t, goBin, "list", "-C", root, "-f", `{{.ImportPath}} {{join .Deps " "}}`,
		"github.com/postpilot/backend/internal/quality/...")

	checked := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg, deps := fields[0], fields[1:]
		checked[pkg] = true
		for _, dep := range deps {
			for _, bad := range measurementForbidden {
				if dep == bad || strings.HasPrefix(dep, bad+"/") {
					t.Errorf("%s depends on %s, which quality must never reach", pkg, dep)
				}
			}
		}
	}
	if len(checked) == 0 {
		t.Fatal("no quality package was checked")
	}
	// The store and the Connect edge are held to the same rule as the measurements: neither may
	// reach a provider or the ledger either.
	for _, pkg := range []string{
		"github.com/postpilot/backend/internal/quality",
		"github.com/postpilot/backend/internal/quality/store",
		"github.com/postpilot/backend/internal/quality/rpc",
	} {
		if !checked[pkg] {
			t.Errorf("%s was not among the checked packages", pkg)
		}
	}
}

// searchAPINames are what a package would have to name to call the 네이버 검색 API: its host,
// its API hub and the client package that once called it. Each is split so that this file,
// which the scan below reads too, does not name them itself.
var searchAPINames = []string{"openapi" + ".naver.com", "naver" + "apihub", "naver" + "search"}

// QUAL-47: no 네이버 검색 API result may reach a prompt, a stored list or a screen, so no backend
// package names the API at all, in any case. Every Go file and embedded file of every package in
// the module is read, tests included; the migrations alone are skipped, because an applied
// migration is never edited (ARCH-43).
func TestNoBackendPackageNamesNaversSearchAPI(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}

	root := strings.TrimSpace(goOutput(t, goBin, "list", "-m", "-f", "{{.Dir}}"))
	migrations := filepath.Join(root, "internal", "platform", "db", "migrations") + string(filepath.Separator)
	decoder := json.NewDecoder(strings.NewReader(goOutput(t, goBin, "list", "-C", root, "-json", "./...")))
	scanned := map[string]bool{}
	for {
		var pkg struct {
			Dir                                                          string
			GoFiles, CgoFiles, IgnoredGoFiles, TestGoFiles, XTestGoFiles []string
			EmbedFiles, TestEmbedFiles, XTestEmbedFiles                  []string
		}
		if err := decoder.Decode(&pkg); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		for _, files := range [][]string{
			pkg.GoFiles, pkg.CgoFiles, pkg.IgnoredGoFiles, pkg.TestGoFiles, pkg.XTestGoFiles,
			pkg.EmbedFiles, pkg.TestEmbedFiles, pkg.XTestEmbedFiles,
		} {
			for _, name := range files {
				path := filepath.Join(pkg.Dir, name)
				if strings.HasPrefix(path, migrations) {
					continue
				}
				source, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				relative, err := filepath.Rel(root, path)
				if err != nil {
					t.Fatal(err)
				}
				scanned[filepath.ToSlash(relative)] = true
				text := strings.ToLower(string(source))
				for _, banned := range searchAPINames {
					if strings.Contains(text, banned) {
						t.Errorf("%s names %q, and no backend package may reach the Naver search API", relative, banned)
					}
				}
			}
		}
	}
	// The composition root, where a client would be wired, and this file, a test, were both read.
	for _, file := range []string{"cmd/api/contexts.go", "internal/quality/boundary_test.go"} {
		if !scanned[file] {
			t.Errorf("%s was not among the %d scanned files", file, len(scanned))
		}
	}
}

func goOutput(t *testing.T, bin string, args ...string) string {
	t.Helper()
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s %s: %v\n%s", bin, strings.Join(args, " "), err, exit.Stderr)
		}
		t.Fatalf("%s %s: %v", bin, strings.Join(args, " "), err)
	}
	return string(out)
}
