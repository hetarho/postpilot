package store_test

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"github.com/postpilot/backend/internal/clip/workerclient"
	"github.com/postpilot/backend/internal/job"
)

func earlyNativeClient(t *testing.T, g *remoteRender) (*workerclient.Client, clip.MediaWorkerProfile) {
	t.Helper()
	h := g.h
	control := clipapp.NewMediaControl(h.db.Writer, capacityPorts(h), h.store, clip.DefaultRenderCapacity(clip.Environment{}))
	artifacts := clipapp.NewMediaArtifacts(h.db.Writer, capacityPorts(h), g.objects, h.cfg.Media, nil)
	api := httptest.NewServer(cliprpc.NewMediaWorkerServer("", map[string]string{"native-early": "early-fixture-token"}, clipapp.NewMediaWorker(control, artifacts, nil)).Handler)
	t.Cleanup(api.Close)
	client := workerclient.New(api.URL, "native-early", "early-fixture-token")
	profile := mediaProfile()
	profile.WorkerID, profile.Operation = "native-early", clip.MediaRender
	return client, profile
}

func TestNativeRenderCompletedBeforeFirstParkUsesDurableResume(t *testing.T) {
	g := remoteRenderSetup(t, false)
	h := g.h
	first := g.pick(t)
	client, profile := earlyNativeClient(t, g)

	// The production authorization layer permits a worker to finish the
	// admission-created stage while its parent is running but has not parked.
	work, err := client.Claim(t.Context(), profile)
	if err != nil || work == nil {
		t.Fatalf("early native claim: %v, %v", work, err)
	}
	_, receipt := g.upload(t, client, work)
	if err := client.Complete(t.Context(), work.Credentials, receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := h.jobs.Continuation(t.Context(), g.jobID); !errors.Is(err, job.ErrInvalidWait) {
		t.Fatal("fixture already had a continuation", err)
	}
	if err := g.run(t, first); !errors.Is(err, job.ErrYield) {
		t.Fatal("early accepted output must yield to durable resume", err)
	}
	wait, err := h.jobs.Continuation(t.Context(), g.jobID)
	if err != nil || wait.WaitKey != clipapp.MediaWaitKey(work.Credentials.StageID) || wait.Policy != job.ReplaySafe || wait.State != job.ContinuationReady {
		t.Fatal("accepted output has no ready owning continuation", wait, err)
	}
	g.preserved(t)
	var canonical int
	if err := h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_artifacts WHERE attempt_id=? AND canonical=1`, work.Credentials.AttemptID).Scan(&canonical); err != nil || canonical != 0 {
		t.Fatal("output published before its continuation was claimed", canonical, err)
	}

	resumed := g.pick(t)
	wait, err = h.jobs.Continuation(t.Context(), g.jobID)
	if err != nil || wait.State != job.ContinuationClaimed {
		t.Fatal("queue did not claim durable completion", wait, err)
	}
	if err := g.run(t, resumed); err != nil {
		t.Fatal(err)
	}
	if err := client.Complete(t.Context(), work.Credentials, receipt); err != nil {
		t.Fatal("lost completion reply could not replay", err)
	}
	completed, err := h.jobs.GetByID(t.Context(), g.jobID)
	if err != nil || completed.Status != job.StatusDone {
		t.Fatal("early output did not complete its parent", completed, err)
	}
	rows, err := h.store.MediaArtifacts(t.Context(), work.Credentials.AttemptID)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	project, err := h.store.GetProject(t.Context(), "alice", g.before.ID)
	if err != nil || project.Result.Key != rows[0].ObjectKey || project.EditPlan != g.before.EditPlan {
		t.Fatal("claimed output changed authored work or was not saved", project, err)
	}
	if err := h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_artifacts WHERE attempt_id=? AND canonical=1`, work.Credentials.AttemptID).Scan(&canonical); err != nil || canonical != 1 {
		t.Fatal("completion did not publish exactly once", canonical, err)
	}
	if next, err := client.Claim(t.Context(), profile); err != nil || next != nil {
		t.Fatal("accepted native stage was dispatched again", next, err)
	}
	if h.media.probes != 0 || h.renderer.calls != 0 || len(h.admitter.calls) != 0 {
		t.Fatal("resuming accepted output performed API media or AI work")
	}
}

