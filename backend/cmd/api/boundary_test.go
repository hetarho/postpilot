package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each context owns its tables (ARCHITECTURE §2.2): another context reads them only through
// published behavior adapted here, never by importing a sibling's store or sqlc package.
// This walks every non-test file under internal/ and fails on such an import.
func TestNoContextImportsASiblingStore(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	const module = "github.com/postpilot/backend/internal/"
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		context := strings.Split(filepath.ToSlash(rel), "/")[0]
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			target := strings.Trim(imported.Path.Value, `"`)
			if !strings.HasPrefix(target, module) {
				continue
			}
			parts := strings.Split(strings.TrimPrefix(target, module), "/")
			if len(parts) < 2 || parts[0] == context {
				continue
			}
			if parts[1] == "store" || parts[1] == "sqlc" {
				t.Errorf("%s (context %s) imports %s: read another context through its published behavior instead", rel, context, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
