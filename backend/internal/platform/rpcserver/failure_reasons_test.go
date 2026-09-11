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
)

// publicFailureReasons is every reason string this API can put on the wire.
//
// IT IS HALF OF A CROSS-LANGUAGE CONTRACT. A reason the browser does not know about is
// rendered as the generic unknown failure, so a new entry here must also be added to:
//   - frontend/src/shared/api/app-failure.ts   (appFailureSpecs, the browser's allowlist)
//   - frontend/src/app/providers/i18n/resources/ko/errors.ts
//   - frontend/src/app/providers/i18n/resources/en/errors.ts
//
// The frontend's own resources.test.ts compares those three against each other, which only
// proves the browser agrees with itself: three reasons shipped that were emitted here and
// registered nowhere. This list is the missing half, asserted where the reasons are created.
// The proto carries no enum for them (ARCH-3 keeps generated code to the proto contract), so
// a committed list plus this comment is the honest gate at that seam.
var publicFailureReasons = []string{
	"CLIP_QUOTE_REQUIRED",
	"CLIP_QUOTE_EXPIRED",
	"CLIP_QUOTE_CHANGED",
	"CLIP_MODEL_PRICING_UNAVAILABLE",
	"CLIP_CREDIT_CEILING_EXCEEDED",
	"CLIP_BUSY",
	"CLIP_PLAN_CONFLICT",
	"CLIP_INVALID_MEDIA",
	"CLIP_ANALYSIS_TOO_LARGE",
	"CLIP_WORKSPACE_LIMIT",
	"CLIP_MODEL_INPUT_UNSUPPORTED",
	"CLIP_INVALID_INPUT",
	"CLIP_COPY_TOO_LONG",
	"CLIP_LAYOUT_SAFE_AREA",
	"CLIP_LAYOUT_SIZE",
	"CLIP_LAYOUT_OVERLAP",
	"CLIP_LAYOUT_MOTION",
	"CLIP_LAYOUT_ANCHOR_STEP",
	"CLIP_LAYOUT_FREQUENCY",
	"CLIP_SOURCE_UNAVAILABLE",
	"CLIP_NOT_FOUND",
	"CLIP_TEMPLATE_NAME_TAKEN",
	"AUTH_REQUIRED",
	"BILLING_SELECTION_INVALID",
	"BILLING_UNAVAILABLE",
	"CHANGE_UNSUPPORTED",
	"CHARGE_FAILED",
	"COMBO_INCOMPLETE",
	"COMBO_UNKNOWN",
	"CONTENT_LANGUAGE_REQUIRED",
	"CURRENT_PASSWORD_WRONG",
	"CUSTOMER_KEY_MISMATCH",
	"EMAIL_ALREADY_VERIFIED",
	"EMAIL_VERIFICATION_REQUIRED",
	"EXPERIMENT_ALREADY_RUNNING",
	"EXPERIMENT_CANDIDATES_DUPLICATE",
	"EXPERIMENT_CANDIDATE_NOT_FOUND",
	"EXPERIMENT_CONFIRMATION_REQUIRED",
	"EXPERIMENT_FORBIDDEN",
	"EXPERIMENT_MODELS_REQUIRED",
	"EXPERIMENT_NOT_FOUND",
	"EXPERIMENT_RETRY_MODEL_UNAVAILABLE",
	"EXPERIMENT_SNAPSHOT_UNAVAILABLE",
	"EXPERIMENT_STAGE_INVALID",
	"EXPERIMENT_STATE_INVALID",
	"EXPERIMENT_TARGET_LENGTH_INVALID",
	"EXPERIMENT_VOICE_REQUIRED",
	"EXPERIMENT_VOICE_UNAVAILABLE",
	"GENERATION_ALREADY_RUNNING",
	"GENERATION_OBSERVE_MODEL_REQUIRED",
	"GENERATION_TARGET_LENGTH_INVALID",
	"POST_TAG_COUNT_INVALID",
	"GENERATION_VOICE_MISMATCH",
	"GENERATION_WRITE_MODEL_REQUIRED",
	"GOOGLE_ACCOUNT_MISMATCH",
	"GOOGLE_EMAIL_UNVERIFIED",
	"GOOGLE_SIGNIN_DISABLED",
	"GOOGLE_SIGNIN_FAILED",
	"GUIDELINE_CANDIDATE_NOT_FOUND",
	"GUIDELINE_LIMIT_REACHED",
	"GUIDELINE_NOT_FOUND",
	"GUIDELINE_SCOPE_INVALID",
	"GUIDELINE_TEMPLATE_NOT_FOUND",
	"GUIDELINE_TEXT_REQUIRED",
	"GUIDELINE_TEXT_TAKEN",
	"GUIDELINE_TEXT_TOO_LONG",
	"INSUFFICIENT_CREDITS",
	"INVALID_CREDENTIALS",
	"INVALID_EMAIL",
	"JOB_FORBIDDEN",
	"JOB_HANDLER_MISSING",
	"JOB_INTERRUPTED",
	"JOB_NOT_FOUND",
	"JOB_PANICKED",
	"LAST_MASTER",
	"MASTER_ONLY",
	"MODEL_CANDIDATES_DUPLICATE",
	"MODEL_DISABLED",
	"MODEL_ID_REQUIRED",
	"MODEL_NOT_FOUND",
	"MODEL_NOT_REGISTERED",
	"MODEL_OUTPUT_INVALID",
	"MODEL_OUTPUT_TRUNCATED",
	"MODEL_PURPOSE_INELIGIBLE",
	"MODEL_PURPOSE_INVALID",
	"MODEL_PURPOSE_NOT_REGISTERED",
	"MODEL_RATE_LIMITED",
	"MODEL_REASONING_INVALID",
	"MODEL_RECOMMENDATION_NOT_FOUND",
	"MODEL_SET_UNAVAILABLE",
	"MODEL_STAGE_INVALID",
	"MODEL_STAGE_REQUIRED",
	"MODEL_UNAVAILABLE",
	"MODEL_UNSUITABLE",
	"MODEL_UNSUPPORTED",
	"MODEL_VIDEO_UNSUPPORTED",
	"NO_CHANGE",
	"NO_SCHEDULED_CHANGE",
	"PASSWORD_NOT_SET",
	"PASSWORD_TOO_LONG",
	"PASSWORD_TOO_SHORT",
	"PAYMENT_METHOD_REQUIRED",
	"PLAN_REQUIRED",
	"POST_BUSY",
	"POST_CONTENT_INVALID",
	"POST_CONTENT_STALE",
	"POST_FILENAME_TAKEN",
	"POST_FORBIDDEN",
	"POST_MACHINE_BASELINE_REQUIRED",
	"POST_NOT_FINALIZED",
	"POST_NOT_FOUND",
	"POST_PHOTO_LIMIT",
	"POST_PUBLISHING",
	"POST_TARGET_LANGUAGE_REQUIRED",
	"POST_TEMPLATE_ANSWER_INVALID",
	"POST_TEMPLATE_ANSWER_TOO_LONG",
	"POST_TARGET_LANGUAGE_UNSUPPORTED",
	"POST_VIDEO_LIMIT",
	"PROVIDER_DISABLED",
	"PUBLISH_AGENT_NOT_READY",
	"PUBLISH_AGENT_REVOKED",
	"PUBLISH_AGENT_UNAVAILABLE",
	"PUBLISH_ALREADY_EXISTS",
	"PUBLISH_CATEGORY_NOT_FOUND",
	"PUBLISH_COMMIT_FENCE",
	"PUBLISH_FORBIDDEN",
	"PUBLISH_LEASE_INVALID",
	"PUBLISH_NEEDS_ATTENTION",
	"PUBLISH_NOT_FOUND",
	"PUBLISH_OUTCOME_UNKNOWN",
	"PUBLISH_PAIRING_INVALID",
	"PUBLISH_PAIRING_LIMIT",
	"PUBLISH_POST_NOT_FINALIZED",
	"PUBLISH_REQUEST_INVALID",
	"PUBLISH_STALE_REVISION",
	"PUBLISH_TRANSITION_INVALID",
	"PUBLISH_URL_INVALID",
	"PURCHASE_NOT_FOUND",
	"PURCHASE_SPENT",
	"PURCHASE_TOO_SMALL",
	"PURPOSE_NOT_FOUND",
	"REFUND_FAILED",
	"REFUND_WINDOW_CLOSED",
	"RESET_LINK_INVALID",
	"REVISION_CONTENT_REQUIRED",
	"REVISION_INSTRUCTION_REQUIRED",
	"REVISION_INSTRUCTION_TOO_LONG",
	"SUBSCRIPTION_EXISTS",
	"SUBSCRIPTION_NEEDS_METHOD",
	"SUBSCRIPTION_REQUIRED",
	"TEMPLATE_BODY_REQUIRED",
	"TEMPLATE_FIELD_TOO_LONG",
	"TEMPLATE_LIMIT_REACHED",
	"TEMPLATE_NAME_REQUIRED",
	"TEMPLATE_NAME_TAKEN",
	"TEMPLATE_NOT_FOUND",
	"TEMPLATE_PARSE_FAILED",
	"TIER_NOT_SUBSCRIBABLE",
	"TOO_MANY_ATTEMPTS",
	"UNKNOWN_FAILURE",
	"UPLOAD_INVALID",
	"UPLOAD_NOT_FOUND",
	"UPLOAD_OBJECT_MISSING",
	"UPLOAD_VIDEO_INVALID",
	"UPLOAD_VIDEO_UNSUPPORTED",
	"USER_ID_REQUIRED",
	"USER_NOT_FOUND",
	"VERIFICATION_LINK_INVALID",
	"VIDEO_NOT_PUBLISHABLE",
	"VOICE_ANALYZE_MODEL_REQUIRED",
	"VOICE_BASELINE_MISMATCH",
	"VOICE_BUSY",
	"VOICE_COMPARISON_NOT_FOUND",
	"VOICE_CONFIRMATION_NOT_FOUND",
	"VOICE_CONTENT_LANGUAGE_MISMATCH",
	"VOICE_DEFAULT_DELETE_FORBIDDEN",
	"VOICE_DELETED",
	"VOICE_DESCRIPTION_TOO_LONG",
	"VOICE_FEEDBACK_INVALID",
	"VOICE_INSUFFICIENT_SOURCES",
	"VOICE_INVALID_LIFECYCLE",
	"VOICE_LEARNING_NOT_FOUND",
	"VOICE_NAME_REQUIRED",
	"VOICE_NAME_TAKEN",
	"VOICE_NAME_TOO_LONG",
	"VOICE_NOT_FOUND",
	"VOICE_REQUIRED",
	"VOICE_RULE_NOT_FOUND",
	"VOICE_SAMPLE_MUTATION_FAILED",
	"VOICE_SAMPLE_NOT_FOUND",
	"VOICE_SAMPLE_TOO_SHORT",
	"VOICE_SOURCE_LANGUAGE_REQUIRED",
	"VOICE_SOURCE_LANGUAGE_UNSUPPORTED",
	"VOICE_VALIDATION_NOT_FOUND",
}

