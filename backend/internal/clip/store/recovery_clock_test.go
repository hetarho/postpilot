package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

func TestRecoverySuccessorSurvivesAClockRollback(t *testing.T) {
	h := generationSetup(t)
	first := h.start(t)
	if picked, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil || picked.ID != first {
		t.Fatal("first attempt did not start", picked, err)
	}
	if err := h.store.SaveRecovery(t.Context(), "alice", h.project.ID, clip.RecoveryState{Version: 1, JobID: first}); err != nil {
		t.Fatal(err)
	}
	if err := h.jobs.Finish(t.Context(), first, job.StatusFailed, &job.Failure{Reason: "JOB_INTERRUPTED"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := h.sources.ReleaseAttempt(t.Context(), "alice", first, time.Now()); err != nil {
		t.Fatal(err)
	}
	second := h.start(t)
	// A successor can have an earlier wall-clock timestamp after an NTP or VM
	// clock adjustment. The active attempt still owns the recovery checkpoint.
	if _, err := h.db.Writer.Exec(`UPDATE generation_jobs SET created_at=CASE WHEN id=? THEN '2026-01-02T00:00:00.000000000Z' ELSE '2026-01-01T00:00:00.000000000Z' END WHERE clip_project_id=?`, first, h.project.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.store.SaveRecovery(t.Context(), "alice", h.project.ID, clip.RecoveryState{Version: 1, JobID: second}); err != nil {
		t.Fatal("active successor lost its checkpoint after clock rollback", err)
	}
	if err := h.store.SaveRecovery(t.Context(), "alice", h.project.ID, clip.RecoveryState{Version: 1, JobID: first}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("terminal predecessor rewrote the successor's checkpoint", err)
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil || state.JobID != second {
		t.Fatal("successor checkpoint was not retained", state, err)
	}
}
