package llm_test

import (
	"os/exec"
	"strings"
	"testing"
)

// Packages allowed to know an adapter exists: the port itself and its sub-packages, and
// the composition root, whose whole job is to inject concrete adapters into the port
// (ARCHITECTURE §2.1 — "wiring: inject adapters → cmd/api").
func exempt(pkg string) bool {
	return strings.Contains(pkg, "/internal/llm") || strings.Contains(pkg, "/cmd/")
}

// Import paths that must never appear in the dependency closure of a package above the
// port: our adapters, and any vendor SDK an adapter might one day pull in.
var forbidden = []string{
	"github.com/postpilot/backend/internal/llm/",
	"github.com/anthropics/",
	"github.com/openai/",
	"github.com/sashabaranov/go-openai",
	"google.golang.org/genai",
}

// TestNothingAboveThePortImportsAnAdapter is plan 04's acceptance criterion 10: the
// provider abstraction is a boundary, not a preference. It asks the Go toolchain for
// the real dependency closure rather than grepping imports, so an indirect leak
// (context → helper → adapter) is caught too.
func TestNothingAboveThePortImportsAnAdapter(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}

	root := strings.TrimSpace(run(t, goBin, "list", "-m", "-f", "{{.Dir}}"))
	// One invocation for every package's transitive closure: "<pkg> dep dep …" per line.
	listing := run(t, goBin, "list", "-C", root, "-f", `{{.ImportPath}} {{join .Deps " "}}`, "./...")

	checked := 0
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg, deps := fields[0], fields[1:]
		if exempt(pkg) {
			continue
		}
		checked++
		for _, dep := range deps {
			for _, bad := range forbidden {
				if strings.HasPrefix(dep, bad) {
					t.Errorf("%s depends on %s — nothing above internal/llm may import an adapter or a provider SDK", pkg, dep)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no packages were checked — the exemption is too broad")
	}
}

func run(t *testing.T, bin string, args ...string) string {
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
