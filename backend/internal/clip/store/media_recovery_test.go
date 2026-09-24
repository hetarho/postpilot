package store_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/clip/workerclient"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

type recoveryObjects struct {
	renderObjects
	writer  *sql.DB
	fail    bool
	failKey string
	listed  []clip.StoredObject
	deletes int
}

func (o *recoveryObjects) Delete(ctx context.Context, key string) error {
	// Object I/O must never monopolize the SQLite writer.
	check, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if _, err := o.writer.ExecContext(check, "SELECT 1"); err != nil {
		return err
	}
	if o.fail || key == o.failKey {
		return errors.New("simulated object deletion outage")
	}
	o.deletes++
	delete(o.data, key)
	return nil
}
func (o *recoveryObjects) ListMediaOutputs(context.Context) ([]clip.StoredObject, error) {
	return o.listed, nil
}

type recoveryHarness struct {
	*remoteRender
	now        time.Time
	bind       clipapp.Binder
	objects    *recoveryObjects
	reconciler *clipapp.MediaReconciler
	terminal   atomic.Int32
}

func newRecovery(t *testing.T) *recoveryHarness {
	t.Helper()
	g := remoteRenderSetup(t, false)
	h := &recoveryHarness{remoteRender: g, now: time.Now().UTC()}
	h.objects = &recoveryObjects{renderObjects: g.objects, writer: g.h.db.Writer}
	h.bind = func(tx *sql.Tx) clipapp.Ports {
		c := store.NewTx(tx)
		j := jobstore.NewTx(tx, jobKindsForTest())
		return clipapp.Ports{Clips: c, Jobs: j, Waits: j, Stages: c, Media: c, Publication: c, Recovery: c, Control: c}
	}
	h.restart()
	artifacts := clipapp.NewMediaArtifacts(g.h.db.Writer, h.bind, g.objects, g.h.cfg.Media, func() time.Time { return h.now })
	control := clipapp.NewMediaControl(g.h.db.Writer, h.bind, g.h.store)
	api := httptest.NewServer(cliprpc.NewMediaWorkerServer("", map[string]string{"render-one": "one", "render-two": "two"}, clipapp.NewMediaWorker(control, artifacts, func() time.Time { return h.now })).Handler)
	t.Cleanup(api.Close)
	g.clients = [2]*workerclient.Client{workerclient.New(api.URL, "render-one", "one"), workerclient.New(api.URL, "render-two", "two")}
	g.h.queue.OnTerminal(clip.JobKindRender, func(ctx context.Context, j job.Job, at time.Time) error {
		h.terminal.Add(1)
		return g.h.sources.ReleaseAttempt(ctx, j.UserID, j.ID, at)
	})
	if err := g.run(t, g.pick(t)); err != job.ErrYield {
		t.Fatal(err)
	}
	return h
}
func (h *recoveryHarness) restart() {
	h.reconciler = clipapp.NewMediaReconciler(h.h.db.Writer, h.bind, h.h.store, h.h.jobs, h.h.queue, h.objects, time.Minute, func() time.Time { return h.now })
}
func (h *recoveryHarness) reconcile(t *testing.T) {
	t.Helper()
	if err := h.reconciler.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
}
func (h *recoveryHarness) stage(t *testing.T) clip.MediaStage {
	t.Helper()
	s, err := h.h.store.MediaStageForJob(t.Context(), h.jobID, clip.MediaRender)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func (h *recoveryHarness) cancel(t *testing.T) {
	t.Helper()
	if _, err := h.h.queue.Cancel(t.Context(), "alice", job.Subject{Dimension: clip.JobSubject, ID: h.before.ID}, h.jobID); err != nil {
		t.Fatal(err)
	}
}
func (h *recoveryHarness) job(t *testing.T) job.Job {
	t.Helper()
	j, err := h.h.jobs.GetByID(t.Context(), h.jobID)
	if err != nil {
		t.Fatal(err)
	}
	return j
}
func (h *recoveryHarness) drain(t *testing.T) job.Job {
	t.Helper()
	h.h.queue.Register(clip.JobKindRender, func(ctx context.Context, j job.Job, p job.Progress) error {
		return h.h.service.RunRender(ctx, j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, p)
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); h.h.queue.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		j := h.job(t)
		if job.Terminal(j.Status) {
			return j
		}
		select {
		case <-deadline.C:
			t.Fatal("media parent did not finish")
		case <-ticker.C:
		}
	}
}
func (h *recoveryHarness) noClaim(t *testing.T) {
	t.Helper()
	p := mediaProfile()
	p.Operation = clip.MediaRender
	w, err := h.clients[0].Claim(t.Context(), p)
	if err != nil || w != nil {
		t.Fatal("unexpected claim", w, err)
	}
}

func TestMediaRecoveryWaitDeadlineAndTotalBudget(t *testing.T) {
	for _, mode := range []string{"offline", "total", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			h := newRecovery(t)
			s := h.stage(t)
			want := "CLIP_MEDIA_UNAVAILABLE"
			if mode == "offline" {
				h.now = s.QueueDeadlineAt.Add(time.Second)
			} else {
				c, w := h.claim(t)
				if mode == "total" {
					h.now = s.DeadlineAt.Add(time.Second)
					want = "CLIP_MEDIA_TIMEOUT"
				} else {
					if err := c.Fail(t.Context(), w.Credentials, clip.MediaFailureInvalidInput); err != nil {
						t.Fatal(err)
					}
					want = "CLIP_INVALID_MEDIA"
				}
			}
			h.restart()
			h.reconcile(t)
			h.noClaim(t)
			if _, err := h.h.queue.SweepRunning(t.Context()); err != nil {
				t.Fatal(err)
			}
			j := h.drain(t)
			if j.Status != job.StatusFailed || j.Failure == nil || j.Failure.Reason != want {
				t.Fatal(j.Status, j.Failure, want)
			}
			h.preserved(t)
			if h.h.planner.observe != 0 || h.h.planner.plans != 0 || len(h.h.admitter.calls) != 0 {
				t.Fatal("media recovery spent model work")
			}
		})
	}
}