// reasonShape is what a public failure reason looks like: SCREAMING_SNAKE_CASE. It is how a
// reason constant is told apart from every other string constant in the tree.
var reasonShape = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,}$`)

// TestPublicFailureReasonsAreRegistered reads the reasons back out of the source and compares
// them with the list above, so adding one without registering it fails here rather than
// reaching a user as the catch-all copy.
func TestPublicFailureReasonsAreRegistered(t *testing.T) {
	found, unresolved := scanFailureReasons(t)
	for _, position := range unresolved {
		// A reason this scan cannot read is a reason it cannot protect. Naming the variable
		// or constant with "reason" in it is what makes it visible here.
		t.Errorf("%s: NewAppError's reason is neither a string literal nor named for a reason", position)
	}

	want := append([]string(nil), publicFailureReasons...)
	sort.Strings(want)
	if got := strings.Join(found, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("the API's reasons and this list disagree.\nmissing from the list: %v\nlisted but not emitted: %v",
			difference(found, want), difference(want, found))
	}
	if len(want) != len(publicFailureReasons) {
		t.Errorf("the list holds duplicates")
	}
}

func difference(from, without []string) []string {
	absent := map[string]bool{}
	for _, value := range without {
		absent[value] = true
	}
	var result []string
	for _, value := range from {
		if !absent[value] {
			result = append(result, value)
		}
	}
	return result
}

// scanFailureReasons collects, from every non-test source file under internal/ and cmd/:
// the literal reason of every NewAppError call, and every constant or variable whose name
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
					if literal, ok := current.Args[2].(*ast.BasicLit); ok && literal.Kind == token.STRING {
						value, _ := strconv.Unquote(literal.Value)
						found[value] = true
						return true
					}
					if !mentionsReason(current.Args[2]) {
						unresolved = append(unresolved, fileSet.Position(current.Args[2].Pos()).String())
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
