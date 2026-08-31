package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/guideline"
)

// A1/A2: every refusal the user can provoke has a stable code and reason, and the two
// not-found cases stay indistinguishable from unknown ids.
func TestConnectCodesAndStableReasons(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		code   connect.Code
		reason string
	}{
		"duplicate text":     {guideline.ErrDuplicateText, connect.CodeAlreadyExists, "GUIDELINE_TEXT_TAKEN"},
		"unknown or foreign": {guideline.ErrNotFound, connect.CodeNotFound, "GUIDELINE_NOT_FOUND"},
		"foreign purpose":    {guideline.ErrPurposeNotFound, connect.CodeNotFound, "GUIDELINE_PURPOSE_NOT_FOUND"},
		"blank text":         {guideline.ErrInvalidText, connect.CodeInvalidArgument, "GUIDELINE_TEXT_REQUIRED"},
		"scope shape":        {guideline.ErrScopeShape, connect.CodeInvalidArgument, "GUIDELINE_SCOPE_INVALID"},
		"text too long":      {&guideline.TextTooLongError{Chars: 301, Max: 300}, connect.CodeInvalidArgument, "GUIDELINE_TEXT_TOO_LONG"},
		"account cap":        {&guideline.AccountCapError{Max: 100}, connect.CodeFailedPrecondition, "GUIDELINE_LIMIT_REACHED"},
	} {
		t.Run(name, func(t *testing.T) {
			mapped := toConnectError("op", tc.err)
			if connect.CodeOf(mapped) != tc.code {
				t.Fatalf("code = %v, want %v", connect.CodeOf(mapped), tc.code)
			}
			detail := appErrorDetail(t, mapped)
			if got := detail.GetReason(); got != tc.reason {
				t.Fatalf("reason = %q, want %q", got, tc.reason)
			}
			switch tc.reason {
			case "GUIDELINE_TEXT_TOO_LONG":
				if detail.GetParams()["max"] != "300" || detail.GetParams()["actual"] != "301" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			case "GUIDELINE_LIMIT_REACHED":
				// The message the user reads has to name the cap, so it travels as a param.
				if detail.GetParams()["max"] != "100" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			}
		})
	}

	leaky := toConnectError("list guidelines", errors.New("no such column: secret_internal_detail"))
	if connect.CodeOf(leaky) != connect.CodeInternal || strings.Contains(leaky.Error(), "secret_internal_detail") {
		t.Fatalf("internal error leaked: %v", leaky)
	}
	if detail := appErrorDetail(t, leaky); detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Fatalf("internal detail = %#v", detail)
	}
}

func appErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T, want *connect.Error", err)
	}
	if len(connectErr.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(connectErr.Details()))
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail type = %T", value)
	}
	return detail
}

// A1: the account comes from the session on every procedure, and the contract gives a caller
// nowhere to claim one.
func TestEveryProcedureRequiresASessionAndNoRequestCarriesAUserID(t *testing.T) {
	handler := NewHandler(guideline.NewService(nil, guideline.Limits{TextMaxChars: 1, MaxPerAccount: 1}))
	anonymous := context.Background()

	if _, err := handler.ListGuidelines(anonymous, connect.NewRequest(&postpilotv1.ListGuidelinesRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("list = %v", err)
	}
	if _, err := handler.CreateGuideline(anonymous, connect.NewRequest(&postpilotv1.CreateGuidelineRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("create = %v", err)
	}
	if _, err := handler.UpdateGuideline(anonymous, connect.NewRequest(&postpilotv1.UpdateGuidelineRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("update = %v", err)
	}
	if _, err := handler.DeleteGuideline(anonymous, connect.NewRequest(&postpilotv1.DeleteGuidelineRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("delete = %v", err)
	}

	for _, message := range []proto.Message{
		&postpilotv1.ListGuidelinesRequest{}, &postpilotv1.CreateGuidelineRequest{},
		&postpilotv1.UpdateGuidelineRequest{}, &postpilotv1.DeleteGuidelineRequest{},
	} {
		fields := message.ProtoReflect().Descriptor().Fields()
		for i := 0; i < fields.Len(); i++ {
			switch name := string(fields.Get(i).Name()); name {
			case "user_id", "account_id", "owner_id":
				t.Fatalf("%s carries %s", message.ProtoReflect().Descriptor().FullName(), name)
			}
		}
	}
}

// A2: an unset scope is a refusal, never a silent "global" that would apply the rule to every
// post of the account.
func TestUnspecifiedScopeIsRefused(t *testing.T) {
	if _, err := fromProtoScope(postpilotv1.GuidelineScope_GUIDELINE_SCOPE_UNSPECIFIED); !errors.Is(err, guideline.ErrScopeShape) {
		t.Fatalf("unspecified scope err = %v", err)
	}
	for wire, want := range map[postpilotv1.GuidelineScope]guideline.Scope{
		postpilotv1.GuidelineScope_GUIDELINE_SCOPE_GLOBAL:   guideline.ScopeGlobal,
		postpilotv1.GuidelineScope_GUIDELINE_SCOPE_PURPOSES: guideline.ScopePurposes,
	} {
		got, err := fromProtoScope(wire)
		if err != nil || got != want {
			t.Fatalf("%v mapped to %q (%v)", wire, got, err)
		}
	}
}

// A2: the projection carries the authored text and the NAME projection, never the raw ids.
func TestGuidelineProjectionCarriesTextScopeAndProjectedNames(t *testing.T) {
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	projected := toProtoGuideline(guideline.Guideline{
		ID: "g1", Text: "CCTV 언급 금지", Scope: guideline.ScopePurposes,
		PurposeIDs: []string{"p1"}, Purposes: []guideline.PurposeRef{{ID: "p1", Name: "리뷰"}},
		CreatedAt: at, UpdatedAt: at,
	})
	if projected.GetId() != "g1" || projected.GetText() != "CCTV 언급 금지" {
		t.Fatalf("projection = %+v", projected)
	}
	if projected.GetScope() != postpilotv1.GuidelineScope_GUIDELINE_SCOPE_PURPOSES {
		t.Fatalf("scope = %v", projected.GetScope())
	}
	if len(projected.GetPurposes()) != 1 || projected.GetPurposes()[0].GetName() != "리뷰" {
		t.Fatalf("purposes = %+v", projected.GetPurposes())
	}
	// An orphaned scoped guideline is a real state: purposes empty, scope still PURPOSES.
	orphan := toProtoGuideline(guideline.Guideline{ID: "g2", Scope: guideline.ScopePurposes, CreatedAt: at, UpdatedAt: at})
	if orphan.GetScope() != postpilotv1.GuidelineScope_GUIDELINE_SCOPE_PURPOSES || len(orphan.GetPurposes()) != 0 {
		t.Fatalf("orphan projection = %+v", orphan)
	}
	if toProtoGuideline(guideline.Guideline{}) != nil {
		t.Fatal("an empty guideline projected as a message instead of unset")
	}
}
