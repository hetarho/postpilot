package rpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestToConnectErrorCancellationIsNotLogged(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, tc := range []struct {
		cause error
		code  connect.Code
	}{{context.Canceled, connect.CodeCanceled}, {context.DeadlineExceeded, connect.CodeDeadlineExceeded}} {
		err := toConnectError(fmt.Errorf("source availability: %w", tc.cause))
		if connect.CodeOf(err) != tc.code || !errors.Is(err, tc.cause) {
			t.Fatalf("cancellation lost its code/cause: %v", err)
		}
	}
	if logs.Len() != 0 {
		t.Fatalf("expected cancellation was logged: %s", logs.String())
	}
	unknown := errors.New("fixture storage failure")
	if connect.CodeOf(toConnectError(fmt.Errorf("source HEAD failed: %w", unknown))) != connect.CodeInternal {
		t.Fatal("unknown failure did not remain internal")
	}
	if !strings.Contains(logs.String(), "fixture storage failure") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("unknown wrapped cause was not logged: %s", logs.String())
	}
}