func TestMediaRecoveryTransientBackoffAndExhaustion(t *testing.T) {
	h := newRecovery(t)
	initial := h.stage(t)
	for attempt := 1; attempt <= 3; attempt++ {
		c, w := h.claim(t)
		if err := c.Fail(t.Context(), w.Credentials, clip.MediaFailureWorkerLost); err != nil {
			t.Fatal(err)
		}
		s := h.stage(t)
		if err := c.Fail(t.Context(), w.Credentials, clip.MediaFailureWorkerLost); err != nil {
			t.Fatal("retry reply replay", err)
		}
		repeated := h.stage(t)
		if !s.RetryNotBefore.Equal(repeated.RetryNotBefore) || !s.DeadlineAt.Equal(initial.DeadlineAt) || s.AttemptCount != attempt {
			t.Fatal("budget reset", s)
		}
		h.noClaim(t)
		h.restart()
		h.reconcile(t)
		if attempt < 3 {
			want := clip.MediaRetryDelay(attempt)
			if s.RetryNotBefore.Sub(h.now) != want {
				t.Fatal(s.RetryNotBefore, h.now, want)
			}
			h.now = s.RetryNotBefore
		}
	}
	s := h.stage(t)
	if s.State != clip.MediaFailed || s.Failure != clip.MediaFailureAttemptsExhausted {
		t.Fatal(s)
	}
	j := h.drain(t)
	if j.Failure == nil || j.Failure.Reason != "CLIP_MEDIA_RETRY_EXHAUSTED" {
		t.Fatal(j)
	}
}

func TestMediaRecoveryProductLimitsKeepTheirOwnerReason(t *testing.T) {
	for failure, reason := range map[clip.MediaFailure]string{
		clip.MediaFailureWorkspaceLimit:   "CLIP_WORKSPACE_LIMIT",
		clip.MediaFailureInputTooLarge:    "CLIP_INPUT_TOO_LARGE",
		clip.MediaFailureAnalysisTooLarge: "CLIP_ANALYSIS_TOO_LARGE",
	} {
		t.Run(string(failure), func(t *testing.T) {
			h := newRecovery(t)
			c, w := h.claim(t)
			for range 2 {
				if err := c.Fail(t.Context(), w.Credentials, failure); err != nil {
					t.Fatal(err)
				}
			}
			h.restart()
			h.reconcile(t)
			h.noClaim(t)
			j := h.drain(t)
			if j.Failure == nil || j.Failure.Reason != reason || h.stage(t).AttemptCount != 1 {
				t.Fatal("product limit was retried or lost its reason", j.Failure)
			}
		})
	}
}

