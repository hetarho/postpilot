package main

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/authoring"
	authoringstore "github.com/postpilot/backend/internal/authoring/store"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/provider"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestWritingTestInspectionProductionWireKeepsBlindIdentityAndRevealsActualWitnessOnly(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	plan := h.plan(2)
	quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h, &v1.EstimateWritingTestRequest{Plan: plan}))
	if err != nil {
		t.Fatal(err)
	}
	started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h, &v1.StartWritingTestRequest{Plan: plan, RequestKey: "inspect-two", QuoteKey: quote.Msg.QuoteKey}))
	if err != nil {
		t.Fatal(err)
	}
	current := h.settledTest(t, started.Msg.Test.Id, v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW)
	h.provider.mu.Lock()
	calls := len(h.provider.requests)
	h.provider.mu.Unlock()
	for _, candidate := range current.Candidates {
		for _, status := range []v1.InspectionStatus{v1.InspectionStatus_INSPECTION_STATUS_CURRENT, v1.InspectionStatus_INSPECTION_STATUS_PREPARED, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED} {
			response, err := h.inspection.GetWritingTestRequestInspection(t.Context(), writingRPCRequest(h, &v1.GetWritingTestRequestInspectionRequest{TestId: current.Id, CandidateId: candidate.Id, Stage: "write", Status: status}))
			if err != nil || response.Msg.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE {
				t.Fatal("blind evidence exposed", err)
			}
			raw, err := protojson.Marshal(response.Msg)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"writer-", "Registered writer", "frozen", "fixture.invalid", "fragments", "conditions", "schemaVersion", "sourceFiles", "providerPromptTokens", "CostMicrousd"} {
				if strings.Contains(string(raw), forbidden) {
					t.Fatalf("blind inspection leaks %q: %s", forbidden, raw)
				}
			}
		}
	}
	raw, err := protojson.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"writer-", "Registered writer", "RequestInspections", "Origins", "CostMicrousd"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("ordinary blind output leaks %q", forbidden)
		}
	}
	match := current.Matches[0]
	picked, err := h.client.DecideTestMatch(t.Context(), writingRPCRequest(h, &v1.DecideTestMatchRequest{TestId: current.Id, ExpectedRevision: current.Revision, RequestKey: "inspect-champion", MatchId: match.Id, WinnerCandidateId: match.LeftCandidateId}))
	if err != nil || !picked.Msg.Test.Revealed {
		t.Fatal("champion reveal failed", err)
	}
	for _, candidate := range picked.Msg.Test.Candidates {
		response, err := h.inspection.GetWritingTestRequestInspection(t.Context(), writingRPCRequest(h, &v1.GetWritingTestRequestInspectionRequest{TestId: current.Id, CandidateId: candidate.Id, Stage: "write", Status: v1.InspectionStatus_INSPECTION_STATUS_CAPTURED}))
		if err != nil {
			t.Fatal(err)
		}
		inspection := response.Msg.Inspection
		if len(response.Msg.Inspections) != 1 || inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || inspection.CallId == "" || inspection.IssuedAt == nil || inspection.Conditions.GetModel().GetModelId() != candidate.Identity.GetSource().GetModel().GetModelId() || inspection.Measures.GetProviderPromptTokens() != 11 || inspection.Measures.GetProviderCompletionTokens() != 7 {
			t.Fatalf("actual witness lost: %+v", inspection)
		}
		raw, err := protojson.Marshal(response.Msg)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"fixture.invalid", "costMicrousd", "CostMicrousd", "supplierCost", "api_key"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("private execution internals leak %q", forbidden)
			}
		}
	}
	h.provider.mu.Lock()
	after := len(h.provider.requests)
	h.provider.mu.Unlock()
	if calls != after {
		t.Fatal("inspection created provider work")
	}
	for _, id := range []string{current.Id, "unknown"} {
		request := connect.NewRequest(&v1.GetWritingTestRequestInspectionRequest{TestId: id, CandidateId: current.Candidates[0].Id, Stage: "write", Status: v1.InspectionStatus_INSPECTION_STATUS_CAPTURED})
		request.Header().Set("Cookie", auth.SessionCookieName+"=bob-authenticated-session")
		if _, err := h.inspection.GetWritingTestRequestInspection(t.Context(), request); connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatal("foreign/unknown test differs", err)
		}
	}
}

