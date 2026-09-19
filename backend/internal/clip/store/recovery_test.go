package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// Legacy attempt evidence predates clip-observation-v2's complete-coverage and
// scene-status rules, so it is never promoted into a v2 generation: the
// successor re-observes and pays for every chunk, whatever the legacy record
// claimed to have completed (CLIP-93).
func TestLegacyAttemptEvidenceIsNeverReusedForNewGeneration(t *testing.T) {
	for _, limited := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "limited"}[limited], func(t *testing.T) {
			h := generationSetup(t)
			h.renderer.fail = clip.ErrInvalidMedia
			id := h.start(t)
			_ = h.run(t)
			checkpoint, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, id)
			if err != nil || checkpoint == nil {
				t.Fatal("missing retained legacy evidence", err)
			}
			checkpoint.EvidenceLimited = limited
			raw, err := json.Marshal(clip.RecoveryState{Version: 1, JobID: id, Legacy: checkpoint})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = h.db.Writer.Exec("UPDATE clip_recovery_states SET state_json=? WHERE project_id=?", string(raw), h.project.ID); err != nil {
				t.Fatal(err)
			}
			q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
			if err != nil {
				t.Fatal(err)
			}
			if q.Pricing.ReusedChunks != 0 || q.Pricing.RenderOnly() || q.Pricing.ObservationCalls != 3 {
				t.Fatalf("v1 evidence entered a v2 generation: reused=%d observe=%d skip=%v", q.Pricing.ReusedChunks, q.Pricing.ObservationCalls, q.Pricing.RenderOnly())
			}
		})
	}
}

func TestRecoveryRenderRestartSkipsCompletedWork(t *testing.T) {
	h := generationSetup(t)
	// The attempt fails after its plan is validated and durable: the render is
	// its own job now, so this is the interruption a continuation resumes from
	// (CLIP-151, CLIP-93).
	h.media.cleanupErr = errors.New("workspace cleanup failed")
	first := h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected attempt failure")
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil || state.JobID != first || len(state.Chunks) != 3 || state.Plan == "" {
		t.Fatal("candidate not durable", err)
	}
	if foreign, err := h.store.GetRecovery(t.Context(), "bob", h.project.ID); err != nil || foreign != nil {
		t.Fatal("foreign recovery exposed")
	}
	h.media.cleanupErr = nil
	h.planner.gate = llm.ErrModelUnavailable
	// Recreate the service to prove that continuation does not depend on memory.
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, h.clipJobs(), h.cfg, generationDeps(generationFinisher{h.store}, &quotePricing{}, nil))
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal("AI-free continuation consulted unavailable models", err)
	}
	if !q.Pricing.RenderOnly() || q.Pricing.MaxCredits != 0 || q.Pricing.ObservationCalls != 0 || q.Pricing.ReusedChunks != 3 {
		t.Fatal("wrong remaining work quote")
	}
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal("continuation failed", err)
	}
	// The third probe was the render's own read of the original; the render is
	// its own job now, so the continuation probes nothing again (CLIP-151).
	if h.planner.observe != 3 || h.planner.plans != 1 || h.media.probes != 2 || len(h.media.prepared) != 3 || h.renderer.calls != 0 || len(h.admitter.calls) != 1 {
		t.Fatalf("successful work repeated: observe=%d plan=%d probe=%d render=%d holds=%d", h.planner.observe, h.planner.plans, h.media.probes, h.renderer.calls, len(h.admitter.calls))
	}
	h.assertClean(t)
}

func TestRecoveryChangedInputsKeepAnalysisAndInvalidatePlan(t *testing.T) {
	for _, change := range []string{"facts", "target", "template", "instruction"} {
		t.Run(change, func(t *testing.T) {
			h := generationSetup(t)
			h.media.cleanupErr = errors.New("workspace cleanup failed")
			h.start(t)
			if err := h.run(t); err == nil {
				t.Fatal("missing failure")
			}
			h.media.cleanupErr = nil
			patch := clip.ProjectPatch{}
			switch change {
			case "facts":
				v := "save"
				patch.CTA = &v
			case "target":
				v := 45000
				patch.TargetDurationMS = &v
			case "instruction":
				v := "강조하고 싶은 점: 보습력"
				patch.Instruction = &v
			case "template":
				name := "Original template"
				if _, err := h.projects.UpdateTemplate(t.Context(), "alice", h.template.ID, clip.TemplatePatch{Name: &name}); err != nil {
					t.Fatal(err)
				}
				template, _ := create(t, h.projects)
				patch.VideoTemplateID = &template.ID
			}
			if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, patch); err != nil {
				t.Fatal(err)
			}
			q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
			if err != nil {
				t.Fatal(err)
			}
			// A changed input invalidates the whole assembly, so both writing
			// calls are priced again.
			if q.Pricing.RenderOnly() || q.Pricing.ObservationCalls != 0 || q.Pricing.ReusedChunks != 3 || q.Pricing.PlanCalls() != 2 {
				t.Fatal("changed input reused an incompatible plan")
			}
			h.renderer.fail = nil
			h.start(t)
			if err := h.run(t); err != nil {
				t.Fatal(err)
			}
			// Two probes, not three: the third was the render reading its
			// original back, and the render is its own job now (CLIP-151).
			if h.planner.observe != 3 || h.planner.plans != 2 || h.media.probes != 2 {
				t.Fatalf("changed template/facts repeated analysis observe=%d plans=%d probes=%d", h.planner.observe, h.planner.plans, h.media.probes)
			}
		})
	}
}