func TestMediaRecoveryLeaseReclaimRejectsLateUploadReport(t *testing.T) {
	h := newRecovery(t)
	c, w := h.claim(t)
	_, raw := h.upload(t, c, w)
	h.restart()
	h.reconcile(t)
	if n, err := h.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
		t.Fatal("valid lease failed at boot", n, err)
	}
	h.noClaim(t)
	s := h.stage(t)
	h.now = h.now.Add(s.Limits.LeaseTTL + time.Second)
	h.reconcile(t)
	next, newWork := h.claim(t)
	if newWork.Credentials.AttemptID == w.Credentials.AttemptID || newWork.Credentials.Token == w.Credentials.Token {
		t.Fatal("retry reused authority")
	}
	if err := c.Complete(t.Context(), w.Credentials, raw); err == nil {
		t.Fatal("expired result accepted")
	}
	_, accepted := h.upload(t, next, newWork)
	if err := next.Complete(t.Context(), newWork.Credentials, accepted); err != nil {
		t.Fatal(err)
	}
	h.restart()
	h.reconcile(t)
	if n, err := h.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	j := h.drain(t)
	if j.Status != job.StatusDone {
		t.Fatal(j)
	}
	h.reconcile(t)
	if err := h.reconciler.Cleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	var canonical int
	if err := h.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_artifacts WHERE canonical=1`).Scan(&canonical); err != nil || canonical != 1 {
		t.Fatal(canonical, err)
	}
}

func TestMediaRecoveryCancellationWaitsForStopOrExpiry(t *testing.T) {
	for _, mode := range []string{"ack", "expiry", "queued", "accepted"} {
		t.Run(mode, func(t *testing.T) {
			h := newRecovery(t)
			var c *workerclient.Client
			var w *clip.MediaWork
			var raw string
			if mode != "queued" {
				c, w = h.claim(t)
				_, raw = h.upload(t, c, w)
				if mode == "accepted" {
					if err := c.Complete(t.Context(), w.Credentials, raw); err != nil {
						t.Fatal(err)
					}
				}
			}
			h.cancel(t)
			h.cancel(t)
			// Renewal is denied immediately, before the five-second reconciler.
			if w != nil && mode != "accepted" {
				if _, err := c.Renew(t.Context(), w.Credentials, 0); !errors.Is(err, clip.ErrMediaCancelled) {
					t.Fatal("cancel still renewed", err)
				}
				if err := c.Complete(t.Context(), w.Credentials, raw); err == nil {
					t.Fatal("cancel accepted a result")
				}
			}
			h.reconcile(t)
			h.noClaim(t)
			if mode == "ack" || mode == "expiry" {
				if h.job(t).Status != job.StatusRunning || h.terminal.Load() != 0 {
					t.Fatal("cancellation finished before media stopped")
				}
				h.restart()
				if _, err := h.h.queue.SweepRunning(t.Context()); err != nil {
					t.Fatal(err)
				}
				if h.job(t).Status != job.StatusRunning {
					t.Fatal("boot acknowledged live media")
				}
				if mode == "ack" {
					if err := c.Fail(t.Context(), w.Credentials, clip.MediaFailureCancelled); err != nil {
						t.Fatal(err)
					}
				} else {
					h.now = h.now.Add(h.stage(t).Limits.LeaseTTL + time.Second)
				}
				h.reconcile(t)
			}
			if h.job(t).Status != job.StatusCancelled || h.terminal.Load() != 1 {
				t.Fatal(h.job(t), h.terminal.Load())
			}
			h.cancel(t)
			h.reconcile(t)
			if h.terminal.Load() != 1 {
				t.Fatal("terminal observer repeated")
			}
			h.preserved(t)
		})
	}
}

func TestMediaRecoveryDelayedUploadAndFailedDeletionSurviveRestart(t *testing.T) {
	h := newRecovery(t)
	c, w := h.claim(t)
	h.upload(t, c, w)
	rows, err := h.h.store.MediaArtifacts(t.Context(), w.Credentials.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	key := rows[0].ObjectKey
	h.cancel(t)
	h.reconcile(t)
	if err = h.reconciler.Cleanup(t.Context()); err != nil || h.objects.deletes != 0 {
		t.Fatal("live PUT was swept", err)
	}
	// The cancelled PUT may finish after its reservation was retired.
	h.objects.data[key] = []byte("late upload")
	h.now = rows[0].PutExpiresAt.Add(time.Minute + time.Second)
	h.reconcile(t)
	h.objects.fail = true
	if err = h.reconciler.Cleanup(t.Context()); err == nil {
		t.Fatal("lost failed delete")
	}
	var pending int
	if err = h.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_deletions WHERE object_key=?`, key).Scan(&pending); err != nil || pending != 1 {
		t.Fatal("forgot object identity", pending, err)
	}
	h.restart()
	h.objects.fail = false
	if err = h.reconciler.Cleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.data[key]; ok {
		t.Fatal("abandoned upload retained")
	}
	if err = h.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_deletions WHERE object_key=?`, key).Scan(&pending); err != nil || pending != 0 {
		t.Fatal(pending, err)
	}
}

func TestMediaRecoveryMissingChildFailsAndReleasesOnce(t *testing.T) {
	h := newRecovery(t)
	s := h.stage(t)
	if _, err := h.h.db.Writer.Exec(`DELETE FROM clip_media_stages WHERE id=?`, s.ID); err != nil {
		t.Fatal(err)
	}
	h.reconcile(t)
	h.restart()
	h.reconcile(t)
	j := h.job(t)
	if j.Status != job.StatusFailed || j.Failure == nil || j.Failure.Reason != "CLIP_MEDIA_UNAVAILABLE" || h.terminal.Load() != 1 {
		t.Fatal(j, h.terminal.Load())
	}
}

func TestMediaRecoveryOneFailedDeletionDoesNotBlockOtherArtifacts(t *testing.T) {
	h := newRecovery(t)
	first := clip.MediaAnalysisPrefix + "alice/abandoned/attempt/a.mp4"
	second := clip.MediaAnalysisPrefix + "alice/abandoned/attempt/b.mp4"
	for _, key := range []string{first, second} {
		h.objects.data[key] = []byte("orphan")
		if err := h.h.store.QueueOrphanMediaDeletion(t.Context(), key, h.now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	h.objects.failKey = first
	if err := h.reconciler.Cleanup(t.Context()); err == nil {
		t.Fatal("failed deletion was hidden")
	}
	if _, ok := h.objects.data[second]; ok {
		t.Fatal("one unavailable object blocked another deletion")
	}
	rows, err := h.h.store.DueMediaDeletions(t.Context(), h.now)
	if err != nil || len(rows) != 1 || rows[0].Key != first {
		t.Fatal("failed deletion identity was lost", rows, err)
	}
}

func TestMediaRecoveryMissingAttemptIsReclaimedWithinOriginalBudget(t *testing.T) {
	h := newRecovery(t)
	_, w := h.claim(t)
	before := h.stage(t)
	if _, err := h.h.db.Writer.Exec(`DELETE FROM clip_media_attempts WHERE id=?`, w.Credentials.AttemptID); err != nil {
		t.Fatal(err)
	}
	h.reconcile(t)
	_, next := h.claim(t)
	after := h.stage(t)
	if next.Credentials.AttemptID == w.Credentials.AttemptID || after.AttemptCount != 2 || !after.DeadlineAt.Equal(before.DeadlineAt) {
		t.Fatal("missing attempt reset durable budget", after)
	}
}

func TestMediaRecoveryOrphansPreserveCanonicalAndCascadeIntents(t *testing.T) {
	h := newRecovery(t)
	c, w := h.claim(t)
	_, raw := h.upload(t, c, w)
	if err := c.Complete(t.Context(), w.Credentials, raw); err != nil {
		t.Fatal(err)
	}
	if j := h.drain(t); j.Status != job.StatusDone {
		t.Fatal(j)
	}
	h.reconcile(t)
	p, err := h.h.store.GetProject(t.Context(), "alice", h.before.ID)
	if err != nil {
		t.Fatal(err)
	}
	key := p.Result.Key
	orphan := clip.MediaAnalysisPrefix + "alice/abandoned/attempt/output.mp4"
	h.objects.data[orphan] = []byte("orphan")
	h.objects.listed = []clip.StoredObject{{Key: key, Modified: h.now.Add(-time.Hour)}, {Key: orphan, Modified: h.now.Add(-time.Hour)}}
	if err = h.reconciler.SweepOrphans(t.Context()); err != nil {
		t.Fatal(err)
	}
	h.now = h.now.Add(12 * time.Minute)
	if err = h.reconciler.Cleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.data[key]; !ok {
		t.Fatal("canonical output swept")
	}
	if _, ok := h.objects.data[orphan]; ok {
		t.Fatal("orphan retained")
	}
	if err = h.h.projects.DeleteProject(t.Context(), "alice", h.before.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = h.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_deletions WHERE object_key=?`, key).Scan(&n); err != nil || n != 1 {
		t.Fatal("cascade lost canonical upload grant", n, err)
	}
	h.restart()
	if err = h.reconciler.Cleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.data[key]; ok {
		t.Fatal("deleted project retained worker output")
	}
}

