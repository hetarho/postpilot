package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
)

// recordingCandidates stands in for the guideline context: it records what reached it and can
// fail, which must never fail the work that caused it.
type recordingCandidates struct {
	recorded []string
	detached []string
	err      error
}

func (r *recordingCandidates) Record(_ context.Context, userID, projectID, request string) error {
	r.recorded = append(r.recorded, userID+"|"+projectID+"|"+request)
	return r.err
}

func (r *recordingCandidates) DetachProject(_ context.Context, userID, projectID string) error {
	r.detached = append(r.detached, userID+"|"+projectID)
	return r.err
}

// withCandidates rebuilds the generation side with a candidate recorder; the constructor binds
// the project service to it as well.
func (h *generationHarness) withCandidates(candidates clip.GuidelineCandidates) {
	deps := generationDeps(generationFinisher{h.store}, &quotePricing{}, nil)
	deps.Candidates = candidates
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, h.clipJobs(), h.cfg, deps)
}

// GUIDE-7: a completed revision's frozen request becomes a 영상 지침 candidate, and a recorder
// that fails leaves the revision the owner paid for successful.
func TestACompletedRevisionRecordsItsRequestAsAClipCandidate(t *testing.T) {
	h := revisionReady(t)
	candidates := &recordingCandidates{err: errors.New("guideline store unavailable")}
	h.withCandidates(candidates)
	startRevision(t, h, "고기 장면을 먼저 보여줘", clip.RevisionFlow)
	if err := h.run(t); err != nil {
		t.Fatalf("a failing candidate recorder failed the revision: %v", err)
	}
	want := []string{"alice|" + h.project.ID + "|고기 장면을 먼저 보여줘"}
	if !reflect.DeepEqual(candidates.recorded, want) {
		t.Fatalf("recorded %v, want %v", candidates.recorded, want)
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || after.EditPlanRevision <= h.project.EditPlanRevision {
		t.Fatalf("the revision did not land: %+v, %v", after.EditPlanRevision, err)
	}
}

// GUIDE-13: deleting a project detaches its candidates after the row is gone, and a failing
// detach never fails the delete.
func TestDeletingAProjectDetachesItsCandidates(t *testing.T) {
	h := generationSetup(t)
	candidates := &recordingCandidates{err: errors.New("guideline store unavailable")}
	h.withCandidates(candidates)
	if err := h.projects.DeleteProject(t.Context(), "alice", h.project.ID); err != nil {
		t.Fatalf("a failing detach failed the delete: %v", err)
	}
	if want := []string{"alice|" + h.project.ID}; !reflect.DeepEqual(candidates.detached, want) {
		t.Fatalf("detached %v, want %v", candidates.detached, want)
	}
	if _, err := h.projects.GetProject(t.Context(), "alice", h.project.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatalf("the project survived its delete: %v", err)
	}
}