func TestRecoveryQuoteRejectsChangedDurableEvidence(t *testing.T) {
	h := generationSetup(t)
	h.renderer.fail = clip.ErrInvalidMedia
	h.start(t)
	_ = h.run(t)
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Writer.Exec("UPDATE clip_recovery_states SET state_json=json_set(state_json,'$.Contract','changed') WHERE project_id=?", h.project.ID); err != nil {
		t.Fatal(err)
	}
	_, err = h.service.Start(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", clip.QuoteApproval{CancellationPolicyVersion: clip.CancellationPolicyVersion, QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits})
	if !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal("stale recovery was admitted", err)
	}
}

func TestRecoveryDoesNotResumeACandidateThatNeverPassedLayout(t *testing.T) {
	h := generationSetup(t)
	h.renderer.fail = clip.ErrInvalidMedia
	h.start(t)
	_ = h.run(t)
	if _, err := h.db.Writer.Exec("UPDATE clip_recovery_states SET state_json=json_set(state_json,'$.PlanReady',json('false')) WHERE project_id=?", h.project.ID); err != nil {
		t.Fatal(err)
	}
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || q.Pricing.RenderOnly() || q.Pricing.ReusedChunks != 3 || q.Pricing.ObservationCalls != 0 || q.Pricing.PlanCalls() != 1 {
		t.Fatal("unvalidated candidate was reused as render-ready", err)
	}
}

// The candidate plan's digest is SEMANTIC. A plan written under a different
// observation or assembly contract is no longer the plan for this input, so it
// is discarded and rewritten — while the observations it was written from stay
// reusable and unpaid (CLIP-93).
func TestAnIncompatibleCandidatePlanIsRewrittenWithoutRepeatingAnalysis(t *testing.T) {
	h := generationSetup(t)
	// Interrupted after the plan is durable, which is where a continuation
	// picks a candidate up now that the render is its own job (CLIP-151).
	h.media.cleanupErr = errors.New("workspace cleanup failed")
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected attempt failure")
	}
	h.media.cleanupErr = nil
	if _, err := h.db.Writer.Exec("UPDATE clip_recovery_states SET state_json=json_set(state_json,'$.PlanDigest','written-under-another-contract') WHERE project_id=?", h.project.ID); err != nil {
		t.Fatal(err)
	}
	q, err := h.service.Quote(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if q.Pricing.RenderOnly() || q.Pricing.ReusedChunks != 3 || q.Pricing.ObservationCalls != 0 || q.Pricing.PlanCalls() != 2 {
		t.Fatalf("an incompatible candidate was reused or its observations repaid: %+v", q.Pricing)
	}
}

func TestRecoveryPartialAnalysisRepeatsOnlyTheMissingChunks(t *testing.T) {
	h := generationSetup(t)
	h.planner.observeErr = llm.ErrBadOutput
	h.planner.failObserveAt = 2
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("missing failed second chunk")
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil || len(state.Chunks) != 1 || state.Plan != "" {
		t.Fatal("lost first completed chunk", err)
	}
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || q.Pricing.ReusedChunks != 1 || q.Pricing.ObservationCalls != 2 || q.Pricing.RenderOnly() {
		t.Fatal("wrong partial continuation quote", err)
	}
	h.planner.observeErr = nil
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.observe != 4 || h.planner.plans != 1 || len(h.media.prepared) != 5 {
		t.Fatalf("completed chunk was repeated: observe=%d proxies=%d", h.planner.observe, len(h.media.prepared))
	}
}

func TestRecoveryIsRemovedWhenOriginalsLoseTheirProjectLifecycle(t *testing.T) {
	for _, column := range []string{"deleting", "finalized_at"} {
		t.Run(column, func(t *testing.T) {
			h := generationSetup(t)
			h.renderer.fail = clip.ErrInvalidMedia
			h.start(t)
			_ = h.run(t)
			statement := "UPDATE clip_projects SET deleting=1 WHERE id=?"
			if column == "finalized_at" {
				statement = "UPDATE clip_projects SET finalized_at='2026-09-14T00:00:00Z', finalized_plan_revision=1, edit_plan_revision=1, rendered_plan_revision=1, finalized_result_key='result', result_key='result', result_id='retained', source_access_revoked_at='2026-09-14T00:00:00Z' WHERE id=?"
			}
			if _, err := h.db.Writer.Exec(statement, h.project.ID); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := h.db.Reader.QueryRow("SELECT count(*) FROM clip_recovery_states WHERE project_id=?", h.project.ID).Scan(&count); err != nil || count != 0 {
				t.Fatal("recovery survived finalization/deletion", err)
			}
		})
	}
}
