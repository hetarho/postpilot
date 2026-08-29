package rpc

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/generation"
)

func TestToConnectErrorMapsGenerationPreconditions(t *testing.T) {
	for _, err := range []error{
		generation.ErrWriteModelRequired,
		generation.ErrObserveModelRequired,
		generation.ErrRevisionContentRequired,
	} {
		if got := connect.CodeOf(toConnectError("start", err)); got != connect.CodeFailedPrecondition {
			t.Errorf("%v mapped to %v", err, got)
		}
	}
	active := &generation.JobAlreadyInProgressError{ActiveID: "job-active"}
	err := toConnectError("start", errors.Join(errors.New("enqueue"), active))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), active.ActiveID) {
		t.Fatalf("active error = %v", err)
	}
}

func TestToConnectErrorMapsRevisionInstructionValidation(t *testing.T) {
	for _, err := range []error{
		generation.ErrRevisionInstructionRequired,
		generation.ErrRevisionInstructionTooLong,
		generation.ErrInvalidTargetLength,
	} {
		if got := connect.CodeOf(toConnectError("start revision", err)); got != connect.CodeInvalidArgument {
			t.Errorf("%v mapped to %v", err, got)
		}
	}
}

func TestToConnectErrorMapsOwnership(t *testing.T) {
	if got := connect.CodeOf(toConnectError("start", generation.ErrNotFound)); got != connect.CodeNotFound {
		t.Fatalf("not found mapped to %v", got)
	}
	if got := connect.CodeOf(toConnectError("start", generation.ErrForbidden)); got != connect.CodePermissionDenied {
		t.Fatalf("forbidden mapped to %v", got)
	}
}
