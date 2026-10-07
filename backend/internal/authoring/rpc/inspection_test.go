package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/authoring"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
)

type rpcInspectionStore struct {
	authoring.Store
	reads int
	state authoring.Session
}

func (s *rpcInspectionStore) ReadAuthoringInspectionSnapshot(_ context.Context, user, id string, kind authoring.Kind, rev uint32, _ authoring.Mode) (authoring.RequestInspectionSnapshot, error) {
	s.reads++
	if user != s.state.UserID || id != s.state.ID || kind != s.state.Kind {
		return authoring.RequestInspectionSnapshot{}, authoring.ErrNotFound
	}
	if rev != s.state.Revision {
		return authoring.RequestInspectionSnapshot{}, authoring.ErrStale
	}
	return authoring.RequestInspectionSnapshot{Session: s.state}, nil
}
func (*rpcInspectionStore) WriteAuthoringRequestCapture(context.Context, authoring.RequestCapture) error {
	panic("read wrote capture")
}
func (*rpcInspectionStore) PurgeAuthoringRequestCaptures(context.Context, string, string) error {
	panic("read purged capture")
}

type rpcInspectionPorts struct {
	authoring.Models
	authoring.Jobs
	authoring.Targets
	authoring.Budget
	authoring.Estimator
}

func (rpcInspectionPorts) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Ref: ref, Stages: []string{"write"}, Levels: map[string]string{"write": "value"}, ContextTokens: 131072, StructuredOutput: true}, true
}
func (rpcInspectionPorts) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	panic("inspection called provider")
}
func (rpcInspectionPorts) Guide(authoring.Kind) string                                 { return "owned kind grammar" }
func (rpcInspectionPorts) CompletionCap(authoring.Kind, authoring.Mode, int, bool) int { return 8192 }
func (rpcInspectionPorts) PrepareAuthoringRequest(_ context.Context, _ string, ref llm.ModelRef, request llm.Request) (llm.RequestInspection, error) {
	out, err := llm.PreparedRequestInspection(request)
	budget, effort := int64(request.MaxTokens), llm.ReasoningLow
	out.Conditions = &llm.EffectiveRequestConditions{Model: &ref, MaxCompletionTokens: &budget, ReasoningEffort: &effort}
	return out, err
}
func (rpcInspectionPorts) ModelForInspection(context.Context, string, string) (llm.ModelRef, bool, error) {
	return llm.ModelRef{ProviderID: "public-model", ModelID: "writer"}, true, nil
}
func rpcInspectionHandler() (*Handler, *rpcInspectionStore) {
	store := &rpcInspectionStore{state: authoring.Session{ID: "session", UserID: "alice", Kind: authoring.PostGuideline, Revision: 7, Purpose: "owner purpose", WorkingSource: &authoring.Artifact{Name: "draft", Body: "private current source"}}}
	ports := rpcInspectionPorts{}
	svc := authoring.NewInspectedService(authoring.NewService(store, ports, ports, ports, ports, ports), authoring.RequestInspectionDependencies{Captures: store, Models: ports, Selections: ports})
	return NewHandler(svc), store
}
func validRPCInspection() *v1.GetAuthoringRequestInspectionRequest {
	return &v1.GetAuthoringRequestInspectionRequest{SessionId: "session", Kind: v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE, Revision: 7, Mode: v1.AuthoringMode_AUTHORING_MODE_REFINE, Stage: "setting-authoring", Status: v1.InspectionStatus_INSPECTION_STATUS_PREPARED, Prompt: "owner latest request"}
}
func TestAuthoringInspectionRPCRequiresOwnerAndExplicitViews(t *testing.T) {
	h, store := rpcInspectionHandler()
	if _, err := h.GetAuthoringRequestInspection(context.Background(), connect.NewRequest(validRPCInspection())); connect.CodeOf(err) != connect.CodeUnauthenticated || store.reads != 0 {
		t.Fatalf("authentication bypassed %v", err)
	}
	for _, status := range []v1.InspectionStatus{v1.InspectionStatus_INSPECTION_STATUS_UNSPECIFIED, v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE, v1.InspectionStatus(99)} {
		in := validRPCInspection()
		in.Status = status
		if _, err := h.GetAuthoringRequestInspection(auth.WithUser(t.Context(), "alice"), connect.NewRequest(in)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("status %v allowed %v", status, err)
		}
	}
	if store.reads != 0 {
		t.Fatal("invalid view reached private read")
	}
}
func TestAuthoringInspectionRPCScopesKindRevisionAndOpaqueOwnerFailures(t *testing.T) {
	h, _ := rpcInspectionHandler()
	for _, id := range []string{"session", "unknown"} {
		in := validRPCInspection()
		in.SessionId = id
		_, err := h.GetAuthoringRequestInspection(auth.WithUser(t.Context(), "bob"), connect.NewRequest(in))
		if connect.CodeOf(err) != connect.CodeNotFound || errorDetail(t, err).Reason != "AUTHORING_SESSION_NOT_FOUND" {
			t.Fatalf("foreign/unknown %s %v", id, err)
		}
	}
	in := validRPCInspection()
	in.Kind = v1.ConfigurationKind_CONFIGURATION_KIND_VIDEO_GUIDELINE
	if _, err := h.GetAuthoringRequestInspection(auth.WithUser(t.Context(), "alice"), connect.NewRequest(in)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("kind scope %v", err)
	}
	in = validRPCInspection()
	in.Revision++
	if _, err := h.GetAuthoringRequestInspection(auth.WithUser(t.Context(), "alice"), connect.NewRequest(in)); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("revision scope %v", err)
	}
	in = validRPCInspection()
	in.DraftId = "other-session"
	if _, err := h.GetAuthoringRequestInspection(auth.WithUser(t.Context(), "alice"), connect.NewRequest(in)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("conflicting alias accepted %v", err)
	}
}
func TestAuthoringInspectionRPCSafelyMapsCurrentPreparedAndMissingCapture(t *testing.T) {
	h, store := rpcInspectionHandler()
	for _, status := range []v1.InspectionStatus{v1.InspectionStatus_INSPECTION_STATUS_CURRENT, v1.InspectionStatus_INSPECTION_STATUS_PREPARED, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED} {
		in := validRPCInspection()
		in.Status = status
		in.SessionId = ""
		in.DraftId = "session"
		response, err := h.GetAuthoringRequestInspection(auth.WithUser(t.Context(), "alice"), connect.NewRequest(in))
		if err != nil {
			t.Fatal(err)
		}
		out := response.Msg.Inspection
		if status == v1.InspectionStatus_INSPECTION_STATUS_CAPTURED {
			if out.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE || len(out.Fragments) != 0 || out.Conditions != nil || out.IssuedAt != nil {
				t.Fatal("missing capture reconstructed")
			}
		} else {
			if out.Status != status || out.Stage != "setting-authoring" || out.Mode != "post_guideline/refine" || out.Conditions == nil || out.Conditions.MaxCompletionTokens == nil || *out.Conditions.MaxCompletionTokens != 8192 || len(out.Fragments) == 0 {
				t.Fatalf("projection wrong %+v", out)
			}
		}
		raw, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		for _, unexpected := range []string{"SavedBaseline", "Publication", "TargetVersion", "CostMicrousd", "SDKBody"} {
			if strings.Contains(string(raw), unexpected) {
				t.Fatalf("private unrelated field %s", unexpected)
			}
		}
	}
	if store.reads != 3 {
		t.Fatal("request performed hidden recovery reads")
	}
}
func TestAuthoringInspectionRPCRejectsInvalidProspectiveCount(t *testing.T) {
	h, _ := rpcInspectionHandler()
	in := validRPCInspection()
	in.CandidateCount = 17
	_, err := h.GetAuthoringRequestInspection(auth.WithUser(t.Context(), "alice"), connect.NewRequest(in))
	var failure *connect.Error
	if !errors.As(err, &failure) || failure.Code() != connect.CodeInvalidArgument || errorDetail(t, err).Reason != "AUTHORING_CANDIDATE_COUNT_INVALID" {
		t.Fatalf("bad prospective count accepted %v", err)
	}
}
