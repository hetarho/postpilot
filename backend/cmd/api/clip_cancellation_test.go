package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type cancellationHarness struct {
	db        *db.DB
	jobs      *jobstore.Store
	clips     *clipstore.Store
	queue     *job.Queue
	ledger    *usage.Service
	finisher  clipFinisher
	admission jobAdmission
}

func newCancellationHarness(t *testing.T, wrap func(*jobstore.Store) job.Store) *cancellationHarness {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(t.Context(), d.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, user := range []string{"alice", "bob"} {
		if _, err := d.Writer.Exec("INSERT INTO users(id,password_hash,plan,created_at) VALUES (?,'hash','free',?)", user, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,analysis_json,edit_plan_json,edit_plan_revision,rendered_plan_revision,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,created_at,updated_at) VALUES ('clip','alice','clip','square',15000,'[]','old-plan',1,1,'clip-results/alice/clip/old.mp4','video/mp4',10,15000,?,?,?)`, now, now, now); err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(authstore.New(d.Writer, d.Reader), time.Hour)
	ledger := usage.NewService(usagestore.New(d.Writer, d.Reader), emptyModels{}, 32768)
	ledger.SetAnchors(usageAnchors{auth: authSvc})
	js, cs := jobstore.New(d.Writer, d.Reader), clipstore.New(d.Writer, d.Reader)
	var queueStore job.Store = js
	if wrap != nil {
		queueStore = wrap(js)
	}
	q := job.New(queueStore, time.Millisecond)
	admission := jobAdmission{ledger: ledger, plans: authSvc}
	q.Admit(admission)
	q.GuardClips(clipGuard{writer: d.Writer, admission: admission})
	return &cancellationHarness{d, js, cs, q, ledger, clipFinisher{writer: d.Writer, clips: cs, jobs: js}, admission}
}

func (h *cancellationHarness) enqueue(t *testing.T, kind string) string {
	t.Helper()
	n := job.NewJob{UserID: "alice", ClipProjectID: "clip", Kind: kind}
	if kind == job.KindRenderClip {
		n.NonMetered = true
	} else {
		n.CancellationPolicyVersion = 1
		n.ObserveModel = "p/o"
		n.WriteModel = "p/w"
	}
	id, err := h.queue.Enqueue(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func cancellationReservation() job.ClipReservation {
	p := llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "p", ModelID: "o"}, Stage: "observe", CompletionTokens: 8192, InputUSDPerMillion: "0.15", OutputUSDPerMillion: "0", Pricing: llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionInlineStatic, Endpoint: "leaf", RequiredParameters: "max_tokens", PromptUSDPerMillion: "0.15", CompletionUSDPerMillion: "0", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}}
	w := p
	w.Ref.ModelID = "w"
	w.Stage = "write"
	w.CompletionTokens = 32768
	w.Pricing.Delivery = llm.ExecutionTextOnly
	return job.ClipReservation{CancellationPolicyVersion: 1, ApprovedMaxCredits: 5, Calls: []job.ClipCall{{Policy: p, Count: 1}, {Policy: w, Count: 1}}}
}
func (h *cancellationHarness) reserve(ctx context.Context, id string) (context.Context, error) {
	return h.queue.ReserveClip(ctx, "alice", id, []job.PlannedCall{{Ref: "p/o", Count: 1, CompletionTokens: 8192}, {Ref: "p/w", Count: 1, CompletionTokens: 32768}}, cancellationReservation())
}
func (h *cancellationHarness) candidate(id string, render bool) clip.AttemptResult {
	c := clip.AttemptResult{JobID: id, UserID: "alice", ProjectID: "clip", ExpectedRevision: 1, Analysis: "[]", EditPlan: "new-plan", Result: clip.Result{Key: "clip-results/alice/clip/new.mp4", ContentType: "video/mp4", Bytes: 20, DurationMS: 15000, CreatedAt: time.Now()}}
	if render {
		c.Analysis = ""
		c.EditPlan = ""
	}
	return c
}
func awaitCancellationSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for clip transition")
	}
}
func (h *cancellationHarness) run(t *testing.T, kind string, handler job.Handler) <-chan struct{} {
	t.Helper()
	terminal := make(chan struct{})
	var once sync.Once
	h.queue.Register(kind, handler)
	h.queue.OnTerminal(kind, func(ctx context.Context, j job.Job, at time.Time) error {
		persisted, err := h.jobs.GetByID(ctx, j.ID)
		if err != nil {
			return err
		}
		if !job.Terminal(persisted.Status) || persisted.FinishedAt == nil || !persisted.FinishedAt.Equal(at) {
			return errors.New("release preceded durable outcome")
		}
		once.Do(func() { close(terminal) })
		return nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	go func() { defer close(stopped); h.queue.Run(ctx) }()
	t.Cleanup(func() { cancel(); awaitCancellationSignal(t, stopped) })
	return terminal
}
func (h *cancellationHarness) assertPreviousResult(t *testing.T) {
	t.Helper()
	p, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil || p.Result == nil || p.Result.Key != "clip-results/alice/clip/old.mp4" || p.EditPlan != "old-plan" || p.EditPlanRevision != 1 {
		t.Fatal("previous result changed", p, err)
	}
}

func TestClipCancellationBeforeReservationAndManualRenderAreFree(t *testing.T) {
	for _, kind := range []string{job.KindGenerateClip, job.KindRenderClip} {
		t.Run(kind, func(t *testing.T) {
			h := newCancellationHarness(t, nil)
			id := h.enqueue(t, kind)
			entered, release := make(chan struct{}), make(chan struct{})
			terminal := h.run(t, kind, func(ctx context.Context, j job.Job, _ job.Progress) error {
				close(entered)
				<-release
				if kind == job.KindGenerateClip {
					_, err := h.reserve(ctx, j.ID)
					if err == nil {
						return errors.New("reservation succeeded after cancellation")
					}
					return err
				}
				return ctx.Err()
			})
			if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, entered)
			cancelled, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id)
			if err != nil || cancelled.CancelRequestedAt == nil || cancelled.Status != job.StatusRunning {
				t.Fatal(cancelled, err)
			}
			close(release)
			awaitCancellationSignal(t, terminal)
			j, err := h.jobs.GetByID(t.Context(), id)
			if err != nil || j.Status != job.StatusCancelled {
				t.Fatal(j, err)
			}
			var holds int
			if err := h.db.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions").Scan(&holds); err != nil || holds != 0 {
				t.Fatal("unreserved cancellation charged", holds, err)
			}
			h.assertPreviousResult(t)
		})
	}
}

func TestClipCancellationWhileProviderExitsWaitsForConfirmedUsage(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown", true: "confirmed"}[known], func(t *testing.T) {
			h := newCancellationHarness(t, nil)
			id := h.enqueue(t, job.KindGenerateClip)
			inFlight, recorded, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			terminal := h.run(t, job.KindGenerateClip, func(ctx context.Context, j job.Job, _ job.Progress) error {
				admitted, err := h.reserve(ctx, j.ID)
				if err != nil {
					return err
				}
				if _, err := job.ConsumeClipPolicy(admitted, "alice", j.ID, "p/o", 8192, "observe"); err != nil {
					return err
				}
				close(inFlight)
				<-ctx.Done()
				// Even a detached continuation cannot authorize the remaining writer call.
				if _, err := job.ConsumeClipPolicy(context.WithoutCancel(admitted), "alice", j.ID, "p/w", 32768, "write"); err == nil {
					return errors.New("writer authorized after durable cancellation")
				}
				u := llm.Usage{}
				if known {
					u.CostReported = true
					u.CostMicrousd = 100
				}
				work := usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: job.KindGenerateClip, JobID: j.ID})
				if err := h.ledger.RecordCall(work, llm.ModelRef{ProviderID: "p", ModelID: "o"}, "observe", u, ctx.Err()); err != nil {
					return err
				}
				close(recorded)
				<-release
				return ctx.Err()
			})
			if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, inFlight)
			if _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id); err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, recorded)
			a, err := h.ledger.ClipAccounting(t.Context(), "alice", id)
			if err != nil || a == nil || a.Settled {
				t.Fatal("settled while handler still owns work", a, err)
			}
			close(release)
			awaitCancellationSignal(t, terminal)
			a, err = h.ledger.ClipAccounting(t.Context(), "alice", id)
			if err != nil || !a.Settled {
				t.Fatal(a, err)
			}
			confirmed, fee := 0, 3
			if known {
				confirmed, fee = 3, 1
			}
			if *a.ConfirmedCharge != confirmed || *a.CancellationFee != fee || *a.FinalCharge != confirmed+fee || *a.Refund != 5-confirmed-fee {
				t.Fatal(a)
			}
			h.assertPreviousResult(t)
		})
	}
}

func TestClipCancellationAndResultCommitHaveOneWinner(t *testing.T) {
	for _, mode := range []string{"cancel after upload", "completion wins", "save fails", "completion response lost"} {
		t.Run(mode, func(t *testing.T) {
			h := newCancellationHarness(t, nil)
			id := h.enqueue(t, job.KindGenerateClip)
			boundary, release := make(chan struct{}), make(chan struct{})
			if mode == "save fails" {
				if _, err := h.db.Writer.Exec(`CREATE TRIGGER reject_clip_done BEFORE UPDATE OF status ON generation_jobs WHEN NEW.status='done' BEGIN SELECT RAISE(ABORT,'test commit failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			terminal := h.run(t, job.KindGenerateClip, func(ctx context.Context, j job.Job, _ job.Progress) error {
				if _, err := h.reserve(ctx, j.ID); err != nil {
					return err
				}
				c := h.candidate(j.ID, false)
				if mode == "cancel after upload" {
					if err := h.clips.StageAttemptResult(ctx, c); err != nil {
						return err
					}
					close(boundary)
					<-release
				}
				err := h.finisher.Complete(ctx, c)
				if mode != "cancel after upload" {
					close(boundary)
					<-release
				}
				if mode == "completion response lost" && err == nil {
					return errors.New("response lost after commit")
				}
				return err
			})
			if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, boundary)
			if mode == "cancel after upload" || mode == "completion wins" {
				j, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id)
				if err != nil {
					t.Fatal(err)
				}
				if (j.CancelRequestedAt != nil) != (mode == "cancel after upload") {
					t.Fatal("wrong cancellation winner", j)
				}
			}
			if mode == "save fails" {
				h.assertPreviousResult(t)
			}
			close(release)
			awaitCancellationSignal(t, terminal)
			if err := h.finisher.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			j, _ := h.jobs.GetByID(t.Context(), id)
			success := mode == "completion wins" || mode == "completion response lost"
			if success {
				p, err := h.clips.GetProject(t.Context(), "alice", "clip")
				if err != nil || j.Status != job.StatusDone || p.Result.Key != "clip-results/alice/clip/new.mp4" || p.EditPlanRevision != 2 {
					t.Fatal(j, p, err)
				}
			} else {
				h.assertPreviousResult(t)
			}
			var candidates, deleted int
			if err := h.db.Reader.QueryRow("SELECT COUNT(*) FROM clip_attempt_results").Scan(&candidates); err != nil || candidates != 0 {
				t.Fatal(candidates, err)
			}
			if err := h.db.Reader.QueryRow("SELECT COUNT(*) FROM clip_object_deletions WHERE object_key='clip-results/alice/clip/new.mp4'").Scan(&deleted); err != nil || deleted != map[bool]int{true: 0, false: 1}[success] {
				t.Fatal("candidate cleanup", deleted, err)
			}
			a, err := h.ledger.ClipAccounting(t.Context(), "alice", id)
			if err != nil || a == nil || !a.Settled {
				t.Fatal(a, err)
			}
			want := 0
			if success {
				want = 2
			} else if mode == "cancel after upload" {
				want = 3
			}
			if *a.FinalCharge != want {
				t.Fatal("wrong settlement winner", a)
			}
		})
	}
}

