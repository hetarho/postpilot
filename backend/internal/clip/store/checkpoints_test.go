package store_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store"
)

func TestFailedAttemptKeepsEvidenceWithoutReplacingSuccessfulState(t *testing.T) {
	h := generationSetup(t)
	id := h.start(t)
	h.planner.errorPlan = clip.WithAttemptDiagnostic(errors.New("private provider response"), clip.AttemptDiagnostic{Check: "plan_timeline", Phase: "timeline_grow", Values: map[string]int{"target_ms": 30000, "after_ms": 12000}})
	if h.run(t) == nil {
		t.Fatal("expected failed plan")
	}
	// A new store instance models a reopened page/process, not an in-memory cache.
	reopened := store.New(h.db.Writer, h.db.Reader)
	c, err := reopened.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id)
	if err != nil || c == nil || c.Stage != "plan" || c.CompletedChunks != 3 || c.TotalChunks != 3 || c.CompletedSources != 2 || len(c.Observations) != 2 || len(c.Observations[0].Segments) != 2 || c.Diagnostic.Values["after_ms"] != 12000 {
		t.Fatalf("checkpoint: %+v %v", c, err)
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || p.Analysis != "" || p.EditPlan != "" || p.Result != nil {
		t.Fatal("failed checkpoint became canonical content", p, err)
	}
	if foreign, err := reopened.GetAttemptCheckpoint(t.Context(), "bob", h.project.ID, id); err != nil || foreign != nil {
		t.Fatal("foreign checkpoint disclosed", err)
	}
	if err := reopened.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, *c); err == nil {
		t.Fatal("terminal worker may not rewrite evidence")
	}
	// An accepted new attempt drops the old checkpoint and cannot inherit it.
	newID := h.start(t)
	if old, err := reopened.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id); err != nil || old != nil {
		t.Fatal("old checkpoint survived replacement", err)
	}
	if next, err := reopened.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, newID); err != nil || next != nil {
		t.Fatal("new attempt borrowed evidence", err)
	}
}

func TestInterruptionPreservesCompletedChunksAndFencesCancellation(t *testing.T) {
	h := generationSetup(t)
	id := h.start(t)
	j, err := h.jobs.PickNextQueued(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	err = h.service.Run(ctx, "alice", id, h.project.ID, j.Payload, func(stage string, done, total int) {
		if stage == "analyze" && done == 1 {
			cancel()
		}
	})
	if err == nil {
		t.Fatal("cancelled context unexpectedly succeeded")
	}
	c, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id)
	if err != nil || c == nil || c.CompletedChunks != 1 || c.CompletedSources != 0 || len(c.Observations[0].Segments) != 1 || len(c.Observations[1].Segments) != 0 || h.planner.plans != 0 {
		t.Fatalf("partial evidence lost: %+v %v", c, err)
	}
	if _, err := h.db.Writer.Exec("UPDATE generation_jobs SET status='failed' WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if recovered, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id); err != nil || recovered == nil || recovered.CompletedChunks != 1 {
		t.Fatal("recovery lost checkpoint", err)
	}
	if _, err := h.db.Writer.Exec("DELETE FROM clip_projects WHERE id=?", h.project.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := h.db.Reader.QueryRow("SELECT count(*) FROM clip_attempt_checkpoints").Scan(&count); err != nil || count != 0 {
		t.Fatal("project deletion retained checkpoint", count, err)
	}
}

func TestRenderFailureRetainsSelectedOriginalRanges(t *testing.T) {
	h := generationSetup(t)
	id := h.start(t)
	h.renderer.fail = clip.ErrInvalidMedia
	if h.run(t) == nil {
		t.Fatal("expected render failure")
	}
	c, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id)
	if err != nil || c == nil || c.Stage != "render" || len(c.Diagnostic.Ranges) != 1 || !c.Diagnostic.Ranges[0].Valid || c.CompletedSources != 2 {
		t.Fatalf("render failure lost successful plan ranges: %+v %v", c, err)
	}
}

