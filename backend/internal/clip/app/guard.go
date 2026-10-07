package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// Guard takes the credit hold for a charged clip job in the same transaction that proves
// the job is still the running, unreserved `prepare`-stage job the approval described.
type Guard struct {
	writer            *sql.DB
	bind              Binder
	authorizer        Authorizer
	access            AccessChecker
	now               func() time.Time
	analysisQualified func(string) bool
}

type BrowserHoldPreflight interface {
	AuthorizeHold(context.Context, Hold) error
}

func NewGuard(writer *sql.DB, bind Binder, authorizer Authorizer, access AccessChecker) Guard {
	if writer == nil || bind == nil || authorizer == nil || access == nil {
		panic("clip app: guard needs writer, binder, authorizer and access checker")
	}
	return Guard{writer: writer, bind: bind, authorizer: authorizer, access: access, now: time.Now, analysisQualified: clip.BrowserAnalysisQualified}
}

// Reservable is the state guard: only a running charged clip job in its
// prepare stage, not yet asked to cancel, under the policy the start names,
// may take a hold.
func Reservable(j job.Job, hold Hold) bool {
	return j.UserID == hold.UserID && clip.ChargedJobKind(j.Kind) && j.Status == job.StatusRunning && j.Stage == "prepare" && j.CancelRequestedAt == nil && j.CancellationPolicyVersion == hold.Reservation.CancellationPolicyVersion
}

// Reserve qualifies the hold's model access first, outside the writer: a free path's
// live check can fetch the provider's endpoint document, and no write transaction spans
// a provider call (ARCH-10). The in-transaction hold is then told the check ran.
func (g Guard) Reserve(ctx context.Context, hold Hold) error {
	// A browser parent must already be verified and live before even the
	// free-model access check can fetch a provider's endpoint document.
	if preflight, ok := g.authorizer.(BrowserHoldPreflight); ok {
		if err := preflight.AuthorizeHold(ctx, hold); err != nil {
			return err
		}
	}
	if err := g.access.CheckAccess(ctx, hold); err != nil {
		return err
	}
	hold.AccessChecked = true
	return WriteTx(ctx, g.writer, g.bind, func(p Ports) error {
		j, err := p.Jobs.GetByID(ctx, hold.JobID)
		if err != nil {
			return err
		}
		if !Reservable(j, hold) {
			return clip.ErrCreditAllowance
		}
		if err := authorizeBrowserHold(ctx, p, j, hold.UserID, g.now(), g.analysisQualified); err != nil {
			return err
		}
		if p.Admission == nil {
			return errors.New("clip ledger unavailable")
		}
		return p.Admission.Hold(ctx, hold)
	})
}

// Authorize serializes dispatch authorization with cancellation through the
// job store's conditional writer statement.
func (g Guard) Authorize(ctx context.Context, user, id string) error {
	return g.authorizer.AuthorizeDispatch(ctx, user, id)
}

func authorizeBrowserHold(ctx context.Context, p Ports, j job.Job, user string, now time.Time, qualified func(string) bool) error {
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
	var payload clip.GenerationPayload
	if clip.StrictJSON(string(j.Payload), &payload) != nil {
		return clip.ErrInvalid
	}
	if p.Analysis == nil || payload.Approval == nil {
		return clip.ErrAnalysisPreparationState
	}
	s, e := p.Analysis.GetAnalysisPreparation(ctx, user, payload.AnalysisPreparationID)
	if e != nil {
		return e
	}
	if s.ParentJobID != j.ID || s.ProjectID != j.Subject(clip.JobSubject) || s.QuoteID != payload.Approval.QuoteID || s.State != "consumed" {
		return clip.ErrAnalysisPreparationState
	}
	if len(s.Copies) > 0 && !qualified(s.ProfileVersion) {
		return clip.ErrAnalysisProfileUnqualified
	}
	return p.Analysis.AuthorizeAnalysisPreparation(ctx, s, now)
}
