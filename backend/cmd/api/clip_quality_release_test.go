package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/config"
)

// This extends the production-binary release harness; localhost serves recorded
// provider responses, private synthetic originals and real rendered MP4 bytes.
func TestClipQualityLifecycle(t *testing.T) {
	if os.Getenv("CLIP_RELEASE_SMOKE") != "1" {
		t.Skip("isolated production release image")
	}
	for _, boundary := range []struct {
		stage string
		done  int
		mode  string
	}{
		{"prepare", 0, "success"}, {"analyze", 0, "success"}, {"analyze", 1, "success"},
		{"plan", 0, "success"}, {"render", 0, "success"}, {"save", 0, "success"}, {"plan", 0, "overage"},
		{"provider-failure", 0, "unknown usage"},
	} {
		t.Run(fmt.Sprintf("cancel-%s-%d-%s", boundary.stage, boundary.done, boundary.mode), func(t *testing.T) {
			var clock atomic.Int64
			now := func() time.Time {
				if n := clock.Load(); n != 0 {
					return time.Unix(0, n)
				}
				return time.Now()
			}
			h := newReleaseHarness(t, boundary.mode, false, now)
			reached, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			h.generationHandler = func(ctx context.Context, j job.Job, p job.Progress) error {
				err := h.service.Run(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, func(stage string, done, total int) {
					p(stage, done, total)
					if stage == boundary.stage && done == boundary.done {
						once.Do(func() { close(reached); <-release })
					}
				})
				if boundary.stage == "provider-failure" {
					once.Do(func() { close(reached); <-release })
				}
				return err
			}
			id := h.qualityStart()
			stop := h.qualityWorker()
			defer stop()
			// Always unblock before stopping the worker if an assertion fails.
			defer close(release)
			select {
			case <-reached:
			case <-time.After(10 * time.Minute):
				t.Fatal("boundary not reached")
			}
			if boundary.stage == "prepare" {
				clock.Store(time.Now().Add(25 * time.Hour).UnixNano())
				if err := h.sources.Sweep(t.Context()); err != nil {
					t.Fatal(err)
				}
				keys, _ := h.objects.ListSourceKeys(t.Context())
				if len(keys) != len(h.sourceKeys) {
					t.Fatal("active job lost bound originals")
				}
				clock.Store(0)
			}
			for range 2 {
				response, err := h.client.CancelClipJob(t.Context(), releaseRequest(h, &v1.CancelClipJobRequest{ProjectId: h.project, JobId: id}))
				if err != nil || !response.Msg.Accepted {
					t.Fatal("durable cancellation", err)
				}
			}
			// The handler is still alive: accounting cannot settle early.
			p := h.qualityRead()
			if p.Accounting != nil && p.Accounting.Settled {
				t.Fatal("settled before worker exit")
			}
			// Signal without a second close in deferred cleanup.
			release <- struct{}{}
			p = h.qualityWait(id, "cancelled")
			stop()
			a := p.Accounting
			r, c := a.GetReservedCredits(), a.GetConfirmedChargeCredits()
			// QUOTA-51: after reservation even zero/unknown usage pays half
			// of that job's unused reservation; pre-reservation R is zero.
			fee := (r - c + 1) / 2
			if a.GetCancellationFeeCredits() != fee || a.GetFinalChargeCredits() != c+fee || a.GetRefundCredits() != r-c-fee || h.balance() != h.before-int(c+fee) {
				t.Fatalf("reservation settlement mismatch: %v", a)
			}
			if boundary.mode == "overage" && (c != r || fee != 0) {
				t.Fatal("over-ceiling cost escaped reservation", a)
			}
			if boundary.mode == "unknown usage" {
				var source string
				if err := h.d.Reader.QueryRow("SELECT cost_source FROM usage_events WHERE job_id=?", id).Scan(&source); err != nil || source != "unavailable" || c != 0 {
					t.Fatal("unknown supplier cost invented", source, c, err)
				}
			}
			calls, balance := h.provider.posts.Load(), h.balance()
			// A fresh queue reads only durable state and never replays paid work.
			recovery := job.New(jobstore.New(h.d.Writer, h.d.Reader), time.Millisecond)
			recovery.Admit(h.admission)
			for range 2 {
				if _, err := recovery.SweepRunning(t.Context()); err != nil {
					t.Fatal(err)
				}
				if _, err := recovery.SweepOpenHolds(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			if h.balance() != balance || h.provider.posts.Load() != calls {
				t.Fatal("restart charged or replayed")
			}
			h.qualityAvailable()
			// Reload the project and accept a NEW quote using the SAME retained batch.
			next := h.qualityStart()
			if next == id {
				t.Fatal("new quote reused old job")
			}
			if _, err := h.client.CancelClipJob(t.Context(), releaseRequest(h, &v1.CancelClipJobRequest{ProjectId: h.project, JobId: next})); err != nil {
				t.Fatal(err)
			}
			h.qualityWait(next, "cancelled")
			if h.provider.posts.Load() != calls || h.balance() != balance {
				t.Fatal("queued retry charged")
			}
			h.qualityAvailable()
			t.Logf("stage=%s calls=%d R=%d C=%d fee=%d refund=%d; retained batch reused after reload", boundary.stage, calls, r, c, fee, a.GetRefundCredits())
		})
	}
	t.Run("edit-rerender-failure-cancel-expire-finalize", func(t *testing.T) {
		var clock atomic.Int64
		now := func() time.Time {
			if n := clock.Load(); n != 0 {
				return time.Unix(0, n)
			}
			return time.Now()
		}
		h := newReleaseHarness(t, "success", false, now)
		template, err := h.projects.CreateTemplate(t.Context(), "release-user", clip.Recipe{Name: "quality-owned-caption", CompositionBody: `<clip version="1" styles="clean"><repeat for="scenes"><scene id="footage"><text id="caption" kind="fixed" role="caption" position="bottom" basis="cut">직접 남긴 기록</text></scene></repeat></clip>`})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = h.projects.UpdateProject(t.Context(), "release-user", h.project, clip.ProjectPatch{VideoTemplateID: &template.ID, CompositionInputs: &clip.CompositionInputs{}}); err != nil {
			t.Fatal(err)
		}
		h.exercise("success")
		h.qualityAvailable()
		p := h.qualityRead()
		h.qualityDownload(p)
		originalID := p.Result.Id
		draft := p.Editing.Plan
		authored := "직접 고친 문장"
		changed := false
		for _, e := range draft.Elements {
			if e.Role == "caption" {
				e.Text = authored
				changed = true
				break
			}
		}
		if !changed {
			for _, c := range draft.Cuts {
				if len(c.Copies) > 0 {
					c.Copies[0].Text = authored
					changed = true
					break
				}
			}
		}
		if !changed {
			t.Fatal("fixture has no caption")
		}
		saved, err := h.client.SaveClipEditPlan(t.Context(), releaseRequest(h, &v1.SaveClipEditPlanRequest{ProjectId: h.project, ExpectedRevision: p.EditPlanRevision, Plan: draft}))
		if err != nil {
			t.Fatal(err)
		}
		if saved.Msg.Project.Result.Id != originalID || saved.Msg.Project.GetCanFinalize() {
			t.Fatal("unsaved render identity")
		}
		if _, err = h.client.SaveClipEditPlan(t.Context(), releaseRequest(h, &v1.SaveClipEditPlanRequest{ProjectId: h.project, ExpectedRevision: p.EditPlanRevision, Plan: draft})); connect.CodeOf(err) != connect.CodeAborted {
			t.Fatal("stale edit accepted", err)
		}
		p = saved.Msg.Project
		calls, balance := h.provider.posts.Load(), h.balance()
		started, err := h.client.StartClipRender(t.Context(), releaseRequest(h, &v1.StartClipRenderRequest{ProjectId: h.project, ExpectedRevision: p.EditPlanRevision, BatchId: h.batch}))
		if err != nil {
			t.Fatal(err)
		}
		stop := h.qualityWorker()
		p = h.qualityWait(started.Msg.JobId, "done")
		stop()
		// The project and latest-job projection are separate reads. Reload once
		// the worker has exited before comparing durable result identities.
		p = h.qualityRead()
		if p.Result.Id == originalID || h.provider.posts.Load() != calls || h.balance() != balance {
			t.Fatalf("manual rerender: result=%s previous=%s calls=%d/%d balance=%d/%d", p.Result.Id, originalID, h.provider.posts.Load(), calls, h.balance(), balance)
		}
		h.qualityDownload(p)
		approvedID := p.Result.Id
		h.renderHandler = func(context.Context, job.Job, job.Progress) error { return errors.New("offline renderer unavailable") }
		started, err = h.client.StartClipRender(t.Context(), releaseRequest(h, &v1.StartClipRenderRequest{ProjectId: h.project, ExpectedRevision: p.EditPlanRevision, BatchId: h.batch}))
		if err != nil {
			t.Fatal(err)
		}
		stop = h.qualityWorker()
		p = h.qualityWait(started.Msg.JobId, "failed")
		stop()
		h.qualityAvailable()
		started, err = h.client.StartClipRender(t.Context(), releaseRequest(h, &v1.StartClipRenderRequest{ProjectId: h.project, ExpectedRevision: p.EditPlanRevision, BatchId: h.batch}))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = h.client.CancelClipJob(t.Context(), releaseRequest(h, &v1.CancelClipJobRequest{ProjectId: h.project, JobId: started.Msg.JobId})); err != nil {
			t.Fatal(err)
		}
		p = h.qualityWait(started.Msg.JobId, "cancelled")
		if p.Result.Id != approvedID || !p.GetCanFinalize() || h.provider.posts.Load() != calls || h.balance() != balance {
			t.Fatal("failed/cancelled rerender changed approved output or debit")
		}
		found := false
		for _, e := range p.Editing.Plan.Elements {
			found = found || e.Text == authored
		}
		for _, c := range p.Editing.Plan.Cuts {
			for _, copy := range c.Copies {
				found = found || copy.Text == authored
			}
		}
		if !found {
			t.Fatal("owner text lost")
		}
		if _, err = h.projects.DeleteTemplate(t.Context(), "release-user", template.ID); err != nil {
			t.Fatal(err)
		}
		if after := h.qualityRead(); after.Result.Id != approvedID || after.EditPlanRevision != p.EditPlanRevision || after.Editing == nil {
			t.Fatal("deleting reusable template changed frozen project")
		}
		h.qualityAvailable()
		batches, err := h.sources.GetSources(t.Context(), "release-user", h.project)
		if err != nil {
			t.Fatal(err)
		}
		src := batches[0].Sources[0]
		clock.Store(src.ExpiresAt.Add(-time.Nanosecond).UnixNano())
		if _, err = h.sources.AvailableBatch(t.Context(), "release-user", h.batch); err != nil {
			t.Fatal("expired before exact deadline", err)
		}
		clock.Store(src.ExpiresAt.UnixNano())
		if _, err = h.sources.AvailableBatch(t.Context(), "release-user", h.batch); !errors.Is(err, clip.ErrSourceState) {
			t.Fatal("expiry depends on sweep", err)
		}
		if _, err = h.sources.Playback(t.Context(), "release-user", h.project, src.ID, src.Fingerprint); !errors.Is(err, clip.ErrSourceExpired) {
			t.Fatal("expired playback", err)
		}
		// Physical deletion failure cannot undo confirmation. A new service instance
		// retries durable cleanup and retains the same downloaded result.
		h.objects.mu.Lock()
		h.objects.failDelete = true
		h.objects.mu.Unlock()
		if _, err = h.client.FinalizeClipProject(t.Context(), releaseRequest(h, &v1.FinalizeClipProjectRequest{ProjectId: h.project, ExpectedRevision: p.EditPlanRevision, ExpectedResultId: approvedID})); err != nil {
			t.Fatal(err)
		}
		p = h.qualityRead()
		if p.GetCanEdit() || p.Editing != nil || p.FinalizedResultId != approvedID {
			t.Fatal("confirmation not durable")
		}
		keys, _ := h.objects.ListSourceKeys(t.Context())
		if len(keys) != len(h.sourceKeys) {
			t.Fatal("cleanup failure fixture did not retain physical objects")
		}
		h.objects.mu.Lock()
		h.objects.failDelete = false
		h.objects.mu.Unlock()
		restarted := clip.NewSourceService(clipstore.New(h.d.Writer, h.d.Reader), h.objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), now)
		if err = restarted.Sweep(t.Context()); err != nil {
			t.Fatal(err)
		}
		keys, _ = h.objects.ListSourceKeys(t.Context())
		if len(keys) != 0 {
			t.Fatal("durable cleanup not retried", keys)
		}
		h.qualityDownload(h.qualityRead())
		if h.provider.posts.Load() != calls || h.balance() != balance {
			t.Fatal("lifecycle generated extra AI or debit")
		}
	})
}

func (h *releaseHarness) qualityRead() *v1.ClipProject {
	h.t.Helper()
	r, e := h.client.GetClipProject(h.t.Context(), releaseRequest(h, &v1.GetClipProjectRequest{Id: h.project}))
	if e != nil {
		h.t.Fatal(e)
	}
	return r.Msg.Project
}
func (h *releaseHarness) qualityStart() string {
	h.t.Helper()
	m := &v1.ModelRef{ProviderId: "fixture", ModelId: releaseModel}
	q, e := h.client.QuoteClipGeneration(h.t.Context(), releaseRequest(h, &v1.QuoteClipGenerationRequest{ProjectId: h.project, BatchId: h.batch, ObserveModel: m, WriteModel: m}))
	if e != nil {
		h.t.Fatal(e)
	}
	max := q.Msg.MaxCredits
	r, e := h.client.StartClipGeneration(h.t.Context(), releaseRequest(h, &v1.StartClipGenerationRequest{ProjectId: h.project, BatchId: h.batch, ObserveModel: m, WriteModel: m, QuoteId: q.Msg.QuoteId, ApprovedMaxCredits: &max, CancellationPolicyVersion: q.Msg.GetCancellationPolicy().GetVersion()}))
	if e != nil {
		h.t.Fatal(e)
	}
	return r.Msg.JobId
}
func (h *releaseHarness) qualityWorker() func() {
	ctx, cancel := context.WithCancel(h.t.Context())
	done := make(chan struct{})
	go func() { defer close(done); h.queue.Run(ctx) }()
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); <-done }) }
	h.t.Cleanup(stop)
	return stop
}
func (h *releaseHarness) qualityWait(id, status string) *v1.ClipProject {
	h.t.Helper()
	deadline := time.NewTimer(10 * time.Minute)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		p := h.qualityRead()
		if p.LatestJob != nil && p.LatestJob.Id == id && job.Terminal(p.LatestJob.Status) {
			if p.LatestJob.Status != status {
				h.t.Fatalf("status=%s want=%s failure=%v", p.LatestJob.Status, status, p.LatestJob.Failure)
			}
			if p.Accounting == nil || p.Accounting.Settled || p.Accounting.Status == "not_reserved" {
				return p
			}
		}
		select {
		case <-deadline.C:
			h.t.Fatal("lifecycle job timeout")
		case <-tick.C:
		}
	}
}
func (h *releaseHarness) qualityAvailable() {
	h.t.Helper()
	b, e := h.sources.AvailableBatch(h.t.Context(), "release-user", h.batch)
	if e != nil || b.ID != h.batch {
		h.t.Fatal("retained source reuse", e)
	}
	keys, _ := h.objects.ListSourceKeys(h.t.Context())
	if len(keys) != len(h.sourceKeys) {
		h.t.Fatal("original inventory changed")
	}
}
func (h *releaseHarness) qualityDownload(p *v1.ClipProject) {
	h.t.Helper()
	r, e := http.Get(p.Result.DownloadUrl)
	if e != nil {
		h.t.Fatal(e)
	}
	defer r.Body.Close()
	n, e := io.Copy(io.Discard, io.LimitReader(r.Body, p.Result.Bytes+1))
	if e != nil || r.StatusCode != 200 || n != p.Result.Bytes {
		h.t.Fatal("download failed", e)
	}
}
