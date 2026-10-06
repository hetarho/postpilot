package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"log/slog"
	"strings"
	"time"
)

// Reconcile never invokes a model or reserves credits. Accepted waiting
// handoffs survive boot; claimed paid continuations retain FailOnInterrupt.
func (a *AnalysisPreparations) Reconcile(ctx context.Context) error {
	rows, e := a.store.AnalysisPreparationsForRecovery(ctx)
	if e != nil {
		return e
	}
	for _, row := range rows {
		var parent, ack, fail string
		reconciled := false
		e = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
			s, e := p.Analysis.GetAnalysisPreparation(ctx, row.UserID, row.ID)
			if e != nil {
				return e
			}
			now := a.now().UTC()
			parent = s.ParentJobID
			closed, cancelled, terminal := !clip.AnalysisPreparationLive(s.State), s.State == "cancelled", false
			if parent != "" {
				j, e := p.Jobs.GetByID(ctx, parent)
				if e != nil && !errors.Is(e, job.ErrNotFound) {
					return e
				}
				terminal = errors.Is(e, job.ErrNotFound) || job.Terminal(j.Status)
				cancelled = cancelled || j.CancelRequestedAt != nil
				closed = closed || terminal || cancelled
			}
			if !s.ExpiresAt.After(now) || !s.DeadlineAt.After(now) {
				closed = true
				if !cancelled && !terminal {
					if e = p.Analysis.SetAnalysisPreparationState(ctx, s.ID, "expired", "deadline_exceeded"); e != nil {
						return e
					}
					fail = "CLIP_MEDIA_UNAVAILABLE"
				}
			}
			if !closed {
				e = p.Analysis.AuthorizeAnalysisPreparation(ctx, s, now)
				if e != nil {
					if !errors.Is(e, clip.ErrNotFound) && !errors.Is(e, clip.ErrFinalized) && !errors.Is(e, clip.ErrPlanConflict) && !errors.Is(e, clip.ErrSourceState) && !errors.Is(e, clip.ErrAnalysisPreparationState) {
						return e
					}
					closed = true
					if e = p.Analysis.SetAnalysisPreparationState(ctx, s.ID, "failed", "invalid_input"); e != nil {
						return e
					}
					fail = "CLIP_PROCESSING_FAILED"
				}
			}
			stopped, e := p.Analysis.AnalysisLeaseStopped(ctx, s, now)
			if e != nil {
				return e
			}
			if !closed && s.State == "verifying" && (s.Attempts == 0 && !s.QueueDeadlineAt.After(now) || stopped && s.Attempts >= a.limits.Stages.MaxAttempts) {
				closed = true
				fail = "CLIP_MEDIA_UNAVAILABLE"
				if e = p.Analysis.SetAnalysisPreparationState(ctx, s.ID, "failed", "verification_expired"); e != nil {
					return e
				}
			}
			if closed {
				if cancelled {
					if e = p.Analysis.SetAnalysisPreparationState(ctx, s.ID, "cancelled", ""); e != nil {
						return e
					}
				}
				if !stopped {
					return nil
				}
				if s.CurrentAttemptID != "" {
					if e = p.Analysis.StopAnalysisAttempt(ctx, s.CurrentAttemptID, "abandoned", now); e != nil {
						return e
					}
				}
				if e = p.Analysis.RetireAnalysisPreparation(ctx, s.ID, now); e != nil {
					return e
				}
				reconciled = terminal || parent == ""
				if parent != "" && !terminal {
					if cancelled {
						ack = AnalysisPreparationWaitKey(s.ID)
					} else {
						if fail == "" {
							fail = "CLIP_PROCESSING_FAILED"
						}
					}
				}
				return nil
			}
			if s.State == "accepted" && parent != "" {
				_, e = p.Waits.Wake(ctx, parent, AnalysisPreparationWaitKey(s.ID), now)
			}
			return e
		})
		if e != nil {
			return e
		}
		if ack != "" {
			changed, ackErr := a.jobs.AcknowledgeWaitCancellation(ctx, parent, ack)
			if ackErr != nil {
				return ackErr
			}
			reconciled = changed
		} else if fail != "" && parent != "" {
			changed, failErr := a.jobs.FailWait(ctx, parent, AnalysisPreparationWaitKey(row.ID), job.Failure{Reason: fail})
			if failErr != nil {
				return failErr
			}
			reconciled = changed
		}
		if reconciled {
			if e = WriteTx(ctx, a.writer, a.bind, func(p Ports) error { return p.Analysis.MarkAnalysisPreparationReconciled(ctx, row.ID, a.now()) }); e != nil {
				return e
			}
		}
	}
	return nil
}
func (a *AnalysisPreparations) Cleanup(ctx context.Context) error {
	rows, e := a.store.DueAnalysisCopyCleanup(ctx, a.now().Add(-a.limits.OrphanGrace))
	if e != nil {
		return e
	}
	for _, r := range rows {
		if !analysisCopyKey(r.Key) {
			return clip.ErrInvalid
		}
		if e = a.objects.Delete(ctx, r.Key); e != nil {
			return errors.New("analysis copy cleanup pending")
		}
		if e = a.store.FinishAnalysisCopyCleanup(ctx, r.Key); e != nil {
			return e
		}
	}
	return nil
}
func analysisCopyKey(key string) bool {
	suffix, ok := strings.CutPrefix(key, clip.AnalysisPreparationPrefix)
	return ok && len(strings.Split(suffix, "/")) == 3 && strings.HasSuffix(suffix, ".mp4")
}
func (a *AnalysisPreparations) SweepOrphans(ctx context.Context) error {
	objects, e := a.objects.ListAnalysisCopies(ctx)
	if e != nil {
		return e
	}
	for _, o := range objects {
		if analysisCopyKey(o.Key) && !o.Modified.IsZero() && o.Modified.Before(a.now().Add(-a.limits.OrphanGrace)) {
			if e = a.store.QueueAnalysisOrphan(ctx, o.Key, a.now()); e != nil {
				return e
			}
		}
	}
	return nil
}
func (a *AnalysisPreparations) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pass, cancel := context.WithTimeout(ctx, 4*time.Second)
			e := a.Reconcile(pass)
			if e == nil {
				e = a.Cleanup(pass)
			}
			cancel()
			if e != nil && ctx.Err() == nil {
				slog.Warn("browser analysis recovery will retry")
			}
		}
	}
}
func (a *AnalysisPreparations) RunOrphans(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pass, cancel := context.WithTimeout(ctx, 30*time.Second)
			e := a.SweepOrphans(pass)
			cancel()
			if e != nil && ctx.Err() == nil {
				slog.Warn("browser analysis orphan cleanup will retry")
			}
		}
	}
}

