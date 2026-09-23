package quality

import (
	"os/exec"
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
					t.Errorf("%s depends on %s — a measurement calls no provider and costs no credit", pkg, dep)
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
