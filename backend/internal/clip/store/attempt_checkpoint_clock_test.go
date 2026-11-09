package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

func failCheckpointAttempt(t *testing.T, h *generationHarness, id string) {
	t.Helper()
	if picked, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil || picked.ID != id {
		t.Fatal("attempt did not start", picked.ID, err)
	}
	if err := h.jobs.Finish(t.Context(), id, job.StatusFailed, &job.Failure{Reason: "JOB_INTERRUPTED"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := h.sources.ReleaseAttempt(t.Context(), "alice", id, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestAttemptCheckpointSuccessorIgnoresWallClockOrder(t *testing.T) {
	const earlier = "2026-01-01T00:00:00.000000000Z"
	const later = "2026-01-02T00:00:00.000000000Z"
	for _, order := range []struct{ name, predecessor, successor string }{
		{"forward", earlier, later},
		{"equal", earlier, earlier},
		{"rollback", later, earlier},
	} {
		for _, status := range []string{job.StatusQueued, job.StatusRunning} {
			t.Run(order.name+"/"+status, func(t *testing.T) {
				h := generationSetup(t)
				first := h.start(t)
				old := clip.AttemptCheckpoint{Version: 1, JobID: first, Stage: "analyze", CompletedChunks: 1}
				if err := h.store.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, old); err != nil {
					t.Fatal(err)
				}
				failCheckpointAttempt(t, h, first)
				second := h.start(t)
				if _, err := h.db.Writer.Exec(`UPDATE generation_jobs SET created_at=CASE WHEN id=? THEN ? ELSE ? END WHERE clip_project_id=?`, first, order.predecessor, order.successor, h.project.ID); err != nil {
					t.Fatal(err)
				}
				if status == job.StatusRunning {
					if picked, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil || picked.ID != second {
						t.Fatal("successor did not start", picked.ID, err)
					}
				}
				current := clip.AttemptCheckpoint{Version: 1, JobID: second, Stage: "analyze", CompletedChunks: 2}
				if err := h.store.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, current); err != nil {
					t.Fatal("sole active successor lost checkpoint ownership", err)
				}
				if err := h.store.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, old); !errors.Is(err, clip.ErrNotFound) {
					t.Fatal("terminal predecessor rewrote the successor", err)
				}
				if got, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, second); err != nil || got == nil || got.CompletedChunks != 2 {
					t.Fatal("successor diagnostic unavailable", got, err)
				}
				if got, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, first); err != nil || got != nil {
					t.Fatal("predecessor borrowed successor diagnostic", got, err)
				}
				if status == job.StatusQueued {
					failCheckpointAttempt(t, h, second)
				} else if err := h.jobs.Finish(t.Context(), second, job.StatusFailed, &job.Failure{Reason: "JOB_INTERRUPTED"}, time.Now()); err != nil {
					t.Fatal(err)
				}
				if got, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, second); err != nil || got == nil || got.CompletedChunks != 2 {
					t.Fatal("terminal successor lost retained diagnostic", got, err)
				}
			})
		}
	}
}