func TestMediaRecoveryRevocationAndOriginalExpiryProtectActiveBinding(t *testing.T) {
	h := newRecovery(t)
	c, w := h.claim(t)
	// The original's public retention clock cannot delete a live attempt's file.
	if _, err := h.h.db.Writer.Exec(`UPDATE clip_source_leases SET retention_expires_at=? WHERE batch_id=?`, h.now.Add(-time.Hour).Format(time.RFC3339Nano), h.batch.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.h.sources.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	b, err := h.h.store.GetSourceBatch(t.Context(), "alice", h.batch.ID)
	if err != nil || len(b.Sources) == 0 || b.Sources[0].CleanupPending {
		t.Fatal("active original expired physically", b, err)
	}
	if err = h.h.sources.RevokeProject(t.Context(), "alice", h.before.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Renew(t.Context(), w.Credentials, 0); err == nil {
		t.Fatal("revoked source renewed")
	}
	h.reconcile(t)
	if h.stage(t).State != clip.MediaFailed {
		t.Fatal("revoked work was preserved")
	}
}

func TestMediaRecoveryKeepsAcceptedAnalysisUntilItsContinuationEnds(t *testing.T) {
	g := remoteGenerationSetup(t)
	if id := g.h.start(t); id == "" {
		t.Fatal("missing parent")
	}
	if err := g.runJob(t, g.pick(t)); err != job.ErrYield {
		t.Fatal(err)
	}
	auth, raw := g.preparedReceipt(t)
	if err := g.artifacts.Complete(t.Context(), auth, raw); err != nil {
		t.Fatal(err)
	}
	bind := func(tx *sql.Tx) clipapp.Ports {
		p := g.bind(tx)
		p.Recovery = store.NewTx(tx)
		return p
	}
	r := clipapp.NewMediaReconciler(g.h.db.Writer, bind, g.h.store, g.h.jobs, g.h.queue, &recoveryObjects{writer: g.h.db.Writer}, time.Minute, nil)
	if err := r.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	rows, err := g.h.store.MediaArtifacts(t.Context(), auth.AttemptID)
	if err != nil || len(rows) == 0 {
		t.Fatal("accepted analysis disappeared before consumption", err)
	}
	// An API crash after entering the paid continuation is deliberately not
	// replayed: its provider outcome is unknown. Its private copies can retire.
	claimed := g.pick(t)
	if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err = r.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	rows, err = g.h.store.MediaArtifacts(t.Context(), auth.AttemptID)
	if err != nil || len(rows) != 0 || g.holds(t) != 0 || g.h.planner.observe != 0 {
		t.Fatal("interrupted paid work was preserved or replayed", rows, err)
	}
	parent, err := g.h.jobs.GetByID(t.Context(), claimed.ID)
	if err != nil || parent.Status != job.StatusFailed {
		t.Fatal(parent, err)
	}
}