// AnalysisDispatchAuthorizer adds the live browser fence to the existing job
// dispatch CAS. Native accepted jobs retain their established authorization.
type AnalysisDispatchAuthorizer struct {
	writer *sql.DB
	bind   Binder
	base   Authorizer
	now    func() time.Time
}

func NewAnalysisDispatchAuthorizer(writer *sql.DB, bind Binder, base Authorizer) AnalysisDispatchAuthorizer {
	if writer == nil || bind == nil || base == nil {
		panic("clip: browser dispatch needs atomic owned guards")
	}
	return AnalysisDispatchAuthorizer{writer: writer, bind: bind, base: base, now: time.Now}
}

func (g AnalysisDispatchAuthorizer) AuthorizeHold(ctx context.Context, hold Hold) error {
	return WriteTx(ctx, g.writer, g.bind, func(p Ports) error {
		j, e := p.Jobs.GetByID(ctx, hold.JobID)
		if e != nil {
			return e
		}
		var reference struct{ AnalysisPreparationID string }
		if !clip.PreparesMedia(j.Kind) || len(j.Payload) == 0 {
			return nil
		}
		if json.Unmarshal(j.Payload, &reference) != nil {
			return clip.ErrInvalid
		}
		if reference.AnalysisPreparationID == "" {
			return nil
		}
		if !Reservable(j, hold) {
			return clip.ErrCreditAllowance
		}
		return authorizeBrowserHold(ctx, p, j, hold.UserID, g.now(), clip.BrowserAnalysisQualified)
	})
}

type analysisDispatchTx interface {
	AuthorizeDispatch(context.Context, string, string) error
}

func (g AnalysisDispatchAuthorizer) AuthorizeDispatch(ctx context.Context, user, id string) error {
	browser := false
	e := WriteTx(ctx, g.writer, g.bind, func(p Ports) error {
		j, e := p.Jobs.GetByID(ctx, id)
		if e != nil {
			return e
		}
		if !clip.PreparesMedia(j.Kind) {
			return nil
		}
		var reference struct{ AnalysisPreparationID string }
		if len(j.Payload) == 0 {
			return nil
		}
		if json.Unmarshal(j.Payload, &reference) != nil {
			return clip.ErrInvalid
		}
		if reference.AnalysisPreparationID == "" {
			return nil
		}
		var frozen clip.GenerationPayload
		if clip.StrictJSON(string(j.Payload), &frozen) != nil {
			return clip.ErrInvalid
		}
		browser = true
		if p.Analysis == nil {
			return clip.ErrMediaUnsupported
		}
		s, e := p.Analysis.GetAnalysisPreparation(ctx, user, frozen.AnalysisPreparationID)
		if e != nil {
			return e
		}
		if frozen.Approval == nil {
			return clip.ErrInvalid
		}
		if s.ParentJobID != id || s.State != "consumed" || s.ProjectID != j.Subject(clip.JobSubject) || s.QuoteID != frozen.Approval.QuoteID {
			return clip.ErrAnalysisPreparationState
		}
		if e = p.Analysis.AuthorizeAnalysisPreparation(ctx, s, g.now()); e != nil {
			return e
		}
		dispatch, ok := p.Jobs.(analysisDispatchTx)
		if !ok {
			return clip.ErrCompositionUnavailable
		}
		return dispatch.AuthorizeDispatch(ctx, user, id)
	})
	if e != nil || browser {
		return e
	}
	return g.base.AuthorizeDispatch(ctx, user, id)
}