func TestAttemptCheckpointIdentityAndLifecycleFences(t *testing.T) {
	for _, guard := range []string{"owner", "project", "job", "done", "failed", "cancelled", "deleting", "finalized", "deleted"} {
		t.Run(guard, func(t *testing.T) {
			h := generationSetup(t)
			id := h.start(t)
			c := clip.AttemptCheckpoint{Version: 1, JobID: id, Stage: "analyze", CompletedChunks: 1}
			if err := h.store.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, c); err != nil {
				t.Fatal(err)
			}
			user, project := "alice", h.project.ID
			readable := false
			switch guard {
			case "owner":
				user = "bob"
			case "project":
				other, err := h.projects.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "other checkpoint project", Language: "ko", Ratio: "vertical", TargetDurationMS: 30000})
				if err != nil {
					t.Fatal(err)
				}
				project = other.ID
			case "job":
				c.JobID = "another-job"
			case "done", "failed", "cancelled":
				if _, err := h.db.Writer.Exec(`UPDATE generation_jobs SET status=?,cancel_requested_at=CASE WHEN ?='cancelled' THEN '2026-01-01T00:00:00Z' ELSE NULL END WHERE id=?`, guard, guard, id); err != nil {
					t.Fatal(err)
				}
				readable = true
			default:
				failCheckpointAttempt(t, h, id)
				statement := "UPDATE clip_projects SET deleting=1 WHERE id=?"
				if guard == "finalized" {
					statement = "UPDATE clip_projects SET finalized_at='2026-01-01T00:00:00Z',finalized_plan_revision=1,edit_plan_revision=1,rendered_plan_revision=1,finalized_result_key='retained',result_key='retained',result_id='retained',source_access_revoked_at='2026-01-01T00:00:00Z' WHERE id=?"
				} else if guard == "deleted" {
					statement = "DELETE FROM clip_projects WHERE id=?"
				}
				if _, err := h.db.Writer.Exec(statement, h.project.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := h.store.SaveAttemptCheckpoint(t.Context(), user, project, c); !errors.Is(err, clip.ErrNotFound) {
				t.Fatal("checkpoint write escaped guard", err)
			}
			got, err := h.store.GetAttemptCheckpoint(t.Context(), user, project, c.JobID)
			if err != nil || (got != nil) != readable || got != nil && got.CompletedChunks != 1 {
				t.Fatal("checkpoint read escaped identity/lifecycle guard or lost terminal evidence", got, err)
			}
		})
	}
}

func TestAttemptCheckpointReplacementFollowsExistingTrigger(t *testing.T) {
	for _, kind := range []string{clip.JobKindSampleBrowserRender, clip.JobKindUploadText} {
		t.Run(kind, func(t *testing.T) {
			h := generationSetup(t)
			first := h.start(t)
			c := clip.AttemptCheckpoint{Version: 1, JobID: first, Stage: "analyze", CompletedChunks: 1}
			if err := h.store.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, c); err != nil {
				t.Fatal(err)
			}
			failCheckpointAttempt(t, h, first)
			// The current migration preserves evidence for upload text only; sampling replaces it.
			const sideJob = "checkpoint-side-job"
			if err := h.jobs.Insert(t.Context(), job.Job{ID: sideJob, UserID: "alice", Kind: kind, Subjects: []job.Subject{{Dimension: clip.JobSubject, ID: h.project.ID}}, CreatedAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
				t.Fatal(err)
			}
			retained := kind == clip.JobKindUploadText
			var count int
			if err := h.db.Reader.QueryRow("SELECT count(*) FROM clip_attempt_checkpoints WHERE project_id=?", h.project.ID).Scan(&count); err != nil || (count == 1) != retained {
				t.Fatal("job insertion changed established checkpoint replacement", count, err)
			}
			got, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, first)
			if err != nil || (got != nil) != retained || got != nil && got.CompletedChunks != 1 {
				t.Fatal("stored diagnostic did not follow existing trigger", got, err)
			}
			if got, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, sideJob); err != nil || got != nil {
				t.Fatal("new job borrowed the attempt diagnostic", got, err)
			}
			if changed, err := h.jobs.FailQueued(t.Context(), sideJob, "alice", job.Failure{Reason: "JOB_INTERRUPTED"}, time.Now()); err != nil || !changed {
				t.Fatal("job did not stop", changed, err)
			}
			second := h.start(t)
			if err := h.db.Reader.QueryRow("SELECT count(*) FROM clip_attempt_checkpoints WHERE project_id=?", h.project.ID).Scan(&count); err != nil || count != 0 {
				t.Fatal("new generation did not atomically delete prior diagnostic", count, err)
			}
			for _, id := range []string{first, second} {
				if got, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id); err != nil || got != nil {
					t.Fatal("replacement exposed or borrowed a prior diagnostic", got, err)
				}
			}
			if err := h.store.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, c); !errors.Is(err, clip.ErrNotFound) {
				t.Fatal("replaced terminal attempt recreated its diagnostic", err)
			}
		})
	}
}
