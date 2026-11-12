package clip

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrFinalized            = errors.New("clip project is finalized")
	ErrFinalizationConflict = errors.New("clip finalization revision or result changed")
	ErrFinalizationInvalid  = errors.New("clip has no valid matching result to finalize")
	// A plan not confirmed at its current revision cannot be finalized (CLIP-76).
	ErrFinalizationUnconfirmed = fmt.Errorf("%w: plan is not confirmed", ErrFinalizationInvalid)
	// 확정하기 needs a valid saved plan (CLIP-75).
	ErrConfirmationInvalid = errors.New("clip has no valid plan to confirm")
)

type Finalization struct {
	At           time.Time
	PlanRevision int
	ResultID     string
}
type FinalizationRequest struct {
	UserID, ProjectID string
	ExpectedRevision  int
	ExpectedResultID  string
	// Present finalizes as 게시됨, empty as 완료 (CLIP-232); validated against the project's channel.
	PublishedURL string
}

// Confirmation is the plan revision 확정하기 recorded (CLIP-229).
type Confirmation struct {
	At           time.Time
	PlanRevision int
}

// Publication is a finalized clip's posted URL (CLIP-232).
type Publication struct {
	At  time.Time
	URL string
}

type ConfirmationRequest struct {
	UserID, ProjectID    string
	ExpectedPlanRevision int
}

type ProjectFinalizer interface {
	Finalize(context.Context, FinalizationRequest) (Project, error)
	Confirm(context.Context, ConfirmationRequest) (Project, error)
	SavePublishedURL(ctx context.Context, user, id, url string) (Project, error)
}

// Confirmed is CLIP-229's reversible state: the project stays confirmed while its plan
// is the revision 확정하기 recorded, so a later plan save unconfirms it without a write.
func Confirmed(p Project) bool {
	return p.Confirmed != nil && p.EditPlanRevision > 0 && p.Confirmed.PlanRevision == p.EditPlanRevision
}

// ConfirmationRefusal names why 확정하기 cannot confirm the current plan, empty when it can:
// finalized | busy | invalid_plan | narration. A stale expected revision is the request's
// own refusal (ValidateConfirmation), never the project's.
func ConfirmationRefusal(p Project, busy bool, cfg RenderConfig) string {
	if p.Finalized != nil {
		return "finalized"
	}
	if busy {
		return "busy"
	}
	_, err := confirmablePlan(p, cfg)
	var spoken *SpokenError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &spoken):
		return "narration"
	default:
		return "invalid_plan"
	}
}

// ValidateConfirmation checks a 확정하기 request against retained metadata only: it needs
// no render and deletes nothing (CLIP-75). Active attempts are the caller's transaction check.
func ValidateConfirmation(p Project, req ConfirmationRequest, cfg RenderConfig) error {
	if p.UserID != req.UserID || p.ID != req.ProjectID {
		return ErrNotFound
	}
	if p.Finalized != nil {
		return ErrFinalized
	}
	if req.ExpectedPlanRevision <= 0 || req.ExpectedPlanRevision != p.EditPlanRevision {
		return ErrPlanConflict
	}
	_, err := confirmablePlan(p, cfg)
	return err
}

// confirmablePlan is the plan half finalization also requires: valid evidence, a plan the
// correction contract accepts and ready required narration (DUB-26).
func confirmablePlan(p Project, cfg RenderConfig) (EditPlan, error) {
	if p.EditPlanRevision <= 0 {
		return EditPlan{}, ErrConfirmationInvalid
	}
	plan, err := DecodeEditPlan(p.EditPlan)
	if err != nil || ValidateCompositionEvidence(plan) != nil {
		return EditPlan{}, ErrConfirmationInvalid
	}
	if err := NarrationReadiness(plan); err != nil {
		return EditPlan{}, err
	}
	validated, err := ApplyCorrection(cfg, p, CorrectionFromPlan(plan))
	if err != nil || ValidateCompositionEvidence(validated) != nil {
		return EditPlan{}, ErrConfirmationInvalid
	}
	return plan, nil
}

// ValidateFinalization uses retained metadata only. Confirmation cannot require
// source pixels or re-render an already successful, matching export.
func ValidateFinalization(p Project, req FinalizationRequest, cfg RenderConfig) error {
	if p.UserID != req.UserID || p.ID != req.ProjectID {
		return ErrNotFound
	}
	if req.ExpectedRevision <= 0 || req.ExpectedResultID == "" {
		return ErrFinalizationConflict
	}
	if p.Finalized != nil {
		if p.Finalized.PlanRevision != req.ExpectedRevision || p.Finalized.ResultID != req.ExpectedResultID {
			return ErrFinalizationConflict
		}
		return nil
	}
	if p.EditPlanRevision != req.ExpectedRevision || p.RenderedPlanRevision != req.ExpectedRevision {
		return ErrFinalizationConflict
	}
	if !Confirmed(p) {
		return ErrFinalizationUnconfirmed
	}
	if p.Result == nil || p.Result.Key == "" || p.Result.ID == "" || p.Result.ContentType != "video/mp4" || p.Result.Bytes <= 0 || p.Result.DurationMS <= 0 || p.Result.CreatedAt.IsZero() {
		return ErrFinalizationInvalid
	}
	if p.Result.ID != req.ExpectedResultID {
		return ErrFinalizationConflict
	}
	plan, err := confirmablePlan(p, cfg)
	if errors.Is(err, ErrConfirmationInvalid) {
		return ErrFinalizationInvalid
	}
	if err != nil {
		return err
	}
	if SpeechFingerprint(RequestedSpeech(plan)) != SpeechFingerprint(p.Result.Speech) {
		return ErrFinalizationInvalid
	}
	return nil
}
