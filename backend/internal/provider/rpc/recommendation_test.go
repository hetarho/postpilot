package rpc

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/provider"
)

// MODEL-70: a refused draft travels as one InvalidArgument whose params name each field with
// its cause; the limit is a precondition carrying the number.
func TestRecommendationWriteRefusalsCarryTheirParams(t *testing.T) {
	draft := toConnectError("save recommendation set", &provider.SetDraftRefusal{Fields: map[string]string{
		"label": provider.DraftRequired, "write_candidate_b": provider.DraftDuplicate,
	}})
	if connect.CodeOf(draft) != connect.CodeInvalidArgument {
		t.Fatalf("draft code = %v", connect.CodeOf(draft))
	}
	detail := detailWithParams(t, draft)
	if detail.GetReason() != "MODEL_SET_INVALID" || detail.GetParams()["label"] != "required" ||
		detail.GetParams()["write_candidate_b"] != "duplicate" || detail.GetParams()["fields"] != "label,write_candidate_b" {
		t.Fatalf("draft detail = %#v", detail)
	}

	limit := toConnectError("save recommendation set", &provider.SetLimitError{Limit: provider.MaxRecommendationSets})
	if connect.CodeOf(limit) != connect.CodeFailedPrecondition {
		t.Fatalf("limit code = %v", connect.CodeOf(limit))
	}
	if detail := detailWithParams(t, limit); detail.GetReason() != "MODEL_SET_LIMIT" || detail.GetParams()["limit"] != "10" {
		t.Fatalf("limit detail = %#v", detail)
	}
}

// The transport refuses a malformed stage list; a stage left out is a draft to annotate.
func TestFromProtoDraftRefusesOnlyAMalformedStageList(t *testing.T) {
	ref := &postpilotv1.ModelRef{ProviderId: "openrouter", ModelId: "m"}
	for name, selections := range map[string][]*postpilotv1.RecommendationStageSelection{
		"unspecified stage": {{Stage: postpilotv1.Stage_STAGE_UNSPECIFIED, Active: ref}},
		"repeated stage":    {{Stage: postpilotv1.Stage_STAGE_WRITE, Active: ref}, {Stage: postpilotv1.Stage_STAGE_WRITE, Active: ref}},
		"analyze pair":      {{Stage: postpilotv1.Stage_STAGE_ANALYZE, Active: ref, CandidateA: ref}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := fromProtoDraft(&postpilotv1.SaveRecommendationSetRequest{Label: "Set", Selections: selections})
			if err == nil {
				t.Fatal("a malformed stage list was accepted")
			}
			if detail := providerErrorDetail(t, toConnectError("save recommendation set", err)); detail.GetReason() != "MODEL_STAGE_INVALID" {
				t.Fatalf("reason = %q", detail.GetReason())
			}
		})
	}

	draft, err := fromProtoDraft(&postpilotv1.SaveRecommendationSetRequest{Id: "set", Label: "Set", Selections: []*postpilotv1.RecommendationStageSelection{
		{Stage: postpilotv1.Stage_STAGE_OBSERVE, Active: ref, CandidateA: ref},
	}})
	if err != nil || draft.ID != "set" || len(draft.Selections) != 1 || draft.Selections[0].Stage != provider.StageObserve || draft.Selections[0].CandidateA.ModelID != "m" {
		t.Fatalf("draft = %+v, err = %v", draft, err)
	}
}

// detailWithParams decodes the one AppErrorDetail a refusal carries. Unlike
// providerErrorDetail it allows params: these two reasons exist to carry them.
func detailWithParams(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || len(connectErr.Details()) != 1 {
		t.Fatalf("error = %#v", err)
	}
	value, valueErr := connectErr.Details()[0].Value()
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if valueErr != nil || !ok {
		t.Fatalf("detail = %T, %v", value, valueErr)
	}
	return detail
}
