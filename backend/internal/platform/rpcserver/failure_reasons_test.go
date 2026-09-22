package rpcserver_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// reasonShape is what a public failure reason looks like: SCREAMING_SNAKE_CASE. It is how a
// reason constant is told apart from every other string constant in the tree.
var reasonShape = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,}$`)

// TestEveryEmittedReasonIsATypedEnumValue reads the reasons back out of the source and
// checks them against the generated `FailureReason` enum — which IS the list now (T278).
//
// A reason the enum does not name would reach the browser as the generic unknown copy, and
// a domain reason string that no enum value covers would be translated to UNKNOWN_FAILURE by
// `rpcserver.ReasonOf`: ARCH-3 says a fallback branch in a mirrored enum is a test failure,
// not a value, and this is where that is enforced.
func TestEveryEmittedReasonIsATypedEnumValue(t *testing.T) {
	found, unresolved := scanFailureReasons(t)
	for _, position := range unresolved {
		// A reason this scan cannot read is a reason it cannot protect. Naming the variable
		// or constant with "reason" in it is what makes it visible here.
		t.Errorf("%s: NewAppError's reason is neither a typed constant nor named for a reason", position)
	}
	for _, reason := range found {
		if _, named := postpilotv1.FailureReason_value[reason]; !named {
			t.Errorf("%s is emitted but is not a FailureReason: add it to proto/postpilot/v1/error.proto", reason)
		}
	}
}

// TestEveryTypedReasonIsEmitted is the other direction: a value nobody can produce is dead
// weight in a contract three languages compile against, and the browser still has to carry a
// translation for it.
func TestEveryTypedReasonIsEmitted(t *testing.T) {
	found, _ := scanFailureReasons(t)
	emitted := map[string]bool{}
	for _, reason := range found {
		emitted[reason] = true
	}
	// Reasons this API never constructs itself, each for its own reason:
	allowed := map[string]bool{
		// the zero value every unmapped failure falls back to
		"UNKNOWN_FAILURE": true,
		// the browser's own, raised when no answer arrives at all
		"NETWORK_UNAVAILABLE": true,
		// notice codes the clip plan and the voice profile carry as data, rendered from the
		// same catalogue as a failure
		"CLIP_LAYOUT_FREQUENCY": true, "VOICE_PROFILE_FIELD_REQUIRED": true,
		// T283 removes every producer before T284 removes and reserves the obsolete wire
		// values. Keep this bridge exact so an unrelated dead reason still fails the test.
		"POST_PUBLISHING":         true,
		"PUBLISH_AGENT_NOT_READY": true, "PUBLISH_AGENT_REVOKED": true,
		"PUBLISH_AGENT_UNAVAILABLE": true, "PUBLISH_ALREADY_EXISTS": true,
		"PUBLISH_CATEGORY_NOT_FOUND": true, "PUBLISH_COMMIT_FENCE": true,
		"PUBLISH_FORBIDDEN": true, "PUBLISH_LEASE_INVALID": true,
		"PUBLISH_NEEDS_ATTENTION": true, "PUBLISH_NOT_FOUND": true,
		"PUBLISH_OUTCOME_UNKNOWN": true, "PUBLISH_PAIRING_INVALID": true,
		"PUBLISH_PAIRING_LIMIT": true, "PUBLISH_POST_NOT_FINALIZED": true,
		"PUBLISH_REQUEST_INVALID": true, "PUBLISH_STALE_REVISION": true,
		"PUBLISH_TRANSITION_INVALID": true, "PUBLISH_URL_INVALID": true,
		"VIDEO_NOT_PUBLISHABLE": true,
	}
	var dead []string
	for name := range postpilotv1.FailureReason_value {
		if !emitted[name] && !allowed[name] {
			dead = append(dead, name)
		}
	}
	sort.Strings(dead)
	if len(dead) > 0 {
		t.Errorf("these FailureReason values are emitted by nothing: %v", dead)
	}
}

// scanFailureReasons collects, from every non-test source file under internal/ and cmd/:
// the typed reason constant (or, for the few that predate T278, the literal) of every
// NewAppError call, and every constant or variable whose name
// mentions a reason and whose value has a reason's shape (which is how the typed failures'
// own constants, reached through the Failure interface, are picked up).
func scanFailureReasons(t *testing.T) (reasons []string, unresolved []string) {
	t.Helper()
	found := map[string]bool{}
	for _, root := range []string{"internal", "cmd"} {
		root = filepath.Join("..", "..", "..", root)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				// Generated code neither creates reasons nor may be hand-edited (ARCH-3).
				if entry.Name() == "gen" || entry.Name() == "sqlc" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fileSet := token.NewFileSet()
			file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(file, func(node ast.Node) bool {
				switch current := node.(type) {
				case *ast.CallExpr:
					if calleeName(current.Fun) != "NewAppError" || len(current.Args) < 3 {
						return true
					}
					// The typed constant every adapter now passes: postpilotv1.FailureReason_X.
					if selector, ok := current.Args[2].(*ast.SelectorExpr); ok {
						if name, typed := strings.CutPrefix(selector.Sel.Name, "FailureReason_"); typed {
							found[name] = true
							return true
						}
					}
					if literal, ok := current.Args[2].(*ast.BasicLit); ok && literal.Kind == token.STRING {
						value, _ := strconv.Unquote(literal.Value)
						found[value] = true
						return true
					}
					if !mentionsReason(current.Args[2]) {
						unresolved = append(unresolved, fileSet.Position(current.Args[2].Pos()).String())
					}
				case *ast.KeyValueExpr:
					// A durable failure written as a struct field: `Failure{Reason: "X"}`. The
					// browser renders these from the job record rather than from a status, and
					// they are values of the same enum, so they count as emitted.
					if key, ok := current.Key.(*ast.Ident); ok && key.Name == "Reason" {
						if literal, ok := current.Value.(*ast.BasicLit); ok && literal.Kind == token.STRING {
							value, _ := strconv.Unquote(literal.Value)
							if reasonShape.MatchString(value) {
								found[value] = true
							}
						}
					}
				case *ast.ValueSpec:
					for index, name := range current.Names {
						if index < len(current.Values) {
							collectReason(found, name.Name, current.Values[index])
						}
					}
				case *ast.AssignStmt:
					for index, target := range current.Lhs {
						if name, ok := target.(*ast.Ident); ok && index < len(current.Rhs) {
							collectReason(found, name.Name, current.Rhs[index])
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
	for reason := range found {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons, unresolved
}

func collectReason(into map[string]bool, name string, value ast.Expr) {
	if !strings.Contains(strings.ToLower(name), "reason") {
		return
	}
	literal, ok := value.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return
	}
	text, _ := strconv.Unquote(literal.Value)
	if reasonShape.MatchString(text) {
		into[text] = true
	}
}

func calleeName(function ast.Expr) string {
	switch current := function.(type) {
	case *ast.SelectorExpr:
		return current.Sel.Name
	case *ast.Ident:
		return current.Name
	}
	return ""
}

// mentionsReason reports whether an expression names a reason anywhere inside it — a
// constant, a local variable, or the Failure interface's own Reason() method.
func mentionsReason(expression ast.Expr) bool {
	mentions := false
	ast.Inspect(expression, func(node ast.Node) bool {
		if name, ok := node.(*ast.Ident); ok && strings.Contains(strings.ToLower(name.Name), "reason") {
			mentions = true
		}
		return !mentions
	})
	return mentions
}