func TestAuthoringInspectionProductionTransportUsesCurrentRevisionWithoutExecutionOrPublication(t *testing.T) {
	h := newPostInspectionHarness(t)
	if _, err := h.app.provider.SaveSelection(t.Context(), "alice", provider.StageWrite, h.ref); err != nil {
		t.Fatal(err)
	}
	state, err := h.app.authoring.Create(t.Context(), "alice", authoring.PostGuideline, "", "inspection-authoring")
	if err != nil {
		t.Fatal(err)
	}
	read := func(user string, revision uint32, status v1.InspectionStatus) (*v1.RequestInspection, error) {
		request := connect.NewRequest(&v1.GetAuthoringRequestInspectionRequest{SessionId: state.ID, Kind: v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE, Revision: revision, Mode: v1.AuthoringMode_AUTHORING_MODE_RECOMMEND, CandidateCount: 1, Prompt: "확인한 사실과 감상을 구분해 주세요.", Status: status})
		request.Header().Set("Cookie", auth.SessionCookieName+"="+user+"-inspection-session")
		response, err := h.client.GetAuthoringRequestInspection(t.Context(), request)
		if err != nil {
			return nil, err
		}
		return response.Msg.Inspection, nil
	}
	h.provider.mu.Lock()
	beforeCalls, beforeQualifications := len(h.provider.requests), h.provider.freePathReads
	h.provider.mu.Unlock()
	var beforeJobs, beforeHolds int
	if err := h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM generation_jobs`).Scan(&beforeJobs); err != nil {
		t.Fatal(err)
	}
	if err := h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM usage_admissions`).Scan(&beforeHolds); err != nil {
		t.Fatal(err)
	}
	for _, status := range []v1.InspectionStatus{v1.InspectionStatus_INSPECTION_STATUS_CURRENT, v1.InspectionStatus_INSPECTION_STATUS_PREPARED} {
		inspection, err := read("alice", state.Revision, status)
		if err != nil || inspection.Status != status || inspection.Mode != "post_guideline/recommend" || inspection.Conditions.GetModel().GetModelId() != "writer" || inspection.Conditions.GetReasoningEffort() != "high" || inspection.IssuedAt != nil {
			t.Fatal("preview lost exact configured request", err)
		}
	}
	h.provider.mu.Lock()
	afterCalls, afterQualifications := len(h.provider.requests), h.provider.freePathReads
	h.provider.mu.Unlock()
	var afterJobs, afterHolds int
	_ = h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM generation_jobs`).Scan(&afterJobs)
	_ = h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM usage_admissions`).Scan(&afterHolds)
	var saved int
	if err := h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM guidelines WHERE user_id='alice'`).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	if beforeCalls != afterCalls || beforeQualifications != afterQualifications || beforeJobs != afterJobs || beforeHolds != afterHolds || saved != 0 {
		t.Fatal("authoring inspection mutated canonical or admission state")
	}
	jobID, _, err := h.app.authoring.Start(t.Context(), "alice", authoring.Start{SessionID: state.ID, ExpectedRevision: state.Revision, RequestID: "inspect-start", Mode: authoring.Recommend, Prompt: "확인한 사실과 감상을 구분해 주세요.", RequestedCandidateCount: 1, WriteModel: h.ref})
	if err != nil {
		t.Fatal(err)
	}
	h.wait(t, jobID, "done")
	state, err = h.app.authoring.Get(t.Context(), "alice", state.ID)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := read("alice", state.Revision, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || capture.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || capture.CallId != jobID || capture.IssuedAt == nil || capture.Measures.GetProviderPromptTokens() != 17 {
		t.Fatalf("authoring actual capture unavailable: %+v %v", capture, err)
	}
	raw, _ := protojson.Marshal(capture)
	for _, forbidden := range []string{"PRIVATE_ENDPOINT", "PRIVATE_CREDENTIAL", "987654321", "api_key", "costMicrousd"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatal("authoring inspection leaked execution internals", forbidden)
		}
	}
	if _, err := read("bob", state.Revision, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("foreign session lookup differs", err)
	}
	if _, err := read("alice", state.Revision-1, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatal("stale revision returned private evidence", err)
	}
	if err := authoringstore.New(h.app.platform.db.Writer, h.app.platform.db.Reader).PurgeAuthoringRequestCaptures(t.Context(), "alice", state.ID); err != nil {
		t.Fatal(err)
	}
	capture, err = read("alice", state.Revision, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || capture.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE {
		t.Fatal("purged authoring capture restored", err)
	}
}
