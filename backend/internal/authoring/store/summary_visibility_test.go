package store_test

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
)

// createWithWork is an unsaved creation holding its owner's direct edit, which a summary
// lists; a creation opened and left holds nothing and is not listed (see below).
func createWithWork(t *testing.T, h harness, kind authoring.Kind, key string) authoring.Session {
	t.Helper()
	s, err := h.svc.Create(context.Background(), "alice", kind, "", key)
	if err != nil {
		t.Fatal(err)
	}
	return patch(t, h, s, key+"-edit", "Owner's draft "+key)
}

func TestListSummariesHidesUnsavedCreationsWithoutOwnerWork(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	left, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "", "opened-and-left")
	if err != nil {
		t.Fatal(err)
	}
	edited := createWithWork(t, h, authoring.PostGuideline, "edited")
	recommended := recommend(t, h, create(t, h, authoring.PostGuideline))
	running, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "", "running")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: running.ID, ExpectedRevision: running.Revision, RequestID: "running-request", Mode: authoring.Recommend, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}}); err != nil {
		t.Fatal(err)
	}

	rows, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline, UnsavedOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, row := range rows {
		listed[row.SessionID] = true
	}
	if listed[left.ID] {
		t.Fatal("a creation opened and left is offered as work in progress")
	}
	for name, id := range map[string]string{"direct edit": edited.ID, "recommendations": recommended.ID, "running request": running.ID} {
		if !listed[id] {
			t.Fatalf("a creation holding a %s was hidden: %+v", name, rows)
		}
	}
	// Hidden, not deleted: the session is still stored and read by its id.
	if got, err := h.svc.Get(ctx, "alice", left.ID); err != nil || got.ID != left.ID {
		t.Fatalf("hidden session unreadable by id: %+v %v", got, err)
	}
	stored, _, err := h.store.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline, UnsavedOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range stored {
		found = found || row.SessionID == left.ID
	}
	if !found {
		t.Fatal("the hidden session is gone from the store")
	}
	// Another owner's view is unaffected and never sees alice's sessions.
	if foreign, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "bob", Kind: authoring.PostGuideline}); err != nil || len(foreign) != 0 {
		t.Fatalf("foreign summaries = %+v %v", foreign, err)
	}
}