func TestAttemptCheckpointPreservesPriorResultAndDisappearsOnFinalization(t *testing.T) {
	h := generationSetup(t)
	seedCompletedGeneration(t, h)
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := h.start(t)
	h.planner.errorPlan = errors.New("failed writer")
	if h.run(t) == nil {
		t.Fatal("expected failure")
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || after.Analysis != before.Analysis || after.EditPlan != before.EditPlan || !reflect.DeepEqual(after.Result, before.Result) {
		t.Fatal("previous successful state changed", err)
	}
	if c, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id); err != nil || c == nil {
		t.Fatal("missing attempt", err)
	}
	_, err = h.db.Writer.Exec("UPDATE clip_projects SET finalized_at=?, finalized_plan_revision=edit_plan_revision, finalized_result_key=result_key, source_access_revoked_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id); err != nil || c != nil {
		t.Fatal("finalization exposed intermediate work", err)
	}
}

func TestCheckpointBudgetPreservesDiagnosticsAndMarksOmittedEvidence(t *testing.T) {
	h := generationSetup(t)
	id := h.start(t)
	c := clip.AttemptCheckpoint{Version: 1, JobID: id, Stage: "plan", CompletedChunks: 3, TotalChunks: 3, Diagnostic: clip.AttemptDiagnostic{Check: "plan_timeline", Phase: "timeline_grow", Values: map[string]int{"remaining_ms": 3000}}, Observations: []clip.SourceAnalysis{{Segments: []clip.Segment{{Event: strings.Repeat("a", clip.AttemptCheckpointMaxBytes)}}}}}
	if err := h.store.SaveAttemptCheckpoint(t.Context(), "alice", h.project.ID, c); err != nil {
		t.Fatal(err)
	}
	got, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id)
	if err != nil || got == nil || !got.EvidenceLimited || got.CompletedChunks != 3 || got.Diagnostic.Values["remaining_ms"] != 3000 || len(got.Observations[0].Segments) != 0 || len(c.Observations[0].Segments) != 1 {
		t.Fatal("budget hid failure or mutated worker", got, err)
	}
}

func TestCheckpointFailureStopsBeforeAnotherPaidCall(t *testing.T) {
	for _, completed := range []int{0, 1, 3} {
		t.Run(fmt.Sprint(completed), func(t *testing.T) {
			h := generationSetup(t)
			id := h.start(t)
			j, err := h.jobs.PickNextQueued(t.Context(), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			broken := false
			err = h.service.Run(t.Context(), "alice", id, h.project.ID, j.Payload, func(stage string, done, total int) {
				if !broken && stage == "analyze" && done == completed {
					// Simulate a durable-write outage after exactly this much paid work.
					_, e := h.db.Writer.Exec(`CREATE TRIGGER reject_checkpoint BEFORE INSERT ON clip_attempt_checkpoints BEGIN SELECT RAISE(ABORT, 'write unavailable'); END`)
					if e != nil {
						t.Fatal(e)
					}
					broken = true
				}
			})
			if !broken || !errors.Is(err, clip.ErrAttemptCheckpointUnavailable) || h.planner.observe != completed || h.planner.plans != 0 || h.renderer.calls != 0 {
				t.Fatalf("unrecorded paid work continued: %v observe=%d plan=%d", err, h.planner.observe, h.planner.plans)
			}
		})
	}
}

func TestObservationDiagnosticKeepsWorkerLocationAndCompletedWork(t *testing.T) {
	h := generationSetup(t)
	id := h.start(t)
	j, err := h.jobs.PickNextQueued(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	err = h.service.Run(t.Context(), "alice", id, h.project.ID, j.Payload, func(stage string, done, total int) {
		if stage == "analyze" && done == 2 {
			h.planner.observeErr = clip.WithAttemptDiagnostic(errors.New("private-canary"), clip.AttemptDiagnostic{Check: "observe_subject_bounds", Phase: "observation", Values: map[string]int{"source": 99, "chunk": 99, "segment": 1, "subject_x_ppm": 200000, "subject_width_ppm": 1000000}})
		}
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	c, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id)
	if err != nil || c == nil || c.CompletedChunks != 2 || c.CompletedSources != 1 || c.Diagnostic.Check != "observe_subject_bounds" || c.Diagnostic.Values["source"] != 2 || c.Diagnostic.Values["chunk"] != 3 || c.Diagnostic.Values["segment"] != 1 || c.Diagnostic.Values["subject_width_ppm"] != 1000000 || len(c.Observations[0].Segments) != 2 || len(c.Observations[1].Segments) != 0 || h.planner.plans != 0 {
		t.Fatalf("lost completed work or authoritative locator: %+v %v", c, err)
	}
}
