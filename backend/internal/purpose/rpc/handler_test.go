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

func TestConnectCodesAndKoreanMessages(t *testing.T) {
	for name, tc := range map[string]struct {
		err     error
		code    connect.Code
		message string
	}{
		"duplicate":             {purpose.ErrDuplicateName, connect.CodeAlreadyExists, "같은 이름의 용도가 이미 있어요"},
		"unknown or foreign":    {purpose.ErrNotFound, connect.CodeNotFound, "용도를 찾을 수 없어요"},
		"blank name":            {purpose.ErrNameRequired, connect.CodeInvalidArgument, "용도 이름을 입력해 주세요"},
		"blank instructions":    {purpose.ErrInstructionsRequired, connect.CodeInvalidArgument, "작성 지침을 입력해 주세요"},
		"instructions too long": {&purpose.FieldTooLongError{Field: "instructions", Chars: 2001, Max: 2000}, connect.CodeInvalidArgument, "작성 지침은(는) 2000자까지 쓸 수 있어요 (지금 2001자)"},
	} {
		t.Run(name, func(t *testing.T) {
			mapped := toConnectError("op", tc.err)
			if connect.CodeOf(mapped) != tc.code {
				t.Fatalf("code = %v, want %v", connect.CodeOf(mapped), tc.code)
			}
			if !strings.Contains(mapped.Error(), tc.message) {
				t.Fatalf("message = %q, want it to contain %q", mapped.Error(), tc.message)
			}
		})
	}

	// An unexpected failure never reaches the client as its internal text.
	leaky := toConnectError("list purposes", errors.New("no such column: secret_internal_detail"))
	if connect.CodeOf(leaky) != connect.CodeInternal || strings.Contains(leaky.Error(), "secret_internal_detail") {
		t.Fatalf("internal error leaked: %v", leaky)
	}
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
