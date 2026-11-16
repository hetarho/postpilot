package rpcserver_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func TestNewAppErrorAttachesOneTypedDetail(t *testing.T) {
	params := map[string]string{"max": "100", "actual": "101"}
	err := rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid field", postpilotv1.FailureReason_TEMPLATE_FIELD_TOO_LONG, params)
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
	if got, want := detail.GetReason(), "TEMPLATE_FIELD_TOO_LONG"; got != want {
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
	err := rpcserver.NewAppError(connect.CodeNotFound, "not found", postpilotv1.FailureReason_POST_NOT_FOUND, nil)
	value, valueErr := err.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail := value.(*postpilotv1.AppErrorDetail)
	if detail.GetParams() != nil {
		t.Fatalf("params = %#v, want nil", detail.GetParams())
	}
}

func TestInternalFailureLogsAndHidesTheCause(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	err := rpcserver.InternalFailure("save thing", errors.New("disk on fire"))

	logged := buf.String()
	for _, want := range []string{"level=ERROR", `msg="save thing failed"`, `err="disk on fire"`} {
		if !strings.Contains(logged, want) {
			t.Errorf("log %q misses %q", logged, want)
		}
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error = %T, want *connect.Error", err)
	}
	if connectErr.Code() != connect.CodeInternal || connectErr.Message() != "save thing failed" {
		t.Fatalf("code/message = %v / %q", connectErr.Code(), connectErr.Message())
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	if got := value.(*postpilotv1.AppErrorDetail).GetReason(); got != "UNKNOWN_FAILURE" {
		t.Fatalf("reason = %q", got)
	}
}
