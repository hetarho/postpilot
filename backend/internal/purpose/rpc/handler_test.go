package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/purpose"
)

func TestConnectCodesAndStableReasons(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		code   connect.Code
		reason string
	}{
		"duplicate":             {purpose.ErrDuplicateName, connect.CodeAlreadyExists, "PURPOSE_NAME_TAKEN"},
		"unknown or foreign":    {purpose.ErrNotFound, connect.CodeNotFound, "PURPOSE_NOT_FOUND"},
		"blank name":            {purpose.ErrNameRequired, connect.CodeInvalidArgument, "PURPOSE_NAME_REQUIRED"},
		"blank instructions":    {purpose.ErrInstructionsRequired, connect.CodeInvalidArgument, "PURPOSE_INSTRUCTIONS_REQUIRED"},
		"instructions too long": {&purpose.FieldTooLongError{Field: "instructions", Chars: 2001, Max: 2000}, connect.CodeInvalidArgument, "PURPOSE_FIELD_TOO_LONG"},
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
			if tc.reason == "PURPOSE_FIELD_TOO_LONG" && (detail.GetParams()["field"] != "instructions" || detail.GetParams()["max"] != "2000" || detail.GetParams()["actual"] != "2001") {
				t.Fatalf("params = %#v", detail.GetParams())
			}
		})
	}

	// An unexpected failure never reaches the client as its internal text.
	leaky := toConnectError("list purposes", errors.New("no such column: secret_internal_detail"))
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

func TestEveryProcedureRequiresASessionAndNoRequestCarriesAUserID(t *testing.T) {
	handler := NewHandler(purpose.NewService(nil, purpose.Limits{NameMaxChars: 1, DescriptionMaxChars: 1, InstructionsMaxChars: 1}))
	anonymous := context.Background()

	if _, err := handler.ListPurposes(anonymous, connect.NewRequest(&postpilotv1.ListPurposesRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("list = %v", err)
	}
	if _, err := handler.CreatePurpose(anonymous, connect.NewRequest(&postpilotv1.CreatePurposeRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("create = %v", err)
	}
	if _, err := handler.UpdatePurpose(anonymous, connect.NewRequest(&postpilotv1.UpdatePurposeRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("update = %v", err)
	}
	if _, err := handler.DeletePurpose(anonymous, connect.NewRequest(&postpilotv1.DeletePurposeRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("delete = %v", err)
	}

	// The account comes from the session, so there is nowhere in the contract for a caller
	// to claim one. A field named like an account id here would be exactly that hole.
	for _, message := range []proto.Message{
		&postpilotv1.ListPurposesRequest{}, &postpilotv1.CreatePurposeRequest{},
		&postpilotv1.UpdatePurposeRequest{}, &postpilotv1.DeletePurposeRequest{},
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

// A purpose is authored text: the projection is exactly the three fields plus the count,
// and PostCount is read-only — the wire has no way to set it.
func TestPurposeProjectionCarriesTheAuthoredTextAndACount(t *testing.T) {
	projected := toProtoPurpose(purpose.Purpose{ID: "p1", Name: "리뷰", Description: "설명", Instructions: "지침", PostCount: 4})
	if projected.GetId() != "p1" || projected.GetName() != "리뷰" || projected.GetDescription() != "설명" ||
		projected.GetInstructions() != "지침" || projected.GetPostCount() != 4 {
		t.Fatalf("projection = %+v", projected)
	}
	if toProtoPurpose(purpose.Purpose{}) != nil {
		t.Fatal("an empty purpose projected as a message instead of unset")
	}
	fields := (&postpilotv1.CreatePurposeRequest{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		if string(fields.Get(i).Name()) == "post_count" {
			t.Fatal("post_count is writable on a create")
		}
	}
}