func TestClipQueuedCancellationIsOwnedIdempotentAndSurvivesRestart(t *testing.T) {
	h := newCancellationHarness(t, nil)
	id := h.enqueue(t, job.KindGenerateClip)
	if _, err := h.queue.CancelClipJob(t.Context(), "bob", "clip", id); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("foreign cancellation", err)
	}
	if _, err := h.queue.CancelClipJob(t.Context(), "alice", "other", id); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("mismatched project", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id); errs <- err })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil || j.Status != job.StatusCancelled || j.FinishedAt == nil {
		t.Fatal(j, err)
	}
	if _, err := h.jobs.PickNextQueued(t.Context(), time.Now()); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("dispatched cancelled queue", err)
	}
	// A new queue represents process restart; a terminal request remains unchanged.
	recovered := job.New(h.jobs, time.Millisecond)
	recovered.Admit(h.admission)
	if _, err := recovered.SweepRunning(t.Context()); err != nil {
		t.Fatal(err)
	}
	again, err := recovered.CancelClipJob(t.Context(), "alice", "clip", id)
	if err != nil || again.Status != job.StatusCancelled || !again.FinishedAt.Equal(*j.FinishedAt) {
		t.Fatal(again, err)
	}
}

func TestClipPendingCancellationRecoversBeforeFailureAndSettlesOnce(t *testing.T) {
	h := newCancellationHarness(t, nil)
	id := h.enqueue(t, job.KindGenerateClip)
	if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.reserve(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if err := h.clips.StageAttemptResult(t.Context(), h.candidate(id, false)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id); err != nil {
		t.Fatal(err)
	}
	var path string
	if err := h.db.Reader.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path); err != nil {
		t.Fatal(err)
	}
	if err := h.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	if err := db.Migrate(t.Context(), reopened.Writer); err != nil {
		t.Fatal(err)
	}
	h.db, h.jobs, h.clips = reopened, jobstore.New(reopened.Writer, reopened.Reader), clipstore.New(reopened.Writer, reopened.Reader)
	h.ledger = h.ledger.WithStore(usagestore.New(reopened.Writer, reopened.Reader))
	h.admission.ledger = h.ledger
	h.finisher = clipFinisher{writer: reopened.Writer, clips: h.clips, jobs: h.jobs}
	recovered := job.New(h.jobs, time.Millisecond)
	recovered.Admit(h.admission)
	for range 2 {
		if _, err := recovered.SweepRunning(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err := recovered.SweepOpenHolds(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := h.finisher.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil || j.Status != job.StatusCancelled {
		t.Fatal(j, err)
	}
	a, err := h.ledger.ClipAccounting(t.Context(), "alice", id)
	if err != nil || !a.Settled || a.SettlementReason != "cancelled" || *a.FinalCharge != 3 || *a.Refund != 2 {
		t.Fatal(a, err)
	}
	h.assertPreviousResult(t)
}

type pausedClipPick struct {
	*jobstore.Store
	picked, release chan struct{}
	once            sync.Once
}

func (s *pausedClipPick) PickNextQueued(ctx context.Context, now time.Time) (job.Job, error) {
	j, err := s.Store.PickNextQueued(ctx, now)
	if err != nil {
		return j, err
	}
	s.once.Do(func() {
		close(s.picked)
		select {
		case <-s.release:
		case <-ctx.Done():
		}
	})
	return j, nil
}
func TestClipCancellationBeforeHandlerRegistrationSkipsTheHandler(t *testing.T) {
	var paused *pausedClipPick
	h := newCancellationHarness(t, func(s *jobstore.Store) job.Store {
		paused = &pausedClipPick{Store: s, picked: make(chan struct{}), release: make(chan struct{})}
		return paused
	})
	id := h.enqueue(t, job.KindGenerateClip)
	called := make(chan struct{}, 1)
	terminal := h.run(t, job.KindGenerateClip, func(context.Context, job.Job, job.Progress) error { called <- struct{}{}; return nil })
	if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	awaitCancellationSignal(t, paused.picked)
	if _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id); err != nil {
		t.Fatal(err)
	}
	close(paused.release)
	awaitCancellationSignal(t, terminal)
	select {
	case <-called:
		t.Fatal("cancelled work entered handler")
	default:
	}
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil || j.Status != job.StatusCancelled {
		t.Fatal(j, err)
	}
}

func TestClipReservationRacingCancellationCannotAcquireALaterHold(t *testing.T) {
	for i := range 8 {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			h := newCancellationHarness(t, nil)
			id := h.enqueue(t, job.KindGenerateClip)
			entered, start, cancelled := make(chan struct{}), make(chan struct{}), make(chan struct{})
			terminal := h.run(t, job.KindGenerateClip, func(ctx context.Context, j job.Job, _ job.Progress) error {
				close(entered)
				<-start
				admitted, err := h.reserve(ctx, j.ID)
				<-cancelled
				if err == nil {
					if _, err := job.ConsumeClipPolicy(context.WithoutCancel(admitted), "alice", j.ID, "p/o", 8192, "observe"); err == nil {
						return errors.New("dispatch escaped cancellation")
					}
				}
				return context.Canceled
			})
			if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, entered)
			cancelErr := make(chan error, 1)
			go func() {
				<-start
				_, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id)
				cancelErr <- err
				close(cancelled)
			}()
			close(start)
			if err := <-cancelErr; err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, terminal)
			a, err := h.ledger.ClipAccounting(t.Context(), "alice", id)
			if err != nil {
				t.Fatal(err)
			}
			if a != nil && (!a.Settled || *a.NominalReservation != 5 || *a.FinalCharge != 3 || *a.Refund != 2) {
				t.Fatal("wrong reservation winner", a)
			}
			j, err := h.jobs.GetByID(t.Context(), id)
			if err != nil || j.Status != job.StatusCancelled {
				t.Fatal(j, err)
			}
			h.assertPreviousResult(t)
		})
	}
}

