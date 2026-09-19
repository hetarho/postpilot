package config

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePrefix = "github.com/postpilot/backend/internal/"

// ARCH-21: platform/config parses env into a typed struct and holds no product
// rule, so it may not import a domain context. A domain import here is how a
// clip limit came to live in platform and how `clip/store → platform/config →
// clip` became a near-cycle (review/arch-260919 F14). The check is direct
// imports of this package's own files, which is where such an edge appears.
func TestConfigImportsNoDomainPackage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			rest, ok := strings.CutPrefix(path, modulePrefix)
			if !ok {
				continue
			}
			if !strings.HasPrefix(rest, "platform/") {
				t.Errorf("%s imports %s: platform/config holds env and cross-context values only — a product limit belongs in the owning context (ARCH-21)", name, path)
			}
		}
	}
}
