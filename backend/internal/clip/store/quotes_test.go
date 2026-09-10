package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

func quote(t *testing.T, h *generationHarness) clip.GenerationQuote {
	t.Helper()
	q, err := h.service.Quote(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func accept(h *generationHarness, q clip.GenerationQuote) (string, error) {
	return h.service.Start(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", clip.QuoteApproval{QuoteID: q.ID, MaxCredits: &q.Pricing.MaxCredits})
}
func assertNoQuoteWork(t *testing.T, h *generationHarness) {
	t.Helper()
	if len(h.admitter.calls) != 0 || h.media.probes != 0 || h.planner.observe != 0 || h.planner.plans != 0 || len(h.objects.downloads) != 0 {
		t.Fatal("quote/refusal did paid or media work")
	}
}

func TestQuoteIsOwnerScopedFreeAndRefreshInvalidatesPreviousApproval(t *testing.T) {
	h := generationSetup(t)
	q := quote(t, h)
	if q.Pricing.MaxCredits != 18 || q.Pricing.ObservationCalls != 3 || time.Until(q.ExpiresAt) > 5*time.Minute {
		t.Fatal(q)
	}
	if _, err := h.store.GetQuote(context.Background(), "bob", q.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := h.service.Start(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w"); !errors.Is(err, clip.ErrQuoteRequired) {
		t.Fatal(err)
	}
	for _, max := range []int{0, q.Pricing.MaxCredits - 1, q.Pricing.MaxCredits + 1} {
		if _, err := h.service.Start(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", clip.QuoteApproval{QuoteID: q.ID, MaxCredits: &max}); !errors.Is(err, clip.ErrQuoteChanged) {
			t.Fatal(max, err)
		}
	}
	q2 := quote(t, h)
	if q2.ID == q.ID {
		t.Fatal("refresh reused id")
	}
	if _, err := accept(h, q); !errors.Is(err, clip.ErrQuoteChanged) {
		t.Fatal(err)
	}
	assertNoQuoteWork(t, h)
	if _, err := accept(h, q2); err != nil {
		t.Fatal(err)
	}
}

func TestChangedQuoteInputsAndPricesRequireNewApproval(t *testing.T) {
	for _, change := range []string{"recipe", "answer", "duration", "title", "rate", "budget", "source"} {
		t.Run(change, func(t *testing.T) {
			h := generationSetup(t)
			pricing := &quotePricing{}
			h.service.WithCredits(pricing, nil)
			q := quote(t, h)
			ctx := context.Background()
			var err error
			switch change {
			case "recipe":
				value := "different"
				_, err = h.projects.UpdateTemplate(ctx, "alice", h.template.ID, clip.TemplatePatch{CutGuidance: &value})
			case "answer":
				_, err = h.projects.UpdateProject(ctx, "alice", h.project.ID, clip.ProjectPatch{Answers: []clip.Answer{{Label: h.project.Answers[0].Label, Text: "changed"}}})
			case "title":
				value := "different"
				_, err = h.projects.UpdateProject(ctx, "alice", h.project.ID, clip.ProjectPatch{Title: &value})
			case "duration":
				value := 15000
				_, err = h.projects.UpdateProject(ctx, "alice", h.project.ID, clip.ProjectPatch{TargetDurationMS: &value})
			case "rate":
				pricing.inputRate = "0.2"
			case "budget":
				pricing.budgetDelta = 1
			case "source":
				_, err = h.db.Writer.Exec("UPDATE clip_source_leases SET fingerprint=? WHERE id=?", "changed", h.batch.Sources[0].ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = accept(h, q); !errors.Is(err, clip.ErrQuoteChanged) {
				t.Fatal(err)
			}
			assertNoQuoteWork(t, h)
		})
	}
}

func TestExpiredQuoteRefusesBeforeEnqueueButAcceptedSnapshotSurvivesCleanup(t *testing.T) {
	h := generationSetup(t)
	q := quote(t, h)
	if _, err := h.db.Writer.Exec("UPDATE clip_generation_quotes SET expires_at=? WHERE id=?", time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano), q.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := accept(h, q); !errors.Is(err, clip.ErrQuoteExpired) {
		t.Fatal(err)
	}
	assertNoQuoteWork(t, h)
	q = quote(t, h)
	id, err := accept(h, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Writer.Exec("UPDATE clip_generation_quotes SET expires_at=? WHERE id=?", time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano), q.ID); err != nil {
		t.Fatal(err)
	}
	if retry, err := accept(h, q); err != nil || retry != id {
		t.Fatal(retry, err)
	}
	h.planner.id = id
	if err = h.run(t); err != nil {
		t.Fatal(err)
	}
	h.assertClean(t)
	if retry, err := accept(h, q); err != nil || retry != id {
		t.Fatal("lost durable idempotency", retry, err)
	}
}

func TestConcurrentQuoteAcceptanceHasOneExecutableJob(t *testing.T) {
	h := generationSetup(t)
	q := quote(t, h)
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	errs := make(chan error, 12)
	for range 12 {
		wg.Go(func() { id, err := accept(h, q); ids <- id; errs <- err })
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, clip.ErrBusy) && !errors.Is(err, job.ErrActiveConflict) && !errors.Is(err, clip.ErrSourceState) {
			t.Fatal(err)
		}
	}
	winner := ""
	for id := range ids {
		if id != "" {
			if winner != "" && winner != id {
				t.Fatal("duplicate jobs")
			}
			winner = id
		}
	}
	if winner == "" {
		t.Fatal("nothing accepted")
	}
	var jobs, ready int
	if err := h.db.Reader.QueryRow("SELECT COUNT(*),SUM(dispatch_ready) FROM generation_jobs WHERE clip_project_id=?", h.project.ID).Scan(&jobs, &ready); err != nil || jobs != 1 || ready != 1 {
		t.Fatal(jobs, ready, err)
	}
	if retry, err := accept(h, q); err != nil || retry != winner {
		t.Fatal(retry, err)
	}
	assertNoQuoteWork(t, h)
}

func TestQuoteLinkRollbackDoesNotConsumeApproval(t *testing.T) {
	h := generationSetup(t)
	q := quote(t, h)
	if _, err := h.db.Writer.Exec("CREATE TRIGGER fail_link BEFORE UPDATE OF job_id ON clip_source_batches BEGIN SELECT RAISE(ABORT,'link failed'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := accept(h, q); err == nil {
		t.Fatal("link failure hidden")
	}
	stored, err := h.store.GetQuote(context.Background(), "alice", q.ID)
	if err != nil || stored.ConsumedJobID != "" {
		t.Fatal(stored, err)
	}
	if _, err = h.db.Writer.Exec("DROP TRIGGER fail_link"); err != nil {
		t.Fatal(err)
	}
	if _, err = accept(h, q); err != nil {
		t.Fatal("rolled back approval unusable", err)
	}
	assertNoQuoteWork(t, h)
}