func TestClipNormalFailureRacingCancellationUsesTheDurableOutcome(t *testing.T) {
	for i := range 8 {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			h := newCancellationHarness(t, nil)
			id := h.enqueue(t, job.KindGenerateClip)
			entered, start := make(chan struct{}), make(chan struct{})
			terminal := h.run(t, job.KindGenerateClip, func(ctx context.Context, j job.Job, _ job.Progress) error {
				if _, err := h.reserve(ctx, j.ID); err != nil {
					return err
				}
				close(entered)
				<-start
				return errors.New("ordinary renderer failure")
			})
			if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, entered)
			cancelErr := make(chan error, 1)
			go func() { <-start; _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id); cancelErr <- err }()
			close(start)
			if err := <-cancelErr; err != nil {
				t.Fatal(err)
			}
			awaitCancellationSignal(t, terminal)
			j, err := h.jobs.GetByID(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			a, err := h.ledger.ClipAccounting(t.Context(), "alice", id)
			if err != nil || a == nil || !a.Settled {
				t.Fatal(a, err)
			}
			want := 0
			if j.CancelRequestedAt != nil {
				want = 3
				if j.Status != job.StatusCancelled {
					t.Fatal(j)
				}
			} else if j.Status != job.StatusFailed {
				t.Fatal(j)
			}
			if *a.FinalCharge != want {
				t.Fatal("settlement ignored durable winner", j, a)
			}
		})
	}
}

