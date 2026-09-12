package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestCancelClipRPCDistinguishesAcceptedRequestFromTerminalCancellation(t *testing.T) {
	for _, running := range []bool{false, true} {
		t.Run(map[bool]string{false: "queued", true: "running"}[running], func(t *testing.T) {
			h := newCancellationHarness(t, nil)
			id := h.enqueue(t, job.KindGenerateClip)
			payload := `{"Version":4,"ProjectID":"clip","Batch":{"UserID":"alice"},"Approval":{"MaxCredits":5,"Pricing":{"CancellationPolicyVersion":1}}}`
			if _, err := h.db.Writer.Exec("UPDATE generation_jobs SET payload=? WHERE id=?", payload, id); err != nil {
				t.Fatal(err)
			}
			if running {
				if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
					t.Fatal(err)
				}
				if _, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			projects := clip.NewService(h.clips, config.ClipLimits())
			generation := clip.NewGenerationService(h.clips, projects, nil, nil, nil, nil, nil, clipJobs{h.queue}, clip.GenerationConfig{ReadTTL: time.Minute, CleanupTimeout: time.Second, OrphanMinAge: time.Hour}).WithCredits(nil, clipAccounting{ledger: h.ledger})
			handler := cliprpc.NewHandler(projects).WithGeneration(generation, h.queue)
			req := connect.NewRequest(&v1.CancelClipJobRequest{ProjectId: "clip", JobId: id})
			if _, err := handler.CancelClipJob(context.Background(), req); connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Fatal("unauthenticated cancellation", err)
			}
			if _, err := handler.CancelClipJob(auth.WithUser(t.Context(), "bob"), req); connect.CodeOf(err) != connect.CodeNotFound {
				t.Fatal("foreign cancellation", err)
			}
			response, err := handler.CancelClipJob(auth.WithUser(t.Context(), "alice"), req)
			if err != nil {
				t.Fatal(err)
			}
			j, a := response.Msg.Job, response.Msg.Accounting
			if !response.Msg.Accepted || j.GetCancelRequestedAt() == "" || j.CanCancel || j.CancellationPolicyVersion != 1 || a == nil || a.JobId != id || a.ReservedCredits == nil || *a.ReservedCredits != 0 {
				t.Fatal(response.Msg)
			}
			if running {
				if j.Status != job.StatusRunning || a.Settled || a.CancellationFeeCredits != nil {
					t.Fatal("pending request reported terminal charge", response.Msg)
				}
				if _, err := h.queue.SweepRunning(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			response, err = handler.CancelClipJob(auth.WithUser(t.Context(), "alice"), req)
			if err != nil {
				t.Fatal(err)
			}
			a = response.Msg.Accounting
			if response.Msg.Job.Status != job.StatusCancelled || !a.Settled || a.ConfirmedChargeCredits == nil || *a.ConfirmedChargeCredits != 0 || a.CancellationFeeCredits == nil || *a.CancellationFeeCredits != 0 || a.FinalChargeCredits == nil || *a.FinalChargeCredits != 0 {
				t.Fatal("free terminal accounting lost presence", response.Msg)
			}
			h.assertPreviousResult(t)
		})
	}
}
