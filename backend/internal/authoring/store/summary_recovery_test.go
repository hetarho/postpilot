package store_test

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/llm"
)

func summaryForTarget(t *testing.T, h harness, id string) authoring.Summary {
	t.Helper()
	rows, _, err := h.svc.ListSummaries(context.Background(), authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.TargetID == id {
			return row
		}
	}
	t.Fatalf("missing target %s in summaries %+v", id, rows)
	return authoring.Summary{}
}

func TestDirectorySummarySettlesAdmittedTerminalJobsWithoutOpeningTheirSessions(t *testing.T) {
	for _, terminal := range []string{"done", "failed", "cancelled", "invalid_output"} {
		t.Run(terminal, func(t *testing.T) {
			h := fixture(t)
			ctx := context.Background()
			state, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "original")
			if err != nil {
				t.Fatal(err)
			}
			jobID, active, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: state.ID, ExpectedRevision: state.Revision, RequestID: "request", Mode: authoring.Refine, Prompt: "Please improve this rule", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
			if err != nil {
				t.Fatal(err)
			}
			if summary := summaryForTarget(t, h, "owned"); summary.SessionID != state.ID || summary.ActiveJobID != jobID {
				t.Fatalf("queued summary=%+v", summary)
			}
			if terminal == "done" {
				h.models.text = `{"artifact":{"name":"Refined rule","description":"","body":"The edited rule","title_area":""},"reply":"지침을 수정했어요."}`
				job, err := h.jobs.Get(ctx, "alice", jobID)
				if err != nil {
					t.Fatal(err)
				}
				if err := h.svc.Run(ctx, authoring.Run{ID: jobID, UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload}, func(string, int, int) {}); err != nil {
					t.Fatal(err)
				}
			}
			jobStatus := terminal
			if terminal == "invalid_output" {
				jobStatus = "done"
			}
			h.jobs.status(jobID, jobStatus)
			// A newer idle saved baseline must not hide the older paid request.
			fresh, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "fresh-baseline")
			if err != nil || fresh.HasUnpublishedChanges || fresh.ActiveJobID != "" {
				t.Fatalf("fresh baseline=%+v %v", fresh, err)
			}
			calls, enqueues, publications := h.models.calls, h.jobs.enqueues, h.targets.creates
			summary := summaryForTarget(t, h, "owned")
			if summary.SessionID != state.ID || summary.ActiveJobID != "" || summary.Revision <= active.Revision || !summary.SavedAvailable {
				t.Fatalf("directory did not settle terminal request: %+v", summary)
			}
			stored, err := h.store.Get(ctx, "alice", state.ID)
			if err != nil || stored.ActiveJobID != "" || stored.ActiveRequestID != "" {
				t.Fatalf("terminal state=%+v %v", stored, err)
			}
			if terminal == "done" {
				if stored.WorkingSource.Body != "The edited rule" || !summary.HasUnpublishedChanges {
					t.Fatal("completed draft was not recovered by the directory")
				}
			} else if stored.WorkingSource.Body != "old body" || stored.PendingRequest != "Please improve this rule" || stored.Phase != "failed" {
				t.Fatal("failed/cancelled request lost retained work")
			}
			_ = summaryForTarget(t, h, "owned")
			if h.models.calls != calls || h.jobs.enqueues != enqueues || h.targets.creates != publications {
				t.Fatal("directory reads repeated AI, admission or publication")
			}
			foreign, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "bob", Kind: authoring.PostGuideline})
			if err != nil || len(foreign) != 0 {
				t.Fatalf("foreign summary=%+v %v", foreign, err)
			}
		})
	}
}

func TestTargetSummaryPrioritizesCurrentPrivateWorkAndLeavesUnsavedCreationsIndependent(t *testing.T) {
	for _, work := range []string{"unpublished", "pending publication", "conflict", "recommendations"} {
		t.Run(work, func(t *testing.T) {
			h := fixture(t)
			ctx := context.Background()
			state, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "original")
			if err != nil {
				t.Fatal(err)
			}
			switch work {
			case "unpublished":
				state = patch(t, h, state, "Edited rule", "Owner's unpublished direction")
			case "pending publication":
				h.targets.failAfterCommit = true
				if _, err := h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "save"}, false); err == nil {
					t.Fatal("publication fixture did not remain pending")
				}
			case "conflict":
				if err := h.store.PublicationFailure(ctx, "alice", state.ID, "AUTHORING_SAVE_CONFLICT", false); err != nil {
					t.Fatal(err)
				}
			case "recommendations":
				state = recommend(t, h, state)
				if state.HasUnpublishedChanges || len(state.Candidates) != 8 {
					t.Fatal("recommendation fixture is not an unselected batch")
				}
			}
			fresh, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "fresh-baseline")
			if err != nil {
				t.Fatal(err)
			}
			if row := summaryForTarget(t, h, "owned"); row.SessionID != state.ID {
				t.Fatalf("fresh baseline hid %s: %+v", work, row)
			}
			// A newer meaningful direct draft replaces the older meaningful work.
			fresh = patch(t, h, fresh, "Newest current work", "The new explicitly edited draft")
			if row := summaryForTarget(t, h, "owned"); row.SessionID != fresh.ID {
				t.Fatalf("latest meaningful work was hidden: %+v", row)
			}
			for _, key := range []string{"new-one", "new-two"} {
				if _, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "", key); err != nil {
					t.Fatal(err)
				}
			}
			unsaved, _, err := h.svc.ListSummaries(ctx, authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline, UnsavedOnly: true})
			if err != nil || len(unsaved) != 2 || unsaved[0].SessionID == unsaved[1].SessionID {
				t.Fatalf("unsaved creations were grouped/hidden: %+v %v", unsaved, err)
			}
		})
	}
}

