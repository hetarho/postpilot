package main

import (
	"go/ast"
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

// GUIDE-33: a guideline row is written only by the owner's own GuidelineService procedures —
// never by generation, a job, a batch or the preset. Three structural checks over every non-test
// file of cmd and internal (the generated code aside): nothing outside internal/guideline and
// cmd/api imports the context; only its store calls the row-writing queries; and cmd/api, which
// wires the service, calls none of its writing methods.
func TestOnlyTheOwnerHandlerWritesGuidelines(t *testing.T) {
	const context = "github.com/postpilot/backend/internal/guideline"
	backend := filepath.Join("..", "..")
	writingQueries := map[string]bool{"InsertGuideline": true, "UpdateGuidelineText": true, "UpdateGuidelineScope": true}
	writingMethods := map[string]bool{"Create": true, "Update": true, "UpdatePreset": true, "Delete": true}

	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(backend, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, backend+string(filepath.Separator)))
			if entry.IsDir() {
				if rel == "internal/gen" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			inContext := strings.HasPrefix(rel, "internal/guideline/")
			inWiring := strings.HasPrefix(rel, "cmd/api/")
			local := ""
			for _, imported := range file.Imports {
				target := strings.Trim(imported.Path.Value, `"`)
				if target != context && !strings.HasPrefix(target, context+"/") {
					continue
				}
				if !inContext && !inWiring {
					t.Errorf("%s imports %s: only the context and its wiring may", rel, target)
				}
				if target == context {
					local = "guideline"
					if imported.Name != nil {
						local = imported.Name.Name
					}
				}
			}
			if rel != "internal/guideline/store/store.go" {
				ast.Inspect(file, func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						if selector, ok := call.Fun.(*ast.SelectorExpr); ok && writingQueries[selector.Sel.Name] {
							t.Errorf("%s calls %s: only the guideline store writes guideline rows", rel, selector.Sel.Name)
						}
					}
					return true
				})
			}
			if inWiring && local != "" {
				checkNoGuidelineWrites(t, rel, file, local, writingMethods)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// checkNoGuidelineWrites fails a cmd/api call of one of the service's writing methods on the
// guideline service: through a selector ending in .guideline, through a method named guidelines,
// or through a *guideline.Service field of the method's own receiver type.
func checkNoGuidelineWrites(t *testing.T, rel string, file *ast.File, local string, writing map[string]bool) {
	t.Helper()
	serviceFields := map[string]map[string]bool{}
	for _, decl := range file.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, spec := range general.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structType.Fields.List {
				star, ok := field.Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				selector, ok := star.X.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Service" {
					continue
				}
				if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != local {
					continue
				}
				if serviceFields[typeSpec.Name.Name] == nil {
					serviceFields[typeSpec.Name.Name] = map[string]bool{}
				}
				for _, name := range field.Names {
					serviceFields[typeSpec.Name.Name][name.Name] = true
				}
			}
		}
	}
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		receiverName, fields := "", map[string]bool(nil)
		if function.Recv != nil && len(function.Recv.List) == 1 && len(function.Recv.List[0].Names) == 1 {
			receiverName = function.Recv.List[0].Names[0].Name
			typeExpr := function.Recv.List[0].Type
			if star, ok := typeExpr.(*ast.StarExpr); ok {
				typeExpr = star.X
			}
			if ident, ok := typeExpr.(*ast.Ident); ok {
				fields = serviceFields[ident.Name]
			}
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !writing[method.Sel.Name] {
				return true
			}
			switch receiver := method.X.(type) {
			case *ast.SelectorExpr:
				if receiver.Sel.Name == "guideline" {
					t.Errorf("%s calls .guideline.%s: only the owner's procedures write guidelines", rel, method.Sel.Name)
				}
				if base, ok := receiver.X.(*ast.Ident); ok && base.Name == receiverName && fields[receiver.Sel.Name] {
					t.Errorf("%s calls %s.%s.%s on the guideline service in %s", rel, receiverName, receiver.Sel.Name, method.Sel.Name, function.Name.Name)
				}
			case *ast.CallExpr:
				if inner, ok := receiver.Fun.(*ast.SelectorExpr); ok && inner.Sel.Name == "guidelines" {
					t.Errorf("%s calls guidelines().%s: only the owner's procedures write guidelines", rel, method.Sel.Name)
				}
			}
			return true
		})
	}
}
