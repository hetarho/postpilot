package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"google.golang.org/protobuf/encoding/protojson"
)

type requestInspectionReader struct {
	calls                        int
	user, test, candidate, stage string
	status                       llm.InspectionStatus
	values                       []llm.RequestInspection
	err                          error
}

func (r *requestInspectionReader) ReadTestRequestInspections(_ context.Context, user, test, candidate, stage string, status llm.InspectionStatus) ([]llm.RequestInspection, error) {
	r.calls++
	r.user, r.test, r.candidate, r.stage, r.status = user, test, candidate, stage, status
	return r.values, r.err
}

func TestTestRequestInspectionRPCAuthenticatesAndUsesOnePluralSnapshot(t *testing.T) {
	reader := &requestInspectionReader{values: []llm.RequestInspection{llm.UnavailableRequestInspection("observe", "writing-test"), llm.UnavailableRequestInspection("observe", "writing-test")}}
	handler := NewRequestInspectionHandler(reader)
	req := connect.NewRequest(&v1.GetWritingTestRequestInspectionRequest{TestId: "test", CandidateId: "candidate", Stage: "observe", Status: v1.InspectionStatus_INSPECTION_STATUS_CAPTURED})
	if _, err := handler.GetWritingTestRequestInspection(context.Background(), req); connect.CodeOf(err) != connect.CodeUnauthenticated || reader.calls != 0 {
		t.Fatalf("unauthorized private read: %v %d", err, reader.calls)
	}
	response, err := handler.GetWritingTestRequestInspection(auth.WithUser(context.Background(), "owner"), req)
	if err != nil || reader.calls != 1 || reader.user != "owner" || reader.test != "test" || reader.candidate != "candidate" || reader.stage != "observe" || reader.status != llm.InspectionCaptured || len(response.Msg.Inspections) != 2 || response.Msg.Inspection != response.Msg.Inspections[1] {
		t.Fatalf("plural snapshot mismatch: %v %#v", err, response)
	}
}

func TestTestRequestInspectionRPCKeepsErrorsAndUnavailableWireIdentitySafe(t *testing.T) {
	reader := &requestInspectionReader{}
	handler := NewRequestInspectionHandler(reader)
	req := connect.NewRequest(&v1.GetWritingTestRequestInspectionRequest{TestId: "test", CandidateId: "candidate", Stage: "write", Status: v1.InspectionStatus_INSPECTION_STATUS_CAPTURED})
	for _, entry := range []struct {
		err  error
		code connect.Code
	}{{experiment.ErrTestNotFound, connect.CodeNotFound}, {llm.ErrInvalidInspection, connect.CodeInvalidArgument}, {errors.New("private-model supplier credential signed-link"), connect.CodeInternal}} {
		reader.err = entry.err
		_, err := handler.GetWritingTestRequestInspection(auth.WithUser(context.Background(), "owner"), req)
		if connect.CodeOf(err) != entry.code || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "supplier") {
			t.Fatalf("identity-bearing error exposed: %v", err)
		}
	}
	reader.err = nil
	value := llm.UnavailableRequestInspection("write", "writing-test")
	value.UnavailableReason = "blind_test_identity_hidden_until_reveal"
	reader.values = []llm.RequestInspection{value}
	response, err := handler.GetWritingTestRequestInspection(auth.WithUser(context.Background(), "owner"), req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := protojson.Marshal(response.Msg)
	for _, hidden := range []string{"model", "fragments", "conditions", "sourceFiles", "promptVersion", "schemaVersion", "provider", "cost"} {
		if strings.Contains(string(raw), hidden) {
			t.Fatalf("hidden wire field disclosed %s: %s", hidden, raw)
		}
	}
}