func TestSavedHistoricalRecommendationBatchDoesNotHideAFreshBaseline(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	state, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "original")
	if err != nil {
		t.Fatal(err)
	}
	state = recommend(t, h, state)
	state, err = h.svc.Select(ctx, "alice", state.ID, state.Revision, state.Candidates[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err = h.svc.SaveWithKey(ctx, authoring.ResetMutation{UserID: "alice", SessionID: state.ID, ExpectedRevision: state.Revision, OperationKey: "save"}, false)
	if err != nil || state.Phase != "saved" || len(state.Candidates) == 0 || state.HasUnpublishedChanges {
		t.Fatalf("saved historical suggestions=%+v %v", state, err)
	}
	fresh, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "fresh-baseline")
	if err != nil {
		t.Fatal(err)
	}
	if row := summaryForTarget(t, h, "owned"); row.SessionID != fresh.ID {
		t.Fatalf("saved historical batch hid fresh baseline: %+v", row)
	}
	// The stored historical candidates remain readable; selecting the visible row
	// did not delete history or silently discard another session's private payload.
	stored, err := h.store.Get(ctx, "alice", state.ID)
	if err != nil || len(stored.Candidates) != len(state.Candidates) || stored.Candidates[0].ID != state.Candidates[0].ID {
		t.Fatal("historical candidates were lost")
	}
}

func TestPaginatedDirectorySummaryRetainsItsTerminalRowAndOriginalNextCursor(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	for _, key := range []string{"tail-one", "tail-two"} {
		if _, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "", key); err != nil {
			t.Fatal(err)
		}
	}
	state, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "owned", "original")
	if err != nil {
		t.Fatal(err)
	}
	jobID, active, err := h.svc.Start(ctx, "alice", authoring.Start{SessionID: state.ID, ExpectedRevision: state.Revision, RequestID: "request", Mode: authoring.Refine, Prompt: "Improve this rule", WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"front-one", "front-two"} {
		if _, err := h.svc.Create(ctx, "alice", authoring.PostGuideline, "", key); err != nil {
			t.Fatal(err)
		}
	}
	h.models.text = `{"artifact":{"name":"Directory completed rule","description":"","body":"The terminal result","title_area":""},"reply":"지침을 수정했어요."}`
	job, _ := h.jobs.Get(ctx, "alice", jobID)
	if err := h.svc.Run(ctx, authoring.Run{ID: jobID, UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	h.jobs.status(jobID, "done")
	calls, enqueues, publications := h.models.calls, h.jobs.enqueues, h.targets.creates
	query := authoring.SummaryQuery{UserID: "alice", Kind: authoring.PostGuideline, PageSize: 2}
	first, next, err := h.svc.ListSummaries(ctx, query)
	if err != nil || len(first) != 2 || next == "" {
		t.Fatalf("first page=%+v next=%s err=%v", first, next, err)
	}
	query.PageToken = next
	second, next, err := h.svc.ListSummaries(ctx, query)
	if err != nil || len(second) != 2 || next == "" {
		t.Fatalf("second page=%+v next=%s err=%v", second, next, err)
	}
	var terminal *authoring.Summary
	for i := range second {
		if second[i].SessionID == state.ID {
			terminal = &second[i]
		}
	}
	if terminal == nil || terminal.ActiveJobID != "" || terminal.Revision <= active.Revision || terminal.DisplayName != "Directory completed rule" || !terminal.HasUnpublishedChanges || !terminal.UpdatedAt.After(first[len(first)-1].UpdatedAt) {
		t.Fatalf("terminal row was dropped or stale on page two: %+v", second)
	}
	query.PageToken = next
	third, last, err := h.svc.ListSummaries(ctx, query)
	if err != nil || len(third) != 1 || last != "" {
		t.Fatalf("original next cursor lost its tail: page=%+v next=%s err=%v", third, last, err)
	}
	seen := map[string]bool{}
	for _, rows := range [][]authoring.Summary{first, second, third} {
		for _, row := range rows {
			if seen[row.SessionID] {
				t.Fatalf("pagination repeated %s", row.SessionID)
			}
			seen[row.SessionID] = true
		}
	}
	if len(seen) != 5 || h.models.calls != calls || h.jobs.enqueues != enqueues || h.targets.creates != publications {
		t.Fatal("directory pagination lost rows or issued new paid/publication work")
	}
}
