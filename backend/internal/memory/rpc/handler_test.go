package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/memory"
)

// Every refusal the user can provoke has a stable code and reason, and the not-found case
// stays indistinguishable from an unknown id.
func TestConnectCodesAndStableReasons(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		code   connect.Code
		reason string
	}{
		"unknown or foreign": {memory.ErrNotFound, connect.CodeNotFound, "MEMORY_NOT_FOUND"},
		"blank text":         {memory.ErrInvalidText, connect.CodeInvalidArgument, "MEMORY_TEXT_REQUIRED"},
		"absent kind":        {memory.ErrInvalidKind, connect.CodeInvalidArgument, "MEMORY_KIND_INVALID"},
		"blank tag":          {memory.ErrInvalidTag, connect.CodeInvalidArgument, "MEMORY_TAG_REQUIRED"},
		"duplicate text":     {memory.ErrDuplicateText, connect.CodeAlreadyExists, "MEMORY_TEXT_TAKEN"},
		"text too long":      {&memory.TextTooLongError{Chars: 121, Max: 120}, connect.CodeInvalidArgument, "MEMORY_TEXT_TOO_LONG"},
		"too many tags":      {&memory.TooManyTagsError{Count: 6, Max: 5}, connect.CodeInvalidArgument, "MEMORY_TAGS_TOO_MANY"},
		"account cap":        {&memory.AccountCapError{Max: 300}, connect.CodeFailedPrecondition, "MEMORY_LIMIT_REACHED"},
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
			case "MEMORY_TEXT_TOO_LONG":
				if detail.GetParams()["max"] != "120" || detail.GetParams()["actual"] != "121" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			case "MEMORY_TAGS_TOO_MANY":
				if detail.GetParams()["max"] != "5" || detail.GetParams()["actual"] != "6" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			case "MEMORY_LIMIT_REACHED":
				// The surface has to say the cap, so it travels as a param (MEM-11).
				if detail.GetParams()["max"] != "300" {
					t.Fatalf("params = %#v", detail.GetParams())
				}
			}
		})
	}

	leaky := toConnectError("list memories", errors.New("no such column: secret_internal_detail"))
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

// The account comes from the session on every procedure, and the contract gives a caller
// nowhere to claim one.
func TestEveryProcedureRequiresASessionAndNoRequestCarriesAUserID(t *testing.T) {
	handler := NewHandler(memory.NewService(nil, memory.Limits{TextMaxChars: 1, TagsMax: 1, MaxPerAccount: 1, InjectMax: 1}))
	anonymous := context.Background()

	if _, err := handler.ListMemories(anonymous, connect.NewRequest(&postpilotv1.ListMemoriesRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("list = %v", err)
	}
	if _, err := handler.CreateMemory(anonymous, connect.NewRequest(&postpilotv1.CreateMemoryRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("create = %v", err)
	}
	if _, err := handler.UpdateMemory(anonymous, connect.NewRequest(&postpilotv1.UpdateMemoryRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("update = %v", err)
	}
	if _, err := handler.DeleteMemory(anonymous, connect.NewRequest(&postpilotv1.DeleteMemoryRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("delete = %v", err)
	}

	for _, message := range []proto.Message{
		&postpilotv1.ListMemoriesRequest{}, &postpilotv1.CreateMemoryRequest{},
		&postpilotv1.UpdateMemoryRequest{}, &postpilotv1.DeleteMemoryRequest{},
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

// The kind crosses the wire as a named enum value and comes back as the same one. An
// UNSPECIFIED kind is refused on both procedures that accept one, because a client that
// forgot the field would otherwise have its retrieval half chosen for it (MEM-12).
func TestTheKindRoundTripsAndUnspecifiedIsRefused(t *testing.T) {
	for _, kind := range memory.Kinds {
		wire := toProtoKind(kind)
		if wire == postpilotv1.MemoryKind_MEMORY_KIND_UNSPECIFIED {
			t.Fatalf("%s has no wire value", kind)
		}
		back, err := fromProtoKind(wire)
		if err != nil || back != kind {
			t.Fatalf("%s round-tripped to %q (%v)", kind, back, err)
		}
	}
	if _, err := fromProtoKind(postpilotv1.MemoryKind_MEMORY_KIND_UNSPECIFIED); !errors.Is(err, memory.ErrInvalidKind) {
		t.Fatalf("unspecified = %v, want ErrInvalidKind", err)
	}
	// One wire value per declared kind, plus UNSPECIFIED: a sixth would be a kind nothing
	// in the domain can store.
	if got := postpilotv1.MemoryKind_name; len(got) != len(memory.Kinds)+1 {
		t.Fatalf("wire kinds = %d, want the closed five plus UNSPECIFIED", len(got))
	}
}