func TestNativeRenderRestartBeforeFirstParkPreservesAdmittedWork(t *testing.T) {
	for _, phase := range []string{"queued", "leased", "accepted"} {
		t.Run(phase, func(t *testing.T) {
			g := remoteRenderSetup(t, false)
			g.pick(t)
			client, profile := earlyNativeClient(t, g)
			var work *clip.MediaWork
			var receipt string
			if phase != "queued" {
				var err error
				work, err = client.Claim(t.Context(), profile)
				if err != nil || work == nil {
					t.Fatal("early native claim", work, err)
				}
			}
			if phase == "accepted" {
				_, receipt = g.upload(t, client, work)
				if err := client.Complete(t.Context(), work.Credentials, receipt); err != nil {
					t.Fatal(err)
				}
			}
			before, err := g.h.store.MediaStageForJob(t.Context(), g.jobID, clip.MediaRender)
			if err != nil {
				t.Fatal(err)
			}
			objects := &recoveryObjects{renderObjects: g.objects, writer: g.h.db.Writer}
			recovery := clipapp.NewMediaReconciler(g.h.db.Writer, capacityPorts(g.h), g.h.store, g.h.jobs, g.h.queue, objects, time.Minute, nil)
			if err := recovery.ReconcileStartup(t.Context()); err != nil {
				t.Fatal(err)
			}
			if swept, err := g.h.queue.SweepRunning(t.Context()); err != nil || swept != 0 {
				t.Fatal("boot interrupted an admitted native render", swept, err)
			}
			wait, err := g.h.jobs.Continuation(t.Context(), g.jobID)
			expected := job.ContinuationWaiting
			if phase == "accepted" {
				expected = job.ContinuationReady
			}
			if err != nil || wait.WaitKey != clipapp.MediaWaitKey(before.ID) || wait.Policy != job.ReplaySafe || wait.State != expected {
				t.Fatal("boot did not restore the owning native continuation", wait, err)
			}
			after, err := g.h.store.MediaStageForJob(t.Context(), g.jobID, clip.MediaRender)
			if err != nil || after.ID != before.ID || after.State != before.State || after.InputDigest != before.InputDigest || !after.QueueDeadlineAt.Equal(before.QueueDeadlineAt) || !after.DeadlineAt.Equal(before.DeadlineAt) || after.AttemptCount != before.AttemptCount {
				t.Fatal("recovery recreated work or extended its finite allowance", before, after, err)
			}
			g.preserved(t)
			if phase == "queued" {
				work, err = client.Claim(t.Context(), profile)
				if err != nil || work == nil {
					t.Fatal("restored stage was not claimable", work, err)
				}
			}
			if phase != "accepted" {
				_, receipt = g.upload(t, client, work)
				if err := client.Complete(t.Context(), work.Credentials, receipt); err != nil {
					t.Fatal(err)
				}
			}
			if err := g.run(t, g.pick(t)); err != nil {
				t.Fatal("restored native render could not finish", err)
			}
			completed, err := g.h.jobs.GetByID(t.Context(), g.jobID)
			if err != nil || completed.Status != job.StatusDone {
				t.Fatal(completed, err)
			}
			if g.h.renderer.calls != 0 || len(g.h.admitter.calls) != 0 {
				t.Fatal("boot replayed media or paid work in the API")
			}
		})
	}
}

func TestNativeStartupRecoveryDrainsPagesAndPeriodicRecoveryStaysBounded(t *testing.T) {
	g := remoteRenderSetup(t, false)
	parent := g.pick(t)
	stage, err := g.h.store.MediaStageForJob(t.Context(), parent.ID, clip.MediaRender)
	if err != nil {
		t.Fatal(err)
	}
	for index := range 200 {
		id := fmt.Sprintf("000-native-terminal-%03d", index)
		if _, err := g.h.db.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at,payload)
SELECT ?,user_id,clip_project_id,kind,'failed',created_at,updated_at,payload FROM generation_jobs WHERE id=?`, id, parent.ID); err != nil {
			t.Fatal(err)
		}
		input := stage.MediaStageInput
		input.ID, input.ParentJobID = id, id
		if _, err := g.h.store.CreateMediaStage(t.Context(), input, time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	objects := &recoveryObjects{renderObjects: g.objects, writer: g.h.db.Writer}
	recovery := clipapp.NewMediaReconciler(g.h.db.Writer, capacityPorts(g.h), g.h.store, g.h.jobs, g.h.queue, objects, time.Minute, nil)
	if err := recovery.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	var reconciled int
	if err := g.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_stages WHERE id LIKE '000-native-terminal-%' AND reconciled_at IS NOT NULL`).Scan(&reconciled); err != nil || reconciled != 100 {
		t.Fatal("periodic recovery exceeded its bounded page", reconciled, err)
	}
	if _, err := g.h.jobs.Continuation(t.Context(), parent.ID); !errors.Is(err, job.ErrInvalidWait) {
		t.Fatal("periodic recovery parked a live handler", err)
	}
	if err := recovery.ReconcileStartup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if swept, err := g.h.queue.SweepRunning(t.Context()); err != nil || swept != 0 {
		t.Fatal("later native page was interrupted during boot", swept, err)
	}
	wait, err := g.h.jobs.Continuation(t.Context(), parent.ID)
	if err != nil || wait.State != job.ContinuationWaiting || wait.WaitKey != clipapp.MediaWaitKey(stage.ID) {
		t.Fatal("later page did not restore its native owner", wait, err)
	}
	if err := g.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_stages WHERE id LIKE '000-native-terminal-%' AND reconciled_at IS NOT NULL`).Scan(&reconciled); err != nil || reconciled != 200 {
		t.Fatal("startup did not finish all bounded pages", reconciled, err)
	}
	g.preserved(t)
}

func TestNativePeriodicRecoveryDoesNotParkAnExecutingParent(t *testing.T) {
	g := remoteRenderSetup(t, false)
	parent := g.pick(t)
	objects := &recoveryObjects{renderObjects: g.objects, writer: g.h.db.Writer}
	recovery := clipapp.NewMediaReconciler(g.h.db.Writer, capacityPorts(g.h), g.h.store, g.h.jobs, g.h.queue, objects, time.Minute, nil)
	if err := recovery.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := g.h.jobs.Continuation(t.Context(), parent.ID); !errors.Is(err, job.ErrInvalidWait) {
		t.Fatal("periodic pass created a competing live continuation", err)
	}
	if err := g.run(t, parent); !errors.Is(err, job.ErrYield) {
		t.Fatal("owning handler could not park its native work", err)
	}
}
