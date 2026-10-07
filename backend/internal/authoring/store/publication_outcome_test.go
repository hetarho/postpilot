package store_test

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/authoring"
)

func TestConfirmedPublicationOutcomeIsDurableAndOriginalReplayKeepsItsOutcome(t *testing.T) {
	h := fixture(t)
	h.svc = authoring.NewService(h.store, h.models, h.jobs, realTargets(h), budget{}, estimates{})
	ctx := context.Background()
	state := create(t, h, authoring.VideoTemplate)
	source := *state.WorkingSource
	source.Name = "Owner structure"
	source.Body = videoBody("Owner content")
	state, err := h.svc.PatchDraft(ctx, authoring.DraftMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "outcome-source", WorkingSource: source})
	if err != nil {
		t.Fatal(err)
	}
	firstRequest := authoring.ResetMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "outcome-create"}
	created, err := h.svc.SaveWithKey(ctx, firstRequest, false)
	if err != nil || created.Saved == nil || created.Saved.Outcome != "created" {
		t.Fatal("new publication outcome unavailable", err)
	}
	reopened, err := secondService(t, h).Get(ctx, "alice", state.ID)
	if err != nil || reopened.Saved.Outcome != "created" {
		t.Fatal("confirmed outcome lost on reopen", err)
	}
	edited := patch(t, h, reopened, "outcome-edit", videoBody("Owner revised content"))
	updated, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: edited.Revision, OperationKey: "outcome-update"}, false)
	if err != nil || updated.Saved.Outcome != "updated" {
		t.Fatal("existing publication reported a new creation", err)
	}
	summaries, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.VideoTemplate})
	if err != nil || len(summaries) != 1 || summaries[0].LastPublication == nil || summaries[0].LastPublication.Outcome != "updated" {
		t.Fatal("directory outcome unavailable", err)
	}
	replay, err := h.svc.SaveWithKey(ctx, firstRequest, false)
	if err != nil || replay.Saved.Outcome != "created" || replay.Saved.ID != updated.Saved.ID {
		t.Fatal("original receipt changed its outcome", err)
	}
	latest, err := h.svc.Get(ctx, "alice", state.ID)
	if err != nil || latest.Selected.Body != updated.Selected.Body || latest.Saved.Outcome != "updated" {
		t.Fatal("receipt replay undid a later edit", err)
	}
}
