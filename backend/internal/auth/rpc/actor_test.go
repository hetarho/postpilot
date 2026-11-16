package rpc

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestActingUserMissing(t *testing.T) {
	_, err := ActingUser(context.Background())
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("want connect error, got %v", err)
	}
	if ce.Code() != connect.CodeUnauthenticated || ce.Message() != "authentication required" {
		t.Fatalf("code/message = %v / %q", ce.Code(), ce.Message())
	}
	if len(ce.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(ce.Details()))
	}
	value, valueErr := ce.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok || detail.GetReason() != "AUTH_REQUIRED" {
		t.Fatalf("detail = %#v", value)
	}
}

func TestActingUserPresent(t *testing.T) {
	ctx := auth.WithUser(context.Background(), "u-1")
	got, err := ActingUser(ctx)
	if err != nil || got != "u-1" {
		t.Fatalf("got %q, %v", got, err)
	}
}
