package store_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
)

// The directory badges a running generation and a failed attempt (CLIP-41), which it can only do
// if the LIST answer names each project's latest job the way the detail does. Everything else on
// the list answer stays as it was — `editing`, `accounting` and `latest_attempt` are detail only.
func TestListClipProjectsCarriesEachProjectsLatestJob(t *testing.T) {
	h := generationSetup(t)
	handler := cliprpc.NewHandler(h.projects).WithGeneration(h.service, h.queue)
	ctx := auth.WithUser(context.Background(), "alice")

	idle, err := h.projects.CreateProject(ctx, "alice", clip.ProjectInput{
		Title:            "no attempt yet",
		VideoTemplateID:  h.template.ID,
		Ratio:            "vertical",
		TargetDurationMS: 15000,
		Answers:          []clip.Answer{{Label: "place", Text: "제주"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	list := func() map[string]*v1.ClipProject {
		t.Helper()
		response, err := handler.ListClipProjects(ctx, connect.NewRequest(&v1.ListClipProjectsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		byID := map[string]*v1.ClipProject{}
		for _, p := range response.Msg.Projects {
			byID[p.Id] = p
		}
		if len(byID) != 2 {
			t.Fatalf("listed %d projects, want 2", len(byID))
		}
		return byID
	}

	for id, p := range list() {
		if p.LatestJob != nil {
			t.Fatalf("project %s invented a job: %+v", id, p.LatestJob)
		}
		// Detail-only fields must not join the list answer.
		if p.Editing != nil || p.Accounting != nil || p.LatestAttempt != nil {
			t.Fatalf("project %s leaked detail fields: %+v", id, p)
		}
	}

	jobID := h.start(t)
	started := list()
	if got := started[h.project.ID].LatestJob; got == nil || got.Id != jobID {
		t.Fatalf("running job missing from the list row: %+v", got)
	}
	if got := started[idle.ID].LatestJob; got != nil {
		t.Fatalf("a project with no attempt of its own took another's job: %+v", got)
	}

	failed, err := h.queue.FailQueued(ctx, jobID, "alice", job.Failure{Reason: "CLIP_PROCESSING_FAILED"})
	if err != nil || !failed {
		t.Fatal(failed, err)
	}
	after := list()
	if got := after[h.project.ID].LatestJob; got == nil || got.Status != "failed" {
		t.Fatalf("failed attempt not reported on the list row: %+v", got)
	}

	// The detail answer is unchanged by any of this — same job, from the same read.
	detail, err := handler.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: h.project.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if detail.Msg.Project.LatestJob == nil || detail.Msg.Project.LatestJob.Id != jobID {
		t.Fatalf("detail lost its latest job: %+v", detail.Msg.Project.LatestJob)
	}
}

// A handler with no job reader wired — the shape every other clip rpc test builds — still answers
// the list rather than panicking on a nil queue.
func TestListClipProjectsWithoutAJobReader(t *testing.T) {
	h := generationSetup(t)
	handler := cliprpc.NewHandler(h.projects)
	ctx := auth.WithUser(context.Background(), "alice")
	response, err := handler.ListClipProjects(ctx, connect.NewRequest(&v1.ListClipProjectsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.Projects) != 1 || response.Msg.Projects[0].LatestJob != nil {
		t.Fatalf("unexpected answer: %+v", response.Msg.Projects)
	}
}
