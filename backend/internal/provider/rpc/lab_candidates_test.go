package rpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
)

func TestLabCandidateWireSlotsAndRefusal(t *testing.T) {
	slots := map[provider.SelectionSlot]postpilotv1.SelectionSlot{
		"":                      postpilotv1.SelectionSlot_SELECTION_SLOT_UNSPECIFIED,
		provider.SlotActive:     postpilotv1.SelectionSlot_SELECTION_SLOT_ACTIVE,
		provider.SlotCandidateA: postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_A,
		provider.SlotCandidateB: postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_B,
		provider.SlotCandidateC: postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_C,
		provider.SlotCandidateD: postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_D,
		provider.SlotCandidateE: postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_E,
	}
	if len(slots) != len(postpilotv1.SelectionSlot_name) {
		t.Fatalf("domain slot mirror is incomplete: %d vs %d", len(slots), len(postpilotv1.SelectionSlot_name))
	}
	for slot, expected := range slots {
		if got := slotToProto(slot); got != expected {
			t.Fatalf("%q maps to %v, want %v", slot, got, expected)
		}
	}
	pair := provider.ComparisonPair{Stage: provider.StageWrite, ExtraCandidates: []provider.Selection{{Stage: provider.StageWrite, Slot: provider.SlotCandidateC, Ref: llm.ModelRef{ProviderID: "p", ModelID: "c"}}}}
	wire := toProtoPair(pair)
	if len(wire.GetExtraCandidates()) != 1 || wire.GetExtraCandidates()[0].GetSlot() != postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_C {
		t.Fatalf("wire extras: %+v", wire)
	}
	err := toConnectError("save lab extras", &provider.LabCandidateError{Slot: provider.SlotCandidateD, Ref: llm.ModelRef{ProviderID: "p", ModelID: "duplicate"}, Cause: provider.ErrDuplicateCandidates})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v", connect.CodeOf(err))
	}
	detail := detailWithParams(t, err)
	if detail.GetReason() != "MODEL_CANDIDATES_DUPLICATE" || detail.GetParams()["slot"] != "candidate_d" || detail.GetParams()["model"] != "p/duplicate" {
		t.Fatalf("detail = %+v", detail)
	}
}

func TestSaveLabExtraCandidatesRequiresActorAndValidStage(t *testing.T) {
	h := NewHandler(nil)
	request := connect.NewRequest(&postpilotv1.SaveLabExtraCandidatesRequest{Stage: postpilotv1.Stage_STAGE_WRITE})
	if _, err := h.SaveLabExtraCandidates(context.Background(), request); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("no actor: %v", err)
	}
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "alice", Plan: plan.Free})
	request.Msg.Stage = postpilotv1.Stage(999)
	if _, err := h.SaveLabExtraCandidates(ctx, request); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("unknown stage: %v", err)
	}
}
