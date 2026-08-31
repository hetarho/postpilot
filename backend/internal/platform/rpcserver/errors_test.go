package rpcserver_test

import (
	"testing"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func TestNewAppErrorAttachesOneTypedDetail(t *testing.T) {
	params := map[string]string{"max": "100", "actual": "101"}
	err := rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid field", "PURPOSE_FIELD_TOO_LONG", params)
	params["max"] = "changed after construction"

	if got, want := err.Code(), connect.CodeInvalidArgument; got != want {
		t.Fatalf("code = %v, want %v", got, want)
	}
	if got, want := err.Message(), "invalid field"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if got, want := len(err.Details()), 1; got != want {
		t.Fatalf("details = %d, want %d", got, want)
	}

	value, valueErr := err.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail type = %T, want *postpilotv1.AppErrorDetail", value)
	}
	if got, want := detail.GetReason(), "PURPOSE_FIELD_TOO_LONG"; got != want {
		t.Errorf("reason = %q, want %q", got, want)
	}
	if got, want := detail.GetParams()["max"], "100"; got != want {
		t.Errorf("max = %q, want cloned value %q", got, want)
	}
	if got, want := detail.GetParams()["actual"], "101"; got != want {
		t.Errorf("actual = %q, want %q", got, want)
	}
}

func TestNewAppErrorAcceptsNoParams(t *testing.T) {
	err := rpcserver.NewAppError(connect.CodeNotFound, "not found", "POST_NOT_FOUND", nil)
	value, valueErr := err.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail := value.(*postpilotv1.AppErrorDetail)
	if detail.GetParams() != nil {
		t.Fatalf("params = %#v, want nil", detail.GetParams())
	}
}