func TestClipCancellationPolicyRejectsLegacyAndOtherJobKinds(t *testing.T) {
	h := newCancellationHarness(t, nil)
	id, err := h.queue.Enqueue(t.Context(), job.NewJob{UserID: "alice", ClipProjectID: "clip", Kind: job.KindGenerateClip, ObserveModel: "p/o", WriteModel: "p/w"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id); !errors.Is(err, job.ErrCancellationUnavailable) {
		t.Fatal("legacy charged cancellation offered", err)
	}
	if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := h.jobs.Finish(t.Context(), id, job.StatusFailed, &job.Failure{Reason: "CLIP_PROCESSING_FAILED"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := h.db.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at) VALUES('voice','alice','analyze_voice','queued',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", "voice"); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("non-clip cancellation exposed", err)
	}
}

func TestClipCompletionRejectsConflictingStagedCandidate(t *testing.T) {
	h := newCancellationHarness(t, nil)
	id := h.enqueue(t, job.KindGenerateClip)
	if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	candidate := h.candidate(id, false)
	if err := h.clips.StageAttemptResult(t.Context(), candidate); err != nil {
		t.Fatal(err)
	}
	changed := candidate
	changed.EditPlan = "different-plan"
	if err := h.finisher.Complete(t.Context(), changed); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("conflicting candidate accepted", err)
	}
	h.assertPreviousResult(t)
	if err := h.finisher.Complete(t.Context(), candidate); err != nil {
		t.Fatal(err)
	}
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil || j.Status != job.StatusDone {
		t.Fatal(j, err)
	}
}

func TestClipManualRenderCompletionChecksSavedRevision(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching", true: "stale"}[stale], func(t *testing.T) {
			h := newCancellationHarness(t, nil)
			id := h.enqueue(t, job.KindRenderClip)
			if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
				t.Fatal(err)
			}
			if _, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil {
				t.Fatal(err)
			}
			c := h.candidate(id, true)
			if stale {
				c.ExpectedRevision = 0
			}
			err := h.finisher.Complete(t.Context(), c)
			if stale {
				if !errors.Is(err, clip.ErrPlanConflict) {
					t.Fatal(err)
				}
				h.assertPreviousResult(t)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				p, err := h.clips.GetProject(t.Context(), "alice", "clip")
				if err != nil || p.EditPlan != "old-plan" || p.EditPlanRevision != 1 || p.RenderedPlanRevision != 1 || p.Result == nil || p.Result.Key != c.Result.Key {
					t.Fatal(p, err)
				}
			}
			var n int
			if err := h.db.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions").Scan(&n); err != nil || n != 0 {
				t.Fatal("manual render charged", n, err)
			}
		})
	}
}
